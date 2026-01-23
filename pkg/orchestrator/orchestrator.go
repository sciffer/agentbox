package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/sciffer/agentbox/internal/config"
	"github.com/sciffer/agentbox/internal/logger"
	"github.com/sciffer/agentbox/pkg/k8s"
	"github.com/sciffer/agentbox/pkg/models"
)

// Orchestrator manages environment lifecycle
type Orchestrator struct {
	k8sClient       k8s.ClientInterface
	config          *config.Config
	logger          *logger.Logger
	environments    map[string]*models.Environment
	envMutex        sync.RWMutex
	namespacePrefix string
}

// New creates a new orchestrator instance
func New(k8sClient k8s.ClientInterface, cfg *config.Config, log *logger.Logger) *Orchestrator {
	return &Orchestrator{
		k8sClient:       k8sClient,
		config:          cfg,
		logger:          log,
		environments:    make(map[string]*models.Environment),
		namespacePrefix: cfg.Kubernetes.NamespacePrefix,
	}
}

// CreateEnvironment creates a new isolated environment
func (o *Orchestrator) CreateEnvironment(ctx context.Context, req *models.CreateEnvironmentRequest, userID string) (*models.Environment, error) {
	envID := generateEnvironmentID()
	namespace := o.generateNamespace(envID)

	env := &models.Environment{
		ID:        envID,
		Name:      req.Name,
		Status:    models.StatusPending,
		Image:     req.Image,
		CreatedAt: time.Now(),
		Resources: req.Resources,
		Namespace: namespace,
		Env:       req.Env,
		Command:   req.Command,
		Labels:    req.Labels,
		Timeout:   req.Timeout,
		UserID:    userID,
		Endpoint:  fmt.Sprintf("ws://localhost:8080/api/v1/environments/%s/attach", envID),
	}

	// Store environment in memory
	o.envMutex.Lock()
	o.environments[envID] = env
	o.envMutex.Unlock()

	// Create Kubernetes resources asynchronously with timeout
	// Capture envID in local variable to avoid race condition
	provisionEnvID := envID
	provisionCtx, cancel := context.WithTimeout(context.Background(), time.Duration(o.config.Timeouts.StartupTimeout)*time.Second)
	go func() {
		defer cancel()
		// Re-acquire the environment from map to ensure we have the latest reference
		o.envMutex.RLock()
		provisionEnv, exists := o.environments[provisionEnvID]
		o.envMutex.RUnlock()

		if !exists {
			o.logger.Warn("environment not found during provisioning",
				zap.String("environment_id", provisionEnvID),
			)
			return
		}

		if err := o.provisionEnvironment(provisionCtx, provisionEnv); err != nil {
			o.logger.Error("failed to provision environment",
				zap.String("environment_id", provisionEnvID),
				zap.Error(err),
			)
			o.updateEnvironmentStatus(provisionEnvID, models.StatusFailed)
		}
	}()

	// Return a copy of the environment to avoid race conditions
	// The caller should not hold a reference to the same struct that the goroutine modifies
	envCopy := *env
	return &envCopy, nil
}

// provisionEnvironment creates the Kubernetes resources
func (o *Orchestrator) provisionEnvironment(ctx context.Context, env *models.Environment) error {
	// Capture values from env to avoid race conditions
	envID := env.ID
	envNamespace := env.Namespace
	envImage := env.Image
	envCommand := env.Command
	envResources := env.Resources
	envEnvVars := env.Env
	envLabels := env.Labels

	// Create namespace
	labels := map[string]string{
		"app":        "agentbox",
		"env-id":     envID,
		"managed-by": "agentbox",
	}
	for k, v := range envLabels {
		labels[k] = v
	}

	if err := o.k8sClient.CreateNamespace(ctx, envNamespace, labels); err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	// Create resource quota
	if err := o.k8sClient.CreateResourceQuota(
		ctx,
		envNamespace,
		envResources.CPU,
		envResources.Memory,
		envResources.Storage,
	); err != nil {
		return fmt.Errorf("failed to create resource quota: %w", err)
	}

	// Apply network policy (isolation)
	if err := o.applyNetworkPolicy(ctx, envNamespace); err != nil {
		return fmt.Errorf("failed to apply network policy: %w", err)
	}

	// Create pod
	podName := "main"
	command := envCommand
	if len(command) == 0 {
		command = []string{"/bin/sh", "-c", "sleep infinity"}
	}

	podSpec := &k8s.PodSpec{
		Name:         podName,
		Namespace:    envNamespace,
		Image:        envImage,
		Command:      command,
		Env:          envEnvVars,
		CPU:          envResources.CPU,
		Memory:       envResources.Memory,
		Storage:      envResources.Storage,
		RuntimeClass: o.config.Kubernetes.RuntimeClass,
		Labels:       labels,
	}

	if err := o.k8sClient.CreatePod(ctx, podSpec); err != nil {
		return fmt.Errorf("failed to create pod: %w", err)
	}

	// Wait for pod to be running
	waitCtx, cancel := context.WithTimeout(ctx, time.Duration(o.config.Timeouts.StartupTimeout)*time.Second)
	defer cancel()

	if err := o.k8sClient.WaitForPodRunning(waitCtx, envNamespace, podName); err != nil {
		return fmt.Errorf("pod failed to start: %w", err)
	}

	// Update environment status
	// Use captured envID to avoid accessing env fields
	now := time.Now()
	o.envMutex.Lock()
	if e, exists := o.environments[envID]; exists {
		// Create a new time value to avoid sharing the pointer
		startedAt := now
		e.Status = models.StatusRunning
		e.StartedAt = &startedAt
	}
	o.envMutex.Unlock()

	// Use captured values to avoid accessing env fields after unlock
	o.logger.Info("environment provisioned successfully",
		zap.String("environment_id", envID),
		zap.String("namespace", envNamespace),
	)

	return nil
}

// GetEnvironment retrieves an environment by ID
func (o *Orchestrator) GetEnvironment(ctx context.Context, envID string) (*models.Environment, error) {
	o.envMutex.RLock()
	env, exists := o.environments[envID]
	o.envMutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("environment not found")
	}

	// Refresh status from Kubernetes if running
	// Create a copy to avoid race conditions
	envCopy := *env
	if envCopy.Status == models.StatusRunning {
		pod, err := o.k8sClient.GetPod(ctx, envCopy.Namespace, "main")
		if err == nil {
			envCopy.Status = convertPodPhaseToStatus(string(pod.Status.Phase))
		}
	}

	return &envCopy, nil
}

// ListEnvironments lists all environments with optional filtering
func (o *Orchestrator) ListEnvironments(ctx context.Context, status *models.EnvironmentStatus, labelSelector string, limit, offset int) (*models.ListEnvironmentsResponse, error) {
	// Validate pagination parameters
	if limit <= 0 {
		limit = 100 // Default limit
	}
	if limit > 1000 {
		limit = 1000 // Max limit
	}
	if offset < 0 {
		offset = 0
	}

	o.envMutex.RLock()
	// Pre-allocate with estimated capacity
	envs := make([]models.Environment, 0, len(o.environments))

	for _, env := range o.environments {
		// Filter by status if specified
		if status != nil && env.Status != *status {
			continue
		}

		// Filter by label if specified
		if labelSelector != "" && !matchesLabelSelector(env.Labels, labelSelector) {
			continue
		}

		// Create a copy to avoid race conditions
		envCopy := *env
		envs = append(envs, envCopy)
	}
	o.envMutex.RUnlock()

	// Apply pagination
	total := len(envs)
	start := offset
	end := offset + limit

	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	// Avoid allocation if no results
	if start >= end {
		return &models.ListEnvironmentsResponse{
			Environments: []models.Environment{},
			Total:        total,
			Limit:        limit,
			Offset:       offset,
		}, nil
	}

	pagedEnvs := envs[start:end]

	return &models.ListEnvironmentsResponse{
		Environments: pagedEnvs,
		Total:        total,
		Limit:        limit,
		Offset:       offset,
	}, nil
}

// DeleteEnvironment terminates and removes an environment
func (o *Orchestrator) DeleteEnvironment(ctx context.Context, envID string, force bool) error {
	o.envMutex.Lock()
	env, exists := o.environments[envID]
	if !exists {
		o.envMutex.Unlock()
		return fmt.Errorf("environment not found")
	}
	env.Status = models.StatusTerminating
	o.envMutex.Unlock()

	// Delete pod (best effort - namespace deletion will cascade)
	if err := o.k8sClient.DeletePod(ctx, env.Namespace, "main", force); err != nil {
		o.logger.Warn("failed to delete pod (will be cleaned up with namespace)",
			zap.String("environment_id", envID),
			zap.String("namespace", env.Namespace),
			zap.Error(err),
		)
	}

	// Delete namespace (cascades to all resources)
	if err := o.k8sClient.DeleteNamespace(ctx, env.Namespace); err != nil {
		return fmt.Errorf("failed to delete namespace: %w", err)
	}

	// Remove from memory
	o.envMutex.Lock()
	delete(o.environments, envID)
	o.envMutex.Unlock()

	o.logger.Info("environment deleted",
		zap.String("environment_id", envID),
		zap.String("namespace", env.Namespace),
	)

	return nil
}

// ExecuteCommand executes a command in an environment
func (o *Orchestrator) ExecuteCommand(ctx context.Context, envID string, command []string, timeout int) (*models.ExecResponse, error) {
	env, err := o.GetEnvironment(ctx, envID)
	if err != nil {
		return nil, err
	}

	if env.Status != models.StatusRunning {
		return nil, fmt.Errorf("environment is not running")
	}

	// Set timeout if specified (with maximum limit)
	maxTimeout := o.config.Timeouts.MaxTimeout
	if timeout > 0 {
		if timeout > maxTimeout {
			timeout = maxTimeout
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	} else {
		// Use default timeout if not specified
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(o.config.Timeouts.DefaultTimeout)*time.Second)
		defer cancel()
	}

	// Execute command via Kubernetes
	startTime := time.Now()
	stdout, stderr, exitCode, err := o.executeInPod(ctx, env.Namespace, "main", command)
	duration := time.Since(startTime)

	if err != nil {
		return nil, fmt.Errorf("failed to execute command: %w", err)
	}

	return &models.ExecResponse{
		Stdout:     stdout,
		Stderr:     stderr,
		ExitCode:   exitCode,
		DurationMs: duration.Milliseconds(),
	}, nil
}

// GetLogs retrieves logs from an environment
func (o *Orchestrator) GetLogs(ctx context.Context, envID string, tailLines *int64) (*models.LogsResponse, error) {
	env, err := o.GetEnvironment(ctx, envID)
	if err != nil {
		return nil, err
	}

	// Get logs from the pod
	logsStr, err := o.k8sClient.GetPodLogs(ctx, env.Namespace, "main", tailLines)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod logs: %w", err)
	}

	// Parse logs into LogEntry format
	// Optimize: Pre-allocate slice with estimated capacity
	lines := strings.Split(logsStr, "\n")
	logs := make([]models.LogEntry, 0, len(lines))

	now := time.Now()
	for _, line := range lines {
		if line != "" {
			logs = append(logs, models.LogEntry{
				Timestamp: now, // Use single timestamp for batch
				Stream:    "stdout",
				Message:   line,
			})
		}
	}

	return &models.LogsResponse{
		Logs: logs,
	}, nil
}

// GetHealthInfo retrieves health information including cluster capacity
func (o *Orchestrator) GetHealthInfo(ctx context.Context) (*models.HealthResponse, error) {
	// Check Kubernetes connectivity
	connected := true
	version := ""
	capacity := models.ClusterCapacity{}

	if err := o.k8sClient.HealthCheck(ctx); err != nil {
		connected = false
	} else {
		// Get version
		var err error
		version, err = o.k8sClient.GetServerVersion(ctx)
		if err != nil {
			o.logger.Warn("failed to get kubernetes version", zap.Error(err))
		}

		// Get cluster capacity
		totalNodes, cpu, memory, err := o.k8sClient.GetClusterCapacity(ctx)
		if err != nil {
			o.logger.Warn("failed to get cluster capacity", zap.Error(err))
		} else {
			capacity = models.ClusterCapacity{
				TotalNodes:      totalNodes,
				AvailableCPU:    cpu,
				AvailableMemory: memory,
			}
		}
	}

	status := "healthy"
	if !connected {
		status = "unhealthy"
	}

	return &models.HealthResponse{
		Status:  status,
		Version: "1.0.0",
		Kubernetes: models.KubernetesHealthStatus{
			Connected: connected,
			Version:   version,
		},
		Capacity: capacity,
	}, nil
}

// Helper functions

// generateEnvironmentID generates a unique environment ID
// Format: env-<8-char-hex> (e.g., env-a1b2c3d4)
func generateEnvironmentID() string {
	// Use UUID v4 for better uniqueness
	id := uuid.New()
	// Take first 8 characters for shorter IDs
	return "env-" + id.String()[:8]
}

func (o *Orchestrator) generateNamespace(envID string) string {
	return o.namespacePrefix + envID
}

func (o *Orchestrator) updateEnvironmentStatus(envID string, status models.EnvironmentStatus) {
	o.envMutex.Lock()
	defer o.envMutex.Unlock()

	if env, exists := o.environments[envID]; exists {
		// Atomically update status to avoid race conditions
		env.Status = status
	}
}

func convertPodPhaseToStatus(phase string) models.EnvironmentStatus {
	switch phase {
	case "Pending":
		return models.StatusPending
	case "Running":
		return models.StatusRunning
	case "Succeeded":
		return models.StatusTerminated
	case "Failed":
		return models.StatusFailed
	default:
		return models.StatusPending
	}
}

func matchesLabelSelector(envLabels map[string]string, selectorStr string) bool {
	if selectorStr == "" {
		return true
	}

	// Parse the selector string using Kubernetes label selector
	selector, err := labels.Parse(selectorStr)
	if err != nil {
		// If parsing fails, return false (don't match)
		return false
	}

	// Convert environment labels to labels.Set for matching
	labelSet := labels.Set(envLabels)
	return selector.Matches(labelSet)
}

func (o *Orchestrator) applyNetworkPolicy(ctx context.Context, namespace string) error {
	return o.k8sClient.CreateNetworkPolicy(ctx, namespace)
}

func (o *Orchestrator) executeInPod(ctx context.Context, namespace, podName string, command []string) (stdout, stderr string, exitCode int, err error) {
	var stdoutBuf, stderrBuf bytes.Buffer

	err = o.k8sClient.ExecInPod(ctx, namespace, podName, command, nil, &stdoutBuf, &stderrBuf)
	if err != nil {
		return "", "", 1, err
	}

	return stdoutBuf.String(), stderrBuf.String(), 0, nil
}

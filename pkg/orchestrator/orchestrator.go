package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
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

// StandbyPod represents a pre-warmed pod ready to accept commands
type StandbyPod struct {
	Name      string
	Namespace string
	Image     string
	CreatedAt time.Time
}

// Orchestrator manages environment lifecycle
type Orchestrator struct {
	k8sClient       k8s.ClientInterface
	config          *config.Config
	logger          *logger.Logger
	environments    map[string]*models.Environment
	envMutex        sync.RWMutex
	namespacePrefix string
	// provisionSem limits concurrent environment provisioning to prevent
	// overwhelming the Kubernetes API with too many parallel requests
	provisionSem chan struct{}
	// execSem limits concurrent executions separately from provisioning
	execSem chan struct{}
	// executions tracks async command executions
	executions map[string]*models.Execution
	execMutex  sync.RWMutex
	// ephemeralNamespace is a shared namespace for all ephemeral executions
	// This avoids quota conflicts with environment namespaces
	ephemeralNamespace string
	ephemeralNsReady   bool
	ephemeralNsMutex   sync.Mutex
	// standbyPool holds pre-warmed pods ready for immediate use
	// Key is the image name, value is a slice of available standby pods
	standbyPool      map[string][]*StandbyPod
	standbyPoolMutex sync.Mutex
	// poolStopChan signals the pool replenishment goroutine to stop
	poolStopChan chan struct{}
}

// MaxConcurrentProvisions is the maximum number of environments that can be
// provisioned in parallel. This prevents overwhelming the Kubernetes API.
const MaxConcurrentProvisions = 10

// MaxConcurrentExecutions is the maximum number of command executions that can
// run in parallel. This is separate from environment provisioning.
const MaxConcurrentExecutions = 20

// New creates a new orchestrator instance
func New(k8sClient k8s.ClientInterface, cfg *config.Config, log *logger.Logger) *Orchestrator {
	o := &Orchestrator{
		k8sClient:          k8sClient,
		config:             cfg,
		logger:             log,
		environments:       make(map[string]*models.Environment),
		namespacePrefix:    cfg.Kubernetes.NamespacePrefix,
		provisionSem:       make(chan struct{}, MaxConcurrentProvisions),
		execSem:            make(chan struct{}, MaxConcurrentExecutions),
		executions:         make(map[string]*models.Execution),
		ephemeralNamespace: cfg.Kubernetes.NamespacePrefix + "ephemeral",
		ephemeralNsReady:   false,
		standbyPool:        make(map[string][]*StandbyPod),
		poolStopChan:       make(chan struct{}),
	}

	// Start pool replenishment if enabled
	if cfg.Pool.Enabled && cfg.Pool.Size > 0 {
		go o.runPoolReplenishment()
	}

	return o
}

// Stop gracefully shuts down the orchestrator
func (o *Orchestrator) Stop() {
	close(o.poolStopChan)
}

// CreateEnvironment creates a new isolated environment
func (o *Orchestrator) CreateEnvironment(ctx context.Context, req *models.CreateEnvironmentRequest, userID string) (*models.Environment, error) {
	envID := generateEnvironmentID()
	namespace := o.generateNamespace(envID)

	env := &models.Environment{
		ID:           envID,
		Name:         req.Name,
		Status:       models.StatusPending,
		Image:        req.Image,
		CreatedAt:    time.Now(),
		Resources:    req.Resources,
		Namespace:    namespace,
		Env:          req.Env,
		Command:      req.Command,
		Labels:       req.Labels,
		Timeout:      req.Timeout,
		UserID:       userID,
		NodeSelector: req.NodeSelector,
		Tolerations:  req.Tolerations,
		Isolation:    req.Isolation,
		Pool:         req.Pool,
		Endpoint:     fmt.Sprintf("ws://localhost:8080/api/v1/environments/%s/attach", envID),
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

		// Acquire semaphore to limit concurrent provisioning
		select {
		case o.provisionSem <- struct{}{}:
			// Acquired semaphore, release it when done
			defer func() { <-o.provisionSem }()
		case <-provisionCtx.Done():
			o.logger.Error("timeout waiting to start provisioning",
				zap.String("environment_id", provisionEnvID),
			)
			o.updateEnvironmentStatus(provisionEnvID, models.StatusFailed)
			return
		}

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
	envNodeSelector := env.NodeSelector
	envTolerations := env.Tolerations
	envIsolation := env.Isolation

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

	// Apply network policy with isolation config
	if err := o.applyNetworkPolicyWithConfig(ctx, envNamespace, envIsolation); err != nil {
		return fmt.Errorf("failed to apply network policy: %w", err)
	}

	// Create pod
	podName := "main"
	command := envCommand
	if len(command) == 0 {
		command = []string{"/bin/sh", "-c", "sleep infinity"}
	}

	// Convert model tolerations to k8s tolerations
	var k8sTolerations []k8s.Toleration
	for _, t := range envTolerations {
		k8sTolerations = append(k8sTolerations, k8s.Toleration{
			Key:               t.Key,
			Operator:          t.Operator,
			Value:             t.Value,
			Effect:            t.Effect,
			TolerationSeconds: t.TolerationSeconds,
		})
	}

	// Determine runtime class (per-environment overrides global)
	runtimeClass := o.config.Kubernetes.RuntimeClass
	if envIsolation != nil && envIsolation.RuntimeClass != "" {
		runtimeClass = envIsolation.RuntimeClass
	}

	// Convert security context if provided
	var securityContext *k8s.SecurityContext
	if envIsolation != nil && envIsolation.SecurityContext != nil {
		securityContext = &k8s.SecurityContext{
			RunAsUser:                envIsolation.SecurityContext.RunAsUser,
			RunAsGroup:               envIsolation.SecurityContext.RunAsGroup,
			RunAsNonRoot:             envIsolation.SecurityContext.RunAsNonRoot,
			ReadOnlyRootFilesystem:   envIsolation.SecurityContext.ReadOnlyRootFilesystem,
			AllowPrivilegeEscalation: envIsolation.SecurityContext.AllowPrivilegeEscalation,
		}
	}

	podSpec := &k8s.PodSpec{
		Name:            podName,
		Namespace:       envNamespace,
		Image:           envImage,
		Command:         command,
		Env:             envEnvVars,
		CPU:             envResources.CPU,
		Memory:          envResources.Memory,
		Storage:         envResources.Storage,
		RuntimeClass:    runtimeClass,
		Labels:          labels,
		NodeSelector:    envNodeSelector,
		Tolerations:     k8sTolerations,
		SecurityContext: securityContext,
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
	var poolEnabled bool
	if e, exists := o.environments[envID]; exists {
		// Create a new time value to avoid sharing the pointer
		startedAt := now
		e.Status = models.StatusRunning
		e.StartedAt = &startedAt
		// Check if pool is enabled for this environment
		poolEnabled = e.Pool != nil && e.Pool.Enabled
	}
	o.envMutex.Unlock()

	// Use captured values to avoid accessing env fields after unlock
	o.logger.Info("environment provisioned successfully",
		zap.String("environment_id", envID),
		zap.String("namespace", envNamespace),
	)

	// If environment has pool enabled, trigger immediate pool replenishment
	if poolEnabled {
		go o.replenishPool()
	}

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
func (o *Orchestrator) ListEnvironments(
	ctx context.Context, status *models.EnvironmentStatus, labelSelector string, limit, offset int,
) (*models.ListEnvironmentsResponse, error) {
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

// StreamLogs streams logs from an environment
func (o *Orchestrator) StreamLogs(ctx context.Context, envID string, tailLines *int64, follow bool) (io.ReadCloser, error) {
	env, err := o.GetEnvironment(ctx, envID)
	if err != nil {
		return nil, err
	}

	// Stream logs from the pod
	logsStream, err := o.k8sClient.StreamPodLogs(ctx, env.Namespace, "main", tailLines, follow)
	if err != nil {
		return nil, fmt.Errorf("failed to stream pod logs: %w", err)
	}

	return logsStream, nil
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

func (o *Orchestrator) applyNetworkPolicyWithConfig(ctx context.Context, namespace string, isolation *models.IsolationConfig) error {
	// If no isolation config, use default restrictive policy
	if isolation == nil || isolation.NetworkPolicy == nil {
		return o.k8sClient.CreateNetworkPolicy(ctx, namespace)
	}

	// Convert model config to k8s config
	npConfig := &k8s.NetworkPolicyConfig{
		AllowInternet:        isolation.NetworkPolicy.AllowInternet,
		AllowedEgressCIDRs:   isolation.NetworkPolicy.AllowedEgressCIDRs,
		AllowedIngressPorts:  isolation.NetworkPolicy.AllowedIngressPorts,
		AllowClusterInternal: isolation.NetworkPolicy.AllowClusterInternal,
	}

	return o.k8sClient.CreateNetworkPolicyWithConfig(ctx, namespace, npConfig)
}

func (o *Orchestrator) executeInPod(ctx context.Context, namespace, podName string, command []string) (stdout, stderr string, exitCode int, err error) {
	var stdoutBuf, stderrBuf bytes.Buffer

	err = o.k8sClient.ExecInPod(ctx, namespace, podName, command, nil, &stdoutBuf, &stderrBuf)
	if err != nil {
		return "", "", 1, err
	}

	return stdoutBuf.String(), stderrBuf.String(), 0, nil
}

// EphemeralExecRequest contains parameters for ephemeral execution
type EphemeralExecRequest struct {
	EnvironmentID string            `json:"environment_id"` // Reference to environment for config
	Command       []string          `json:"command"`
	Timeout       int               `json:"timeout,omitempty"`
	Env           map[string]string `json:"env,omitempty"` // Additional env vars (merged with environment's)
}

// ensureEphemeralNamespace creates the shared ephemeral namespace if it doesn't exist
// This namespace is used for all ephemeral executions and has a larger quota
func (o *Orchestrator) ensureEphemeralNamespace(ctx context.Context) error {
	o.ephemeralNsMutex.Lock()
	defer o.ephemeralNsMutex.Unlock()

	if o.ephemeralNsReady {
		return nil
	}

	labels := map[string]string{
		"app":        "agentbox",
		"managed-by": "agentbox",
		"type":       "ephemeral",
	}

	// Create namespace (idempotent - ignores "already exists" error)
	if err := o.k8sClient.CreateNamespace(ctx, o.ephemeralNamespace, labels); err != nil {
		// Check if namespace already exists
		exists, checkErr := o.k8sClient.NamespaceExists(ctx, o.ephemeralNamespace)
		if checkErr != nil || !exists {
			return fmt.Errorf("failed to create ephemeral namespace: %w", err)
		}
	}

	// Create a larger resource quota for concurrent executions
	// Allow up to 10 concurrent executions with 1 CPU and 1Gi memory each
	if err := o.k8sClient.CreateResourceQuota(
		ctx,
		o.ephemeralNamespace,
		"10",   // 10 CPUs total
		"10Gi", // 10Gi memory total
		"20Gi", // 20Gi storage total
	); err != nil {
		o.logger.Warn("failed to create resource quota for ephemeral namespace (may already exist)",
			zap.Error(err),
		)
	}

	// Apply default network policy (restrictive - no internet by default)
	if err := o.k8sClient.CreateNetworkPolicy(ctx, o.ephemeralNamespace); err != nil {
		o.logger.Warn("failed to apply network policy to ephemeral namespace (may already exist)",
			zap.Error(err),
		)
	}

	o.ephemeralNsReady = true
	o.logger.Info("ephemeral namespace ready",
		zap.String("namespace", o.ephemeralNamespace),
	)

	return nil
}

// SubmitExecution queues an async execution and returns immediately with the execution ID
// The execution runs in a goroutine and can be polled for status via GetExecution
func (o *Orchestrator) SubmitExecution(ctx context.Context, req *EphemeralExecRequest, userID string) (*models.Execution, error) {
	// Look up the environment to inherit its configuration
	env, err := o.GetEnvironment(ctx, req.EnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("environment not found: %w", err)
	}

	// Verify environment is running
	if env.Status != models.StatusRunning {
		return nil, fmt.Errorf("environment is not running (status: %s)", env.Status)
	}

	// Ensure the shared ephemeral namespace exists
	if err := o.ensureEphemeralNamespace(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize ephemeral namespace: %w", err)
	}

	// Generate unique execution ID
	execID := "exec-" + uuid.New().String()[:8]
	podName := execID // Use same name for pod

	now := time.Now()
	exec := &models.Execution{
		ID:            execID,
		EnvironmentID: req.EnvironmentID,
		Command:       req.Command,
		Env:           req.Env,
		Status:        models.ExecutionStatusPending,
		UserID:        userID,
		PodName:       podName,
		Namespace:     o.ephemeralNamespace, // Use shared ephemeral namespace
		CreatedAt:     now,
	}

	// Store execution
	o.execMutex.Lock()
	o.executions[execID] = exec
	o.execMutex.Unlock()

	o.logger.Info("execution submitted",
		zap.String("exec_id", execID),
		zap.String("environment_id", req.EnvironmentID),
		zap.Strings("command", req.Command),
		zap.String("user_id", userID),
	)

	// Run execution in background
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 300 // Default 5 minutes
	}
	if timeout > 3600 {
		timeout = 3600 // Max 1 hour
	}

	go o.runExecution(execID, env, req, timeout)

	// Return a copy to avoid race conditions
	execCopy := *exec
	return &execCopy, nil
}

// runExecution runs the actual pod execution in the background
func (o *Orchestrator) runExecution(execID string, env *models.Environment, req *EphemeralExecRequest, timeout int) {
	// Create timeout context for the execution
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	// Update status to queued (waiting for semaphore)
	o.updateExecutionStatus(execID, models.ExecutionStatusQueued, nil)

	// Acquire semaphore to limit concurrent executions
	select {
	case o.execSem <- struct{}{}:
		defer func() { <-o.execSem }()
	case <-ctx.Done():
		o.updateExecutionError(execID, "timeout waiting in queue")
		return
	}

	// Try to use a standby pod for faster execution
	standbyPod := o.claimStandbyPod(env.Image)

	// Update status to running
	now := time.Now()
	o.execMutex.Lock()
	if exec, exists := o.executions[execID]; exists {
		exec.Status = models.ExecutionStatusRunning
		exec.StartedAt = &now
		exec.QueuedAt = &now
		// Update pod name if using standby
		if standbyPod != nil {
			exec.PodName = standbyPod.Name
		}
	}
	o.execMutex.Unlock()

	// Get execution record
	o.execMutex.RLock()
	exec := o.executions[execID]
	namespace := exec.Namespace
	o.execMutex.RUnlock()

	// If we got a standby pod, use it with exec
	if standbyPod != nil {
		o.runWithStandbyPod(ctx, execID, standbyPod, req.Command, env)
		return
	}

	// No standby pod available, create a new one
	podName := exec.PodName

	o.logger.Info("starting execution (new pod)",
		zap.String("exec_id", execID),
		zap.String("pod", podName),
		zap.String("namespace", namespace),
		zap.String("image", env.Image),
	)

	// Labels for the pod
	labels := map[string]string{
		"app":            "agentbox",
		"exec-id":        execID,
		"managed-by":     "agentbox",
		"type":           "ephemeral",
		"user-id":        exec.UserID,
		"environment-id": req.EnvironmentID,
	}
	for k, v := range env.Labels {
		labels[k] = v
	}

	// Merge environment variables
	mergedEnv := make(map[string]string)
	for k, v := range env.Env {
		mergedEnv[k] = v
	}
	for k, v := range req.Env {
		mergedEnv[k] = v
	}

	// Determine runtime class
	runtimeClass := o.config.Kubernetes.RuntimeClass
	if env.Isolation != nil && env.Isolation.RuntimeClass != "" {
		runtimeClass = env.Isolation.RuntimeClass
	}

	// Convert security context
	var securityContext *k8s.SecurityContext
	if env.Isolation != nil && env.Isolation.SecurityContext != nil {
		securityContext = &k8s.SecurityContext{
			RunAsUser:                env.Isolation.SecurityContext.RunAsUser,
			RunAsGroup:               env.Isolation.SecurityContext.RunAsGroup,
			RunAsNonRoot:             env.Isolation.SecurityContext.RunAsNonRoot,
			ReadOnlyRootFilesystem:   env.Isolation.SecurityContext.ReadOnlyRootFilesystem,
			AllowPrivilegeEscalation: env.Isolation.SecurityContext.AllowPrivilegeEscalation,
		}
	}

	// Convert tolerations
	var k8sTolerations []k8s.Toleration
	for _, t := range env.Tolerations {
		k8sTolerations = append(k8sTolerations, k8s.Toleration{
			Key:               t.Key,
			Operator:          t.Operator,
			Value:             t.Value,
			Effect:            t.Effect,
			TolerationSeconds: t.TolerationSeconds,
		})
	}

	// Create pod spec
	podSpec := &k8s.PodSpec{
		Name:            podName,
		Namespace:       namespace,
		Image:           env.Image,
		Command:         req.Command,
		Env:             mergedEnv,
		CPU:             env.Resources.CPU,
		Memory:          env.Resources.Memory,
		Storage:         env.Resources.Storage,
		RuntimeClass:    runtimeClass,
		Labels:          labels,
		NodeSelector:    env.NodeSelector,
		Tolerations:     k8sTolerations,
		SecurityContext: securityContext,
	}

	// Create pod
	if err := o.k8sClient.CreatePod(ctx, podSpec); err != nil {
		o.updateExecutionError(execID, fmt.Sprintf("failed to create pod: %v", err))
		return
	}

	// Ensure pod cleanup
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := o.k8sClient.DeletePod(cleanupCtx, namespace, podName, true); err != nil {
			o.logger.Warn("failed to cleanup ephemeral pod",
				zap.String("exec_id", execID),
				zap.String("pod", podName),
				zap.Error(err),
			)
		} else {
			o.logger.Debug("cleaned up ephemeral pod",
				zap.String("exec_id", execID),
				zap.String("pod", podName),
			)
		}
	}()

	// Wait for pod completion
	startTime := time.Now()
	result, err := o.k8sClient.WaitForPodCompletion(ctx, namespace, podName)
	duration := time.Since(startTime)

	if err != nil {
		o.updateExecutionError(execID, fmt.Sprintf("execution failed: %v", err))
		return
	}

	// Update execution with results
	completedAt := time.Now()
	durationMs := duration.Milliseconds()
	o.execMutex.Lock()
	if e, exists := o.executions[execID]; exists {
		e.Status = models.ExecutionStatusCompleted
		e.CompletedAt = &completedAt
		e.ExitCode = &result.ExitCode
		e.Stdout = result.Logs
		e.DurationMs = &durationMs
	}
	o.execMutex.Unlock()

	o.logger.Info("execution completed",
		zap.String("exec_id", execID),
		zap.String("pod", podName),
		zap.Int("exit_code", result.ExitCode),
		zap.Int64("duration_ms", durationMs),
	)
}

// runWithStandbyPod executes a command using a pre-warmed standby pod
func (o *Orchestrator) runWithStandbyPod(ctx context.Context, execID string, standbyPod *StandbyPod, command []string, env *models.Environment) {
	o.logger.Info("starting execution (standby pod)",
		zap.String("exec_id", execID),
		zap.String("pod", standbyPod.Name),
		zap.String("image", standbyPod.Image),
	)

	// Ensure pod cleanup after execution (standby pods are single-use)
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := o.k8sClient.DeletePod(cleanupCtx, standbyPod.Namespace, standbyPod.Name, true); err != nil {
			o.logger.Warn("failed to cleanup standby pod",
				zap.String("exec_id", execID),
				zap.String("pod", standbyPod.Name),
				zap.Error(err),
			)
		} else {
			o.logger.Debug("cleaned up standby pod",
				zap.String("exec_id", execID),
				zap.String("pod", standbyPod.Name),
			)
		}
	}()

	// Execute command in the standby pod
	startTime := time.Now()
	var stdoutBuf, stderrBuf bytes.Buffer

	err := o.k8sClient.ExecInPod(ctx, standbyPod.Namespace, standbyPod.Name, command, nil, &stdoutBuf, &stderrBuf)
	duration := time.Since(startTime)

	// Determine exit code (0 if no error, 1 otherwise)
	exitCode := 0
	if err != nil {
		exitCode = 1
	}

	// Update execution with results
	completedAt := time.Now()
	durationMs := duration.Milliseconds()
	o.execMutex.Lock()
	if e, exists := o.executions[execID]; exists {
		if err != nil {
			e.Status = models.ExecutionStatusFailed
			e.Error = err.Error()
		} else {
			e.Status = models.ExecutionStatusCompleted
		}
		e.CompletedAt = &completedAt
		e.ExitCode = &exitCode
		e.Stdout = stdoutBuf.String()
		e.Stderr = stderrBuf.String()
		e.DurationMs = &durationMs
	}
	o.execMutex.Unlock()

	o.logger.Info("execution completed (standby pod)",
		zap.String("exec_id", execID),
		zap.String("pod", standbyPod.Name),
		zap.Int("exit_code", exitCode),
		zap.Int64("duration_ms", durationMs),
	)
}

// GetExecution retrieves an execution by ID
func (o *Orchestrator) GetExecution(ctx context.Context, execID string) (*models.Execution, error) {
	o.execMutex.RLock()
	exec, exists := o.executions[execID]
	o.execMutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("execution not found")
	}

	// Return a copy
	execCopy := *exec
	return &execCopy, nil
}

// ListExecutions lists executions for an environment
func (o *Orchestrator) ListExecutions(ctx context.Context, envID string, limit int) (*models.ExecutionListResponse, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	o.execMutex.RLock()
	var executions []models.ExecutionResponse
	totalInMap := len(o.executions)
	for _, exec := range o.executions {
		if envID != "" && exec.EnvironmentID != envID {
			continue
		}
		executions = append(executions, models.ExecutionResponse{
			ID:            exec.ID,
			EnvironmentID: exec.EnvironmentID,
			Status:        exec.Status,
			CreatedAt:     exec.CreatedAt,
			StartedAt:     exec.StartedAt,
			CompletedAt:   exec.CompletedAt,
			ExitCode:      exec.ExitCode,
			Stdout:        exec.Stdout,
			Stderr:        exec.Stderr,
			Error:         exec.Error,
			DurationMs:    exec.DurationMs,
		})
	}
	o.execMutex.RUnlock()

	// Sort by creation time (newest first)
	sort.Slice(executions, func(i, j int) bool {
		return executions[i].CreatedAt.After(executions[j].CreatedAt)
	})

	// Apply limit
	if len(executions) > limit {
		executions = executions[:limit]
	}

	o.logger.Debug("listing executions",
		zap.String("environment_id", envID),
		zap.Int("total_in_map", totalInMap),
		zap.Int("matched", len(executions)),
		zap.Int("limit", limit),
	)

	return &models.ExecutionListResponse{
		Executions: executions,
		Total:      len(executions),
	}, nil
}

// CancelExecution cancels a running or queued execution
func (o *Orchestrator) CancelExecution(ctx context.Context, execID string) error {
	o.execMutex.Lock()
	exec, exists := o.executions[execID]
	if !exists {
		o.execMutex.Unlock()
		return fmt.Errorf("execution not found")
	}

	// Can only cancel pending, queued, or running executions
	if exec.Status != models.ExecutionStatusPending &&
		exec.Status != models.ExecutionStatusQueued &&
		exec.Status != models.ExecutionStatusRunning {
		o.execMutex.Unlock()
		return fmt.Errorf("execution cannot be canceled (status: %s)", exec.Status)
	}

	exec.Status = models.ExecutionStatusCanceled
	now := time.Now()
	exec.CompletedAt = &now
	exec.Error = "canceled by user"
	namespace := exec.Namespace
	podName := exec.PodName
	o.execMutex.Unlock()

	// Try to delete the pod if it exists
	if podName != "" && namespace != "" {
		if err := o.k8sClient.DeletePod(ctx, namespace, podName, true); err != nil {
			o.logger.Warn("failed to delete pod for canceled execution",
				zap.String("exec_id", execID),
				zap.Error(err),
			)
		}
	}

	o.logger.Info("execution canceled",
		zap.String("exec_id", execID),
	)

	return nil
}

// updateExecutionStatus updates the status of an execution
func (o *Orchestrator) updateExecutionStatus(execID string, status models.ExecutionStatus, timestamp *time.Time) {
	o.execMutex.Lock()
	defer o.execMutex.Unlock()

	if exec, exists := o.executions[execID]; exists {
		exec.Status = status
		if timestamp != nil {
			switch status {
			case models.ExecutionStatusQueued:
				exec.QueuedAt = timestamp
			case models.ExecutionStatusRunning:
				exec.StartedAt = timestamp
			case models.ExecutionStatusCompleted, models.ExecutionStatusFailed:
				exec.CompletedAt = timestamp
			}
		}
	}
}

// updateExecutionError marks an execution as failed with an error message
func (o *Orchestrator) updateExecutionError(execID string, errMsg string) {
	now := time.Now()
	o.execMutex.Lock()
	defer o.execMutex.Unlock()

	if exec, exists := o.executions[execID]; exists {
		exec.Status = models.ExecutionStatusFailed
		exec.CompletedAt = &now
		exec.Error = errMsg
	}
}

// ========== Standby Pod Pool Management ==========

// runPoolReplenishment runs in the background to maintain the standby pod pool
func (o *Orchestrator) runPoolReplenishment() {
	o.logger.Info("starting standby pod pool replenishment",
		zap.Int("target_size", o.config.Pool.Size),
		zap.String("default_image", o.config.Pool.DefaultImage),
	)

	// Initial pool creation
	o.replenishPool()

	// Periodic check to maintain pool size
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-o.poolStopChan:
			o.logger.Info("stopping standby pod pool replenishment")
			o.cleanupPool()
			return
		case <-ticker.C:
			o.replenishPool()
		}
	}
}

// replenishPool ensures the standby pool has the target number of pods
func (o *Orchestrator) replenishPool() {
	// Ensure ephemeral namespace exists
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := o.ensureEphemeralNamespace(ctx); err != nil {
		o.logger.Error("failed to ensure ephemeral namespace for pool", zap.Error(err))
		return
	}

	// Collect all images that need standby pods
	// Map of image -> target pool size
	imageTargets := make(map[string]int)

	// Add global default pool if enabled
	if o.config.Pool.Enabled && o.config.Pool.Size > 0 {
		imageTargets[o.config.Pool.DefaultImage] = o.config.Pool.Size
		o.logger.Debug("global pool target",
			zap.String("image", o.config.Pool.DefaultImage),
			zap.Int("size", o.config.Pool.Size),
		)
	}

	// Add per-environment pools
	o.envMutex.RLock()
	for _, env := range o.environments {
		if env.Pool != nil && env.Pool.Enabled && env.Status == models.StatusRunning {
			poolSize := env.Pool.Size
			if poolSize <= 0 {
				poolSize = 2 // Default pool size
			}
			// Use the larger value if same image has multiple configs
			if current, exists := imageTargets[env.Image]; !exists || poolSize > current {
				imageTargets[env.Image] = poolSize
				o.logger.Debug("per-environment pool target",
					zap.String("environment_id", env.ID),
					zap.String("image", env.Image),
					zap.Int("size", poolSize),
				)
			}
		}
	}
	o.envMutex.RUnlock()

	// Replenish pools for each image
	for image, targetSize := range imageTargets {
		o.standbyPoolMutex.Lock()
		currentSize := len(o.standbyPool[image])
		needed := targetSize - currentSize
		o.standbyPoolMutex.Unlock()

		if needed <= 0 {
			continue
		}

		o.logger.Debug("replenishing standby pool",
			zap.String("image", image),
			zap.Int("current", currentSize),
			zap.Int("target", targetSize),
			zap.Int("creating", needed),
		)

		for i := 0; i < needed; i++ {
			if err := o.createStandbyPod(image); err != nil {
				o.logger.Warn("failed to create standby pod",
					zap.String("image", image),
					zap.Error(err),
				)
			}
		}
	}
}

// createStandbyPod creates a new standby pod for the pool
func (o *Orchestrator) createStandbyPod(image string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	podID := uuid.New().String()[:8]
	podName := "standby-" + podID

	labels := map[string]string{
		"app":        "agentbox",
		"managed-by": "agentbox",
		"type":       "standby",
		"image-hash": hashImage(image),
	}

	// Create pod with sleep command (idle, waiting for exec)
	podSpec := &k8s.PodSpec{
		Name:         podName,
		Namespace:    o.ephemeralNamespace,
		Image:        image,
		Command:      []string{"/bin/sh", "-c", "trap 'exit 0' TERM; while true; do sleep 1; done"},
		CPU:          o.config.Pool.DefaultCPU,
		Memory:       o.config.Pool.DefaultMemory,
		RuntimeClass: o.config.Kubernetes.RuntimeClass,
		Labels:       labels,
	}

	if err := o.k8sClient.CreatePod(ctx, podSpec); err != nil {
		return fmt.Errorf("failed to create standby pod: %w", err)
	}

	// Wait for pod to be running
	if err := o.k8sClient.WaitForPodRunning(ctx, o.ephemeralNamespace, podName); err != nil {
		// Cleanup failed pod (best effort, ignore cleanup errors)
		if cleanupErr := o.k8sClient.DeletePod(ctx, o.ephemeralNamespace, podName, true); cleanupErr != nil {
			o.logger.Warn("failed to cleanup failed standby pod",
				zap.String("pod", podName),
				zap.Error(cleanupErr),
			)
		}
		return fmt.Errorf("standby pod failed to start: %w", err)
	}

	// Add to pool
	standbyPod := &StandbyPod{
		Name:      podName,
		Namespace: o.ephemeralNamespace,
		Image:     image,
		CreatedAt: time.Now(),
	}

	o.standbyPoolMutex.Lock()
	o.standbyPool[image] = append(o.standbyPool[image], standbyPod)
	o.standbyPoolMutex.Unlock()

	o.logger.Debug("created standby pod",
		zap.String("pod", podName),
		zap.String("image", image),
	)

	return nil
}

// claimStandbyPod attempts to claim a standby pod from the pool
// Returns nil if no matching pod is available
func (o *Orchestrator) claimStandbyPod(image string) *StandbyPod {
	o.standbyPoolMutex.Lock()
	defer o.standbyPoolMutex.Unlock()

	pods := o.standbyPool[image]
	if len(pods) == 0 {
		return nil
	}

	// Take the first available pod (FIFO)
	pod := pods[0]
	o.standbyPool[image] = pods[1:]

	o.logger.Debug("claimed standby pod",
		zap.String("pod", pod.Name),
		zap.String("image", image),
		zap.Int("remaining", len(o.standbyPool[image])),
	)

	// Trigger async pool replenishment to replace the claimed pod
	go o.replenishPool()

	return pod
}

// cleanupPool removes all standby pods (called on shutdown)
func (o *Orchestrator) cleanupPool() {
	o.standbyPoolMutex.Lock()
	defer o.standbyPoolMutex.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for image, pods := range o.standbyPool {
		for _, pod := range pods {
			if err := o.k8sClient.DeletePod(ctx, pod.Namespace, pod.Name, true); err != nil {
				o.logger.Warn("failed to delete standby pod",
					zap.String("pod", pod.Name),
					zap.Error(err),
				)
			}
		}
		o.standbyPool[image] = nil
	}

	o.logger.Info("cleaned up standby pod pool")
}

// GetPoolStatus returns the current status of the standby pool
func (o *Orchestrator) GetPoolStatus() map[string]int {
	o.standbyPoolMutex.Lock()
	defer o.standbyPoolMutex.Unlock()

	status := make(map[string]int)
	for image, pods := range o.standbyPool {
		status[image] = len(pods)
	}
	return status
}

// hashImage creates a short hash of an image name for use in labels
func hashImage(image string) string {
	// Simple hash: take first 8 chars of image name (sanitized)
	h := strings.ReplaceAll(image, "/", "-")
	h = strings.ReplaceAll(h, ":", "-")
	if len(h) > 63 {
		h = h[:63] // Kubernetes label value limit
	}
	return h
}

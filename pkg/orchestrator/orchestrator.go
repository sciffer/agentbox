package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sciffertbox/internal/config"
	"github.com/sciffertbox/internal/logger"
	"github.com/sciffertbox/pkg/k8s"
	"github.com/sciffertbox/pkg/models"
)

// Orchestrator manages environment lifecycle
type Orchestrator struct {
	k8sClient       *k8s.Client
	config          *config.Config
	logger          *logger.Logger
	environments    map[string]*models.Environment
	envMutex        sync.RWMutex
	namespacePrefix string
}

// New creates a new orchestrator instance
func New(k8sClient *k8s.Client, cfg *config.Config, log *logger.Logger) *Orchestrator {
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
	
	// Create Kubernetes resources asynchronously
	go func() {
		if err := o.provisionEnvironment(context.Background(), env); err != nil {
			o.logger.Error("failed to provision environment",
				"environment_id", envID,
				"error", err,
			)
			o.updateEnvironmentStatus(envID, models.StatusFailed)
		}
	}()
	
	return env, nil
}

// provisionEnvironment creates the Kubernetes resources
func (o *Orchestrator) provisionEnvironment(ctx context.Context, env *models.Environment) error {
	// Create namespace
	labels := map[string]string{
		"app":        "agentbox",
		"env-id":     env.ID,
		"managed-by": "agentbox",
	}
	for k, v := range env.Labels {
		labels[k] = v
	}
	
	if err := o.k8sClient.CreateNamespace(ctx, env.Namespace, labels); err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}
	
	// Create resource quota
	if err := o.k8sClient.CreateResourceQuota(
		ctx,
		env.Namespace,
		env.Resources.CPU,
		env.Resources.Memory,
		env.Resources.Storage,
	); err != nil {
		return fmt.Errorf("failed to create resource quota: %w", err)
	}
	
	// Apply network policy (isolation)
	if err := o.applyNetworkPolicy(ctx, env.Namespace); err != nil {
		return fmt.Errorf("failed to apply network policy: %w", err)
	}
	
	// Create pod
	podName := "main"
	command := env.Command
	if len(command) == 0 {
		command = []string{"/bin/sh", "-c", "sleep infinity"}
	}
	
	podSpec := &k8s.PodSpec{
		Name:         podName,
		Namespace:    env.Namespace,
		Image:        env.Image,
		Command:      command,
		Env:          env.Env,
		CPU:          env.Resources.CPU,
		Memory:       env.Resources.Memory,
		Storage:      env.Resources.Storage,
		RuntimeClass: o.config.Kubernetes.RuntimeClass,
		Labels:       labels,
	}
	
	if err := o.k8sClient.CreatePod(ctx, podSpec); err != nil {
		return fmt.Errorf("failed to create pod: %w", err)
	}
	
	// Wait for pod to be running
	waitCtx, cancel := context.WithTimeout(ctx, time.Duration(o.config.Timeouts.StartupTimeout)*time.Second)
	defer cancel()
	
	if err := o.k8sClient.WaitForPodRunning(waitCtx, env.Namespace, podName); err != nil {
		return fmt.Errorf("pod failed to start: %w", err)
	}
	
	// Update environment status
	now := time.Now()
	o.envMutex.Lock()
	if e, exists := o.environments[env.ID]; exists {
		e.Status = models.StatusRunning
		e.StartedAt = &now
	}
	o.envMutex.Unlock()
	
	o.logger.Info("environment provisioned successfully",
		"environment_id", env.ID,
		"namespace", env.Namespace,
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
	if env.Status == models.StatusRunning {
		pod, err := o.k8sClient.GetPod(ctx, env.Namespace, "main")
		if err == nil {
			env.Status = convertPodPhaseToStatus(pod.Status.Phase)
		}
	}
	
	return env, nil
}

// ListEnvironments lists all environments with optional filtering
func (o *Orchestrator) ListEnvironments(ctx context.Context, status *models.EnvironmentStatus, labelSelector string, limit, offset int) (*models.ListEnvironmentsResponse, error) {
	o.envMutex.RLock()
	defer o.envMutex.RUnlock()
	
	envs := make([]models.Environment, 0)
	
	for _, env := range o.environments {
		// Filter by status if specified
		if status != nil && env.Status != *status {
			continue
		}
		
		// Filter by label if specified
		if labelSelector != "" && !matchesLabelSelector(env.Labels, labelSelector) {
			continue
		}
		
		envs = append(envs, *env)
	}
	
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
	
	// Delete pod
	if err := o.k8sClient.DeletePod(ctx, env.Namespace, "main", force); err != nil {
		o.logger.Error("failed to delete pod", "error", err)
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
		"environment_id", envID,
		"namespace", env.Namespace,
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
	
	// Set timeout if specified
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
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

// Helper functions

func generateEnvironmentID() string {
	return "env-" + uuid.New().String()[:8]
}

func (o *Orchestrator) generateNamespace(envID string) string {
	return o.namespacePrefix + envID
}

func (o *Orchestrator) updateEnvironmentStatus(envID string, status models.EnvironmentStatus) {
	o.envMutex.Lock()
	defer o.envMutex.Unlock()
	
	if env, exists := o.environments[envID]; exists {
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

func matchesLabelSelector(labels map[string]string, selector string) bool {
	// Simple implementation: selector format "key=value"
	// For production, use proper label selector parsing
	return true // TODO: Implement proper label matching
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

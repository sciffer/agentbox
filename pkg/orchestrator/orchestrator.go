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
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/sciffer/agentbox/internal/config"
	"github.com/sciffer/agentbox/internal/logger"
	"github.com/sciffer/agentbox/pkg/database"
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
	db              *database.DB
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
	// standbyPool holds pre-warmed pods per environment; key is environment ID
	standbyPool      map[string][]*StandbyPod
	standbyPoolMutex sync.Mutex
	// poolStopChan signals the pool replenishment goroutine to stop
	poolStopChan chan struct{}
	// reconciliationStopChan signals the reconciliation loop to stop
	reconciliationStopChan chan struct{}
}

// MaxConcurrentProvisions is the maximum number of environments that can be
// provisioned in parallel. This prevents overwhelming the Kubernetes API.
const MaxConcurrentProvisions = 10

// MaxConcurrentExecutions is the maximum number of command executions that can
// run in parallel. This is separate from environment provisioning.
const MaxConcurrentExecutions = 20

// Kubernetes pod phases
const (
	podPhasePending = "Pending"
	podPhaseRunning = "Running"
)

// New creates a new orchestrator instance
func New(k8sClient k8s.ClientInterface, cfg *config.Config, log *logger.Logger, db *database.DB) *Orchestrator {
	o := &Orchestrator{
		k8sClient:              k8sClient,
		config:                 cfg,
		logger:                 log,
		db:                     db,
		environments:           make(map[string]*models.Environment),
		namespacePrefix:        cfg.Kubernetes.NamespacePrefix,
		provisionSem:           make(chan struct{}, MaxConcurrentProvisions),
		execSem:                make(chan struct{}, MaxConcurrentExecutions),
		executions:             make(map[string]*models.Execution),
		standbyPool:            make(map[string][]*StandbyPod),
		poolStopChan:           make(chan struct{}),
		reconciliationStopChan: make(chan struct{}),
	}

	// Load environments and executions from database on startup
	if db != nil {
		ctx := context.Background()
		if err := o.loadFromDatabase(ctx); err != nil {
			log.Error("failed to load from database on startup", zap.Error(err))
		}
	}

	// Start pool replenishment loop so per-environment standby pools work (env.Pool.Enabled);
	// when no env has pool enabled, replenishPool() is a no-op.
	go o.runPoolReplenishment()

	// Start reconciliation loop (handles pending/failed envs and missing pods)
	go o.runReconciliationLoop()

	return o
}

// Stop gracefully shuts down the orchestrator
func (o *Orchestrator) Stop() {
	close(o.poolStopChan)
	close(o.reconciliationStopChan)
}

// loadFromDatabase loads all environments and executions from the database
func (o *Orchestrator) loadFromDatabase(ctx context.Context) error {
	if o.db == nil {
		return nil
	}

	// Load environments
	envs, err := o.db.LoadAllEnvironments(ctx)
	if err != nil {
		return fmt.Errorf("failed to load environments: %w", err)
	}

	o.envMutex.Lock()
	for _, env := range envs {
		o.environments[env.ID] = env
	}
	o.envMutex.Unlock()

	o.logger.Info("loaded environments from database", zap.Int("count", len(envs)))

	// Load executions
	execs, err := o.db.LoadAllExecutions(ctx)
	if err != nil {
		return fmt.Errorf("failed to load executions: %w", err)
	}

	o.execMutex.Lock()
	for _, exec := range execs {
		o.executions[exec.ID] = exec
	}
	o.execMutex.Unlock()

	o.logger.Info("loaded executions from database", zap.Int("count", len(execs)))

	return nil
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

	// Store environment in memory and database
	o.envMutex.Lock()
	o.environments[envID] = env
	o.envMutex.Unlock()

	// Save to database
	if o.db != nil {
		if err := o.db.SaveEnvironment(ctx, env); err != nil {
			o.logger.Error("failed to save environment to database", zap.Error(err), zap.String("environment_id", envID))
			// Continue even if database save fails
		}
	}

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

	// Create resource quota: main pod + at least one exec pod (+ standby pool if enabled)
	quotaMultiplier := 2 // main + 1 ephemeral exec
	if env.Pool != nil && env.Pool.Enabled && env.Pool.Size > 0 {
		quotaMultiplier += env.Pool.Size
	}
	quotaCPU := multiplyResourceQuantity(envResources.CPU, quotaMultiplier)
	quotaMemory := multiplyResourceQuantity(envResources.Memory, quotaMultiplier)
	if err := o.k8sClient.CreateResourceQuota(
		ctx,
		envNamespace,
		quotaCPU,
		quotaMemory,
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

// refreshEnvironmentStatusFromK8s updates env status from the main pod when appropriate;
// returns a copy of env with possibly updated status and updates in-memory (and DB if updateDB).
func (o *Orchestrator) refreshEnvironmentStatusFromK8s(ctx context.Context, envID string, env *models.Environment, updateDB bool) models.Environment {
	envCopy := *env
	if env.Status == models.StatusRunning {
		pod, err := o.k8sClient.GetPod(ctx, env.Namespace, "main")
		if err == nil {
			newStatus := convertPodPhaseToStatus(string(pod.Status.Phase))
			if newStatus != models.StatusPending || pod.Status.Phase == podPhasePending {
				envCopy.Status = newStatus
				o.envMutex.Lock()
				if e, ok := o.environments[envID]; ok {
					e.Status = newStatus
				}
				o.envMutex.Unlock()
			}
		}
	} else if env.Status == models.StatusPending || env.Status == models.StatusFailed {
		pod, err := o.k8sClient.GetPod(ctx, env.Namespace, "main")
		if err == nil && pod.Status.Phase == podPhaseRunning {
			envCopy.Status = models.StatusRunning
			if updateDB {
				o.updateEnvironmentStatus(envID, models.StatusRunning)
			} else {
				o.envMutex.Lock()
				if e, ok := o.environments[envID]; ok {
					e.Status = models.StatusRunning
				}
				o.envMutex.Unlock()
			}
		}
	}
	return envCopy
}

// getEnvironmentReconciliationRetriesLeft returns maxRetries - count, clamped to >= 0.
func getEnvironmentReconciliationRetriesLeft(maxRetries int, count int) int {
	if maxRetries < 0 {
		maxRetries = 0
	}
	left := maxRetries - count
	if left < 0 {
		return 0
	}
	return left
}

// GetEnvironment retrieves an environment by ID.
// DB is source of truth: if not in DB, we purge from memory and return not found (so deleted envs never reappear).
func (o *Orchestrator) GetEnvironment(ctx context.Context, envID string) (*models.Environment, error) {
	if o.db != nil {
		env, err := o.db.GetEnvironment(ctx, envID)
		if err != nil {
			o.envMutex.Lock()
			delete(o.environments, envID)
			o.envMutex.Unlock()
			return nil, fmt.Errorf("environment not found")
		}
		o.envMutex.Lock()
		o.environments[envID] = env
		o.envMutex.Unlock()

		envCopy := o.refreshEnvironmentStatusFromK8s(ctx, envID, env, true)
		envCopy.ReconciliationRetriesLeft = getEnvironmentReconciliationRetriesLeft(o.config.Reconciliation.MaxRetries, envCopy.ReconciliationRetryCount)
		return &envCopy, nil
	}

	o.envMutex.RLock()
	env, exists := o.environments[envID]
	o.envMutex.RUnlock()
	if !exists {
		return nil, fmt.Errorf("environment not found")
	}

	envCopy := o.refreshEnvironmentStatusFromK8s(ctx, envID, env, false)
	envCopy.ReconciliationRetriesLeft = getEnvironmentReconciliationRetriesLeft(o.config.Reconciliation.MaxRetries, envCopy.ReconciliationRetryCount)
	return &envCopy, nil
}

// ListEnvironments lists all environments from the database (source of truth) with optional filtering.
// In-memory status is overlaid so live status (running/pending/failed) is shown.
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

	// List from database so deleted envs never appear (consistent across replicas)
	var base []*models.Environment
	if o.db != nil {
		fromDB, err := o.db.ListEnvironments(ctx, 1000, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to list environments from database: %w", err)
		}
		base = fromDB
	} else {
		// No DB: fallback to in-memory only (e.g. tests)
		o.envMutex.RLock()
		for _, env := range o.environments {
			envCopy := *env
			base = append(base, &envCopy)
		}
		o.envMutex.RUnlock()
	}

	// Overlay in-memory status so we have live status, then filter by status/label
	o.envMutex.RLock()
	filtered := make([]*models.Environment, 0, len(base))
	for _, env := range base {
		if inMem, ok := o.environments[env.ID]; ok {
			env = inMem
		}
		if status != nil && env.Status != *status {
			continue
		}
		if labelSelector != "" && !matchesLabelSelector(env.Labels, labelSelector) {
			continue
		}
		envCopy := *env
		filtered = append(filtered, &envCopy)
	}
	o.envMutex.RUnlock()

	total := len(filtered)
	start := offset
	end := offset + limit
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	if start >= end {
		return &models.ListEnvironmentsResponse{
			Environments: []models.Environment{},
			Total:        total,
			Limit:        limit,
			Offset:       offset,
		}, nil
	}

	page := filtered[start:end]
	result := make([]models.Environment, 0, len(page))
	maxRetries := o.config.Reconciliation.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	for _, env := range page {
		envCopy := *env
		left := maxRetries - envCopy.ReconciliationRetryCount
		if left < 0 {
			left = 0
		}
		envCopy.ReconciliationRetriesLeft = left
		result = append(result, envCopy)
	}

	return &models.ListEnvironmentsResponse{
		Environments: result,
		Total:        total,
		Limit:        limit,
		Offset:       offset,
	}, nil
}

// UpdateEnvironment applies a partial update to an environment (PATCH); only non-nil fields are updated
func (o *Orchestrator) UpdateEnvironment(ctx context.Context, envID string, patch *models.UpdateEnvironmentRequest) (*models.Environment, error) {
	o.envMutex.Lock()
	env, exists := o.environments[envID]
	if !exists {
		o.envMutex.Unlock()
		return nil, fmt.Errorf("environment not found")
	}
	// Apply patch
	if patch.Name != nil {
		env.Name = *patch.Name
	}
	if patch.Image != nil {
		env.Image = *patch.Image
	}
	if patch.Resources != nil {
		env.Resources = *patch.Resources
	}
	if patch.Timeout != nil {
		env.Timeout = *patch.Timeout
	}
	if patch.Env != nil {
		env.Env = *patch.Env
	}
	if patch.Command != nil {
		env.Command = *patch.Command
	}
	if patch.Labels != nil {
		env.Labels = *patch.Labels
	}
	if patch.NodeSelector != nil {
		env.NodeSelector = *patch.NodeSelector
	}
	if patch.Tolerations != nil {
		env.Tolerations = *patch.Tolerations
	}
	if patch.Isolation != nil {
		env.Isolation = patch.Isolation
	}
	if patch.Pool != nil {
		env.Pool = patch.Pool
	}
	o.envMutex.Unlock()

	if o.db != nil {
		if err := o.db.SaveEnvironment(ctx, env); err != nil {
			o.logger.Error("failed to save updated environment to database", zap.Error(err), zap.String("environment_id", envID))
			return nil, fmt.Errorf("failed to persist update: %w", err)
		}
	}

	envCopy := *env
	return &envCopy, nil
}

// DeleteEnvironment terminates and removes an environment.
// Deletes from DB first so all replicas stop listing it; then K8s; then memory.
// If env is not in memory (e.g. request hit another replica), loads from DB so delete can still succeed.
func (o *Orchestrator) DeleteEnvironment(ctx context.Context, envID string, force bool) error {
	var namespace string
	o.envMutex.Lock()
	env, exists := o.environments[envID]
	if exists {
		namespace = env.Namespace
		o.envMutex.Unlock()
	} else {
		o.envMutex.Unlock()
		// Not in memory: try DB so delete works when request hits a replica that never had this env (e.g. failed env only in DB)
		if o.db != nil {
			dbEnv, err := o.db.GetEnvironment(ctx, envID)
			if err != nil || dbEnv == nil {
				return fmt.Errorf("environment not found")
			}
			namespace = dbEnv.Namespace
		} else {
			return fmt.Errorf("environment not found")
		}
	}

	// Delete from database first so ListEnvironments (DB-backed) stops returning this env on all replicas
	if o.db != nil {
		if err := o.db.DeleteEnvironment(ctx, envID); err != nil {
			return fmt.Errorf("failed to delete environment from database: %w", err)
		}
	}

	// Delete pod (best effort - namespace may not exist if env never provisioned)
	if err := o.k8sClient.DeletePod(ctx, namespace, "main", force); err != nil {
		o.logger.Debug("delete pod (best effort)", zap.String("environment_id", envID), zap.String("namespace", namespace), zap.Error(err))
	}

	// Delete namespace (best effort - may not exist if provisioning failed)
	if err := o.k8sClient.DeleteNamespace(ctx, namespace); err != nil {
		o.logger.Debug("delete namespace (best effort)", zap.String("environment_id", envID), zap.String("namespace", namespace), zap.Error(err))
	}

	// Remove from memory so this replica stops serving it
	o.envMutex.Lock()
	delete(o.environments, envID)
	o.envMutex.Unlock()

	o.logger.Info("environment deleted",
		zap.String("environment_id", envID),
		zap.String("namespace", namespace),
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

// GetLogs retrieves logs from an environment (pod logs merged with reconciliation events for the logs tab)
func (o *Orchestrator) GetLogs(ctx context.Context, envID string, tailLines *int64) (*models.LogsResponse, error) {
	env, err := o.GetEnvironment(ctx, envID)
	if err != nil {
		return nil, err
	}

	var logs []models.LogEntry

	// Fetch reconciliation/lifecycle events for this environment
	if o.db != nil {
		events, errEvents := o.db.ListEnvironmentEvents(ctx, envID, 500)
		if errEvents == nil {
			for _, e := range events {
				msg := e.Message
				if e.Details != "" {
					msg = msg + " — " + e.Details
				}
				logs = append(logs, models.LogEntry{
					Timestamp: e.CreatedAt,
					Stream:    "reconciliation",
					Message:   "[" + e.EventType + "] " + msg,
				})
			}
		}
	}

	// Get logs from the pod (if it exists)
	podLogsStr, err := o.k8sClient.GetPodLogs(ctx, env.Namespace, "main", tailLines)
	if err == nil {
		lines := strings.Split(podLogsStr, "\n")
		now := time.Now()
		for _, line := range lines {
			if line != "" {
				logs = append(logs, models.LogEntry{
					Timestamp: now,
					Stream:    "stdout",
					Message:   line,
				})
			}
		}
	}
	// If pod doesn't exist (e.g. pending/failed), we still return reconciliation events

	// Sort by timestamp so reconciliation events appear in order with pod logs
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Timestamp.Before(logs[j].Timestamp)
	})

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
	var env *models.Environment
	var exists bool
	if env, exists = o.environments[envID]; exists {
		// Atomically update status to avoid race conditions
		env.Status = status
	}
	o.envMutex.Unlock()

	// Save to database
	if exists && o.db != nil {
		ctx := context.Background()
		var startedAt *time.Time
		if status == models.StatusRunning && env.StartedAt == nil {
			now := time.Now()
			startedAt = &now
			env.StartedAt = startedAt
		} else if env.StartedAt != nil {
			startedAt = env.StartedAt
		}
		if err := o.db.UpdateEnvironmentStatus(ctx, envID, status, startedAt); err != nil {
			o.logger.Error("failed to update environment status in database", zap.Error(err), zap.String("environment_id", envID))
		}
		// Also save full environment to ensure all fields are synced
		if err := o.db.SaveEnvironment(ctx, env); err != nil {
			o.logger.Error("failed to save environment to database", zap.Error(err), zap.String("environment_id", envID))
		}
	}
}

// multiplyResourceQuantity returns a resource string equivalent to (base * multiplier), e.g. "500m" * 2 = "1000m".
func multiplyResourceQuantity(base string, multiplier int) string {
	if multiplier <= 0 {
		return "0"
	}
	q := resource.MustParse(base)
	for i := 1; i < multiplier; i++ {
		q.Add(resource.MustParse(base))
	}
	return q.String()
}

func convertPodPhaseToStatus(phase string) models.EnvironmentStatus {
	switch phase {
	case podPhasePending:
		return models.StatusPending
	case podPhaseRunning:
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

// runExecutionInMainPod runs the command in the environment's main pod and updates the execution record (used when ephemeral pod creation fails e.g. quota).
func (o *Orchestrator) runExecutionInMainPod(ctx context.Context, execID, namespace string, command []string, env *models.Environment) {
	startTime := time.Now()
	stdout, stderr, exitCode, err := o.executeInPod(ctx, namespace, "main", command)
	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()

	if err != nil {
		o.updateExecutionError(execID, fmt.Sprintf("execution failed: %v", err))
		return
	}

	completedAt := time.Now()
	o.execMutex.Lock()
	var exec *models.Execution
	var exists bool
	if exec, exists = o.executions[execID]; exists {
		exec.Status = models.ExecutionStatusCompleted
		exec.CompletedAt = &completedAt
		exec.ExitCode = &exitCode
		exec.Stdout = stdout
		exec.Stderr = stderr
		exec.DurationMs = &durationMs
	}
	o.execMutex.Unlock()

	if exists && o.db != nil {
		dbCtx := context.Background()
		if err := o.db.SaveExecution(dbCtx, exec); err != nil {
			o.logger.Error("failed to save execution results to database", zap.Error(err), zap.String("execution_id", execID))
		}
	}
}

// EphemeralExecRequest contains parameters for ephemeral execution
type EphemeralExecRequest struct {
	EnvironmentID string            `json:"environment_id"` // Reference to environment for config
	Command       []string          `json:"command"`
	Timeout       int               `json:"timeout,omitempty"`
	Env           map[string]string `json:"env,omitempty"` // Additional env vars (merged with environment's)
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
		Namespace:     env.Namespace, // Use environment's namespace
		CreatedAt:     now,
	}

	// Store execution in memory and database
	o.execMutex.Lock()
	o.executions[execID] = exec
	o.execMutex.Unlock()

	// Save to database
	if o.db != nil {
		if err := o.db.SaveExecution(ctx, exec); err != nil {
			o.logger.Error("failed to save execution to database", zap.Error(err), zap.String("execution_id", execID))
			// Continue even if database save fails
		}
	}

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
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	o.updateExecutionStatus(execID, models.ExecutionStatusQueued, nil)

	select {
	case o.execSem <- struct{}{}:
		defer func() { <-o.execSem }()
	case <-ctx.Done():
		o.updateExecutionError(execID, "timeout waiting in queue")
		return
	}

	standbyPod := o.claimStandbyPod(env.ID)

	// If canceled while queued, don't overwrite with Running
	o.execMutex.Lock()
	exec, exists := o.executions[execID]
	if !exists {
		o.execMutex.Unlock()
		return
	}
	if exec.Status == models.ExecutionStatusCanceled {
		o.execMutex.Unlock()
		return
	}
	now := time.Now()
	exec.Status = models.ExecutionStatusRunning
	exec.StartedAt = &now
	exec.QueuedAt = &now
	if standbyPod != nil {
		exec.PodName = standbyPod.Name
		exec.Namespace = standbyPod.Namespace
	}
	o.execMutex.Unlock()

	if o.db != nil {
		o.execMutex.RLock()
		execForDB := o.executions[execID]
		o.execMutex.RUnlock()
		if execForDB != nil {
			dbCtx := context.Background()
			if err := o.db.SaveExecution(dbCtx, execForDB); err != nil {
				o.logger.Error("failed to save execution status", zap.Error(err), zap.String("execution_id", execID))
			}
		}
	}

	o.execMutex.RLock()
	execRecord := o.executions[execID]
	namespace := execRecord.Namespace
	podName := execRecord.PodName
	o.execMutex.RUnlock()

	if standbyPod != nil {
		o.runWithStandbyPod(ctx, execID, standbyPod, req.Command, env)
		return
	}

	o.runExecutionWithNewPod(ctx, execID, env, req, namespace, podName, execRecord)
}

// runExecutionWithNewPod creates an ephemeral pod for the execution, waits for completion, and updates the execution record.
func (o *Orchestrator) runExecutionWithNewPod(
	ctx context.Context, execID string, env *models.Environment, req *EphemeralExecRequest,
	namespace, podName string, execRecord *models.Execution,
) {
	if execRecord == nil {
		o.updateExecutionError(execID, "execution record not found")
		return
	}
	if namespace == "" {
		namespace = env.Namespace
	}
	if podName == "" {
		podName = execID
	}
	o.execMutex.RLock()
	current := o.executions[execID]
	canceled := current != nil && current.Status == models.ExecutionStatusCanceled
	o.execMutex.RUnlock()
	if canceled {
		return
	}

	o.logger.Info("starting execution (new pod)",
		zap.String("exec_id", execID),
		zap.String("pod", podName),
		zap.String("namespace", namespace),
		zap.String("image", env.Image),
	)

	podSpec := o.buildEphemeralPodSpec(env, req, execID, namespace, podName, execRecord)
	fallbackToMain, createErr := o.tryCreateEphemeralPodOrFallback(ctx, execID, namespace, podSpec, req, env)
	if fallbackToMain {
		return
	}
	if createErr != nil {
		o.updateExecutionError(execID, fmt.Sprintf("failed to create pod: %v", createErr))
		return
	}

	defer o.cleanupEphemeralPod(execID, namespace, podName)

	startTime := time.Now()
	result, err := o.k8sClient.WaitForPodCompletion(ctx, namespace, podName)
	duration := time.Since(startTime)
	if err != nil {
		o.updateExecutionError(execID, fmt.Sprintf("execution failed: %v", err))
		return
	}

	o.recordEphemeralExecutionCompletion(ctx, execID, podName, result, duration)
}

// buildEphemeralPodSpec builds a PodSpec for an ephemeral execution pod.
func (o *Orchestrator) buildEphemeralPodSpec(
	env *models.Environment, req *EphemeralExecRequest, execID, namespace, podName string,
	execRecord *models.Execution,
) *k8s.PodSpec {
	labels := map[string]string{
		"app":            "agentbox",
		"exec-id":        execID,
		"managed-by":     "agentbox",
		"type":           "ephemeral",
		"user-id":        execRecord.UserID,
		"environment-id": req.EnvironmentID,
	}
	for k, v := range env.Labels {
		labels[k] = v
	}
	mergedEnv := make(map[string]string)
	for k, v := range env.Env {
		mergedEnv[k] = v
	}
	for k, v := range req.Env {
		mergedEnv[k] = v
	}
	runtimeClass := o.config.Kubernetes.RuntimeClass
	if env.Isolation != nil && env.Isolation.RuntimeClass != "" {
		runtimeClass = env.Isolation.RuntimeClass
	}
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
	return &k8s.PodSpec{
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
}

// tryCreateEphemeralPodOrFallback creates the pod; on quota/forbidden error runs in main pod.
// Returns (true, nil) when fallback to main pod was used, (false, err) on create error, (false, nil) on success.
func (o *Orchestrator) tryCreateEphemeralPodOrFallback(
	ctx context.Context, execID, namespace string, podSpec *k8s.PodSpec, req *EphemeralExecRequest, env *models.Environment,
) (fallbackToMain bool, err error) {
	err = o.k8sClient.CreatePod(ctx, podSpec)
	if err == nil {
		return false, nil
	}
	errStr := err.Error()
	if strings.Contains(errStr, "exceeded quota") || strings.Contains(errStr, "forbidden") {
		o.logger.Warn("ephemeral pod creation failed (quota); running in main pod — execution is not in a clean sandbox",
			zap.String("exec_id", execID),
			zap.String("namespace", namespace),
		)
		o.runExecutionInMainPod(ctx, execID, namespace, req.Command, env)
		return true, nil
	}
	return false, err
}

// cleanupEphemeralPod deletes the ephemeral pod after execution (best-effort).
func (o *Orchestrator) cleanupEphemeralPod(execID, namespace, podName string) {
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
}

// recordEphemeralExecutionCompletion updates execution record, persists to DB, and logs completion.
func (o *Orchestrator) recordEphemeralExecutionCompletion(
	ctx context.Context, execID, podName string, result *k8s.PodCompletionResult, duration time.Duration,
) {
	completedAt := time.Now()
	durationMs := duration.Milliseconds()
	o.execMutex.Lock()
	var exec *models.Execution
	var exists bool
	if exec, exists = o.executions[execID]; exists && exec.Status != models.ExecutionStatusCanceled {
		exec.Status = models.ExecutionStatusCompleted
		exec.CompletedAt = &completedAt
		exec.ExitCode = &result.ExitCode
		exec.Stdout = result.Logs
		exec.DurationMs = &durationMs
	}
	o.execMutex.Unlock()

	if !exists {
		return
	}
	if o.db != nil {
		if err := o.db.SaveExecution(ctx, exec); err != nil {
			o.logger.Error("failed to save execution results to database", zap.Error(err), zap.String("execution_id", execID))
		}
	}
	o.logger.Info("execution completed",
		zap.String("exec_id", execID),
		zap.String("pod", podName),
		zap.Int("exit_code", result.ExitCode),
		zap.Int64("duration_ms", durationMs),
	)
}

// runWithStandbyPod executes a command in a pre-warmed standby pod (single-use; pod is deleted after)
func (o *Orchestrator) runWithStandbyPod(ctx context.Context, execID string, standbyPod *StandbyPod, command []string, env *models.Environment) {
	o.logger.Info("starting execution (standby pod)",
		zap.String("exec_id", execID),
		zap.String("pod", standbyPod.Name),
		zap.String("namespace", standbyPod.Namespace),
		zap.String("image", standbyPod.Image),
	)

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

	startTime := time.Now()
	var stdoutBuf, stderrBuf bytes.Buffer
	err := o.k8sClient.ExecInPod(ctx, standbyPod.Namespace, standbyPod.Name, command, nil, &stdoutBuf, &stderrBuf)
	duration := time.Since(startTime)

	exitCode := 0
	if err != nil {
		exitCode = 1
	}

	completedAt := time.Now()
	durationMs := duration.Milliseconds()
	o.execMutex.Lock()
	var exec *models.Execution
	var exists bool
	if exec, exists = o.executions[execID]; exists {
		if err != nil {
			exec.Status = models.ExecutionStatusFailed
			exec.Error = err.Error()
		} else {
			exec.Status = models.ExecutionStatusCompleted
		}
		exec.CompletedAt = &completedAt
		exec.ExitCode = &exitCode
		exec.Stdout = stdoutBuf.String()
		exec.Stderr = stderrBuf.String()
		exec.DurationMs = &durationMs
	}
	o.execMutex.Unlock()

	if exists && o.db != nil {
		if err := o.db.SaveExecution(ctx, exec); err != nil {
			o.logger.Error("failed to save execution results to database", zap.Error(err), zap.String("execution_id", execID))
		}
	}

	o.logger.Info("execution completed (standby pod)",
		zap.String("exec_id", execID),
		zap.String("pod", standbyPod.Name),
		zap.Int("exit_code", exitCode),
		zap.Int64("duration_ms", durationMs),
	)

	// Trigger replenishment for this environment
	go o.replenishPool()
}

// GetExecution retrieves an execution by ID
func (o *Orchestrator) GetExecution(ctx context.Context, execID string) (*models.Execution, error) {
	// Try database first (for persistence across restarts)
	if o.db != nil {
		if exec, err := o.db.GetExecution(ctx, execID); err == nil {
			// Also update in-memory cache
			o.execMutex.Lock()
			o.executions[execID] = exec
			o.execMutex.Unlock()
			// Return a copy
			execCopy := *exec
			return &execCopy, nil
		}
	}

	// Fallback to in-memory
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

	// Try database first (for persistence across restarts)
	var execs []*models.Execution
	var err error
	if o.db != nil {
		execs, err = o.db.ListExecutions(ctx, envID, limit)
		if err == nil {
			// Update in-memory cache
			o.execMutex.Lock()
			for _, exec := range execs {
				o.executions[exec.ID] = exec
			}
			o.execMutex.Unlock()

			// Convert to response format
			executions := make([]models.ExecutionResponse, len(execs))
			for i, exec := range execs {
				executions[i] = models.ExecutionResponse{
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
				}
			}

			o.logger.Debug("listing executions from database",
				zap.String("environment_id", envID),
				zap.Int("count", len(executions)),
				zap.Int("limit", limit),
			)

			return &models.ExecutionListResponse{
				Executions: executions,
				Total:      len(executions),
			}, nil
		}
		// Fall through to in-memory if database query fails
		o.logger.Warn("failed to list executions from database, falling back to in-memory", zap.Error(err))
	}

	// Fallback to in-memory
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

	o.logger.Debug("listing executions from memory",
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

	// Save to database
	if o.db != nil {
		if err := o.db.SaveExecution(ctx, exec); err != nil {
			o.logger.Error("failed to save canceled execution to database", zap.Error(err), zap.String("execution_id", execID))
		}
	}

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
	var exec *models.Execution
	var exists bool
	if exec, exists = o.executions[execID]; exists {
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
	o.execMutex.Unlock()

	// Save to database
	if exists && o.db != nil {
		ctx := context.Background()
		if err := o.db.SaveExecution(ctx, exec); err != nil {
			o.logger.Error("failed to update execution status in database", zap.Error(err), zap.String("execution_id", execID))
		}
	}
}

// updateExecutionError marks an execution as failed with an error message
func (o *Orchestrator) updateExecutionError(execID string, errMsg string) {
	now := time.Now()
	o.execMutex.Lock()
	var exec *models.Execution
	var exists bool
	if exec, exists = o.executions[execID]; exists {
		exec.Status = models.ExecutionStatusFailed
		exec.CompletedAt = &now
		exec.Error = errMsg
	}
	o.execMutex.Unlock()

	// Save to database
	if exists && o.db != nil {
		ctx := context.Background()
		if err := o.db.SaveExecution(ctx, exec); err != nil {
			o.logger.Error("failed to update execution error in database", zap.Error(err), zap.String("execution_id", execID))
		}
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

// replenishPool ensures each environment with pool enabled has the target number of standby pods
func (o *Orchestrator) replenishPool() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	o.envMutex.RLock()
	envsToReplenish := make([]*models.Environment, 0, len(o.environments))
	for _, env := range o.environments {
		if env.Pool != nil && env.Pool.Enabled && env.Status == models.StatusRunning {
			envsToReplenish = append(envsToReplenish, env)
		}
	}
	o.envMutex.RUnlock()

	for _, env := range envsToReplenish {
		poolSize := env.Pool.Size
		if poolSize <= 0 {
			poolSize = 2
		}
		o.standbyPoolMutex.Lock()
		current := len(o.standbyPool[env.ID])
		needed := poolSize - current
		o.standbyPoolMutex.Unlock()

		if needed <= 0 {
			continue
		}

		o.logger.Debug("replenishing standby pool",
			zap.String("environment_id", env.ID),
			zap.Int("current", current),
			zap.Int("target", poolSize),
			zap.Int("creating", needed),
		)

		for i := 0; i < needed; i++ {
			if err := o.createStandbyPod(ctx, env); err != nil {
				o.logger.Warn("failed to create standby pod",
					zap.String("environment_id", env.ID),
					zap.Error(err),
				)
			}
		}
	}
}

// createStandbyPod creates one standby pod in the environment's namespace with a unique name
func (o *Orchestrator) createStandbyPod(ctx context.Context, env *models.Environment) error {
	podName := "standby-" + uuid.New().String()[:8]

	runtimeClass := o.config.Kubernetes.RuntimeClass
	if env.Isolation != nil && env.Isolation.RuntimeClass != "" {
		runtimeClass = env.Isolation.RuntimeClass
	}
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

	labels := map[string]string{
		"app":            "agentbox",
		"managed-by":     "agentbox",
		"type":           "standby",
		"environment-id": env.ID,
	}
	for k, v := range env.Labels {
		labels[k] = v
	}

	cpu := env.Resources.CPU
	mem := env.Resources.Memory
	if cpu == "" {
		cpu = o.config.Pool.DefaultCPU
	}
	if mem == "" {
		mem = o.config.Pool.DefaultMemory
	}

	podSpec := &k8s.PodSpec{
		Name:            podName,
		Namespace:       env.Namespace,
		Image:           env.Image,
		Command:         []string{"/bin/sh", "-c", "trap 'exit 0' TERM; while true; do sleep 1; done"},
		CPU:             cpu,
		Memory:          mem,
		Storage:         env.Resources.Storage,
		RuntimeClass:    runtimeClass,
		Labels:          labels,
		NodeSelector:    env.NodeSelector,
		Tolerations:     k8sTolerations,
		SecurityContext: securityContext,
	}

	if err := o.k8sClient.CreatePod(ctx, podSpec); err != nil {
		return fmt.Errorf("create standby pod: %w", err)
	}

	if err := o.k8sClient.WaitForPodRunning(ctx, env.Namespace, podName); err != nil {
		if delErr := o.k8sClient.DeletePod(ctx, env.Namespace, podName, true); delErr != nil {
			o.logger.Warn("failed to delete standby pod after start failure", zap.Error(delErr), zap.String("pod", podName), zap.String("namespace", env.Namespace))
		}
		return fmt.Errorf("standby pod failed to start: %w", err)
	}

	standbyPod := &StandbyPod{
		Name:      podName,
		Namespace: env.Namespace,
		Image:     env.Image,
		CreatedAt: time.Now(),
	}

	o.standbyPoolMutex.Lock()
	o.standbyPool[env.ID] = append(o.standbyPool[env.ID], standbyPod)
	o.standbyPoolMutex.Unlock()

	o.logger.Debug("created standby pod",
		zap.String("pod", podName),
		zap.String("namespace", env.Namespace),
		zap.String("environment_id", env.ID),
	)
	return nil
}

// claimStandbyPod takes one standby pod from the pool for the environment; returns nil if none available
func (o *Orchestrator) claimStandbyPod(envID string) *StandbyPod {
	o.standbyPoolMutex.Lock()
	defer o.standbyPoolMutex.Unlock()

	pods := o.standbyPool[envID]
	if len(pods) == 0 {
		return nil
	}

	pod := pods[0]
	o.standbyPool[envID] = pods[1:]

	o.logger.Debug("claimed standby pod",
		zap.String("pod", pod.Name),
		zap.String("namespace", pod.Namespace),
		zap.String("environment_id", envID),
		zap.Int("remaining", len(o.standbyPool[envID])),
	)

	go o.replenishPool()
	return pod
}

// cleanupPool removes all standby pods (called on shutdown)
func (o *Orchestrator) cleanupPool() {
	o.standbyPoolMutex.Lock()
	defer o.standbyPoolMutex.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for envID, pods := range o.standbyPool {
		for _, pod := range pods {
			if err := o.k8sClient.DeletePod(ctx, pod.Namespace, pod.Name, true); err != nil {
				o.logger.Warn("failed to delete standby pod",
					zap.String("pod", pod.Name),
					zap.String("environment_id", envID),
					zap.Error(err),
				)
			}
		}
		o.standbyPool[envID] = nil
	}

	o.logger.Info("cleaned up standby pod pool")
}

// GetPoolStatus returns per-environment standby pool counts (key = environment ID)
func (o *Orchestrator) GetPoolStatus() map[string]int {
	o.standbyPoolMutex.Lock()
	defer o.standbyPoolMutex.Unlock()

	status := make(map[string]int)
	for envID, pods := range o.standbyPool {
		status[envID] = len(pods)
	}
	return status
}

// ========== Reconciliation Loop ==========

// runReconciliationLoop runs periodically to reconcile pending/failed environments and restore missing pods
func (o *Orchestrator) runReconciliationLoop() {
	interval := time.Duration(o.config.Reconciliation.IntervalSeconds) * time.Second
	if interval < 10*time.Second {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	o.logger.Info("reconciliation loop started",
		zap.Duration("interval", interval),
		zap.Int("max_retries", o.config.Reconciliation.MaxRetries),
	)

	for {
		select {
		case <-o.reconciliationStopChan:
			o.logger.Info("reconciliation loop stopped")
			return
		case <-ticker.C:
			o.logger.Info("reconciliation cycle starting")
			o.reconcileAll()
			o.logger.Info("reconciliation cycle completed")
		}
	}
}

// reconcileAll iterates over environments and reconciles those that need it.
// Only reconciles envs that still exist in the DB (so deleted envs are skipped on all replicas).
func (o *Orchestrator) reconcileAll() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// When DB is present, only reconcile envs that exist in DB (avoids reconciling deleted envs on other replicas)
	var inDB map[string]struct{}
	if o.db != nil {
		list, err := o.db.ListEnvironments(ctx, 10000, 0)
		if err != nil {
			o.logger.Warn("reconciliation: failed to list environments from DB", zap.Error(err))
			return
		}
		inDB = make(map[string]struct{}, len(list))
		for _, e := range list {
			inDB[e.ID] = struct{}{}
		}
	}

	o.envMutex.RLock()
	envList := make([]*models.Environment, 0, len(o.environments))
	for _, env := range o.environments {
		if inDB != nil {
			if _, ok := inDB[env.ID]; !ok {
				continue // Deleted from DB, skip reconciliation
			}
		}
		if env.Status == models.StatusTerminating || env.Status == models.StatusTerminated {
			continue
		}
		envCopy := *env
		envList = append(envList, &envCopy)
	}
	o.envMutex.RUnlock()

	o.logger.Debug("reconciliation: envs in scope",
		zap.Int("count", len(envList)),
		zap.Int("total_in_memory", len(o.environments)),
	)

	maxRetries := o.config.Reconciliation.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}

	for _, env := range envList {
		// Pending or Failed: retry provisioning if retries left
		if env.Status == models.StatusPending || env.Status == models.StatusFailed {
			if env.ReconciliationRetryCount >= maxRetries {
				continue // Already exceeded retries; user can use "Retry" button to reset
			}
			o.reconcilePendingOrFailed(ctx, env)
			continue
		}

		// Running: ensure main pod exists
		if env.Status == models.StatusRunning {
			o.reconcileRunning(ctx, env)
		}
	}

	// Replenish standby pools so Running envs with pool enabled get standby pods
	// even if the pool ticker hasn't run yet or replenishment previously failed
	o.replenishPool()
}

// reconcilePendingOrFailed retries provisioning for a pending or failed environment
func (o *Orchestrator) reconcilePendingOrFailed(ctx context.Context, env *models.Environment) {
	envID := env.ID
	envNamespace := env.Namespace
	maxRetries := o.config.Reconciliation.MaxRetries
	retryCount := env.ReconciliationRetryCount

	o.logReconciliationEvent(envID, "reconciliation_start", "Reconciliation attempt started", fmt.Sprintf("attempt %d of %d", retryCount+1, maxRetries))

	// Delete main pod if it exists (e.g. stuck Pending/Failed) so provisionEnvironment can recreate
	if errDel := o.k8sClient.DeletePod(ctx, envNamespace, "main", true); errDel != nil {
		o.logger.Debug("delete pod before reconciliation (best-effort)", zap.String("namespace", envNamespace), zap.Error(errDel))
	}

	// Re-acquire env from map for latest spec
	o.envMutex.RLock()
	envToProvision, exists := o.environments[envID]
	o.envMutex.RUnlock()
	if !exists {
		return
	}

	provisionCtx, cancel := context.WithTimeout(context.Background(), time.Duration(o.config.Timeouts.StartupTimeout)*time.Second)
	defer cancel()

	// Try provisioning (reuses existing namespace/quota/network if present)
	if err := o.provisionEnvironment(provisionCtx, envToProvision); err != nil {
		now := time.Now()
		newCount := envToProvision.ReconciliationRetryCount + 1
		errMsg := err.Error()

		o.envMutex.Lock()
		if e, ok := o.environments[envID]; ok {
			e.ReconciliationRetryCount = newCount
			e.LastReconciliationError = errMsg
			e.LastReconciliationAt = &now
		}
		o.envMutex.Unlock()

		if o.db != nil {
			if errDB := o.db.UpdateEnvironmentReconciliationState(ctx, envID, newCount, errMsg, &now); errDB != nil {
				o.logger.Warn("failed to update environment reconciliation state", zap.String("env_id", envID), zap.Error(errDB))
			}
		}

		o.logReconciliationEvent(envID, "reconciliation_failure", "Reconciliation failed", errMsg)

		if newCount >= maxRetries {
			o.updateEnvironmentStatus(envID, models.StatusFailed)
			o.logReconciliationEvent(envID, "reconciliation_max_retries",
				"Max reconciliation retries exceeded; use Retry button to try again",
				fmt.Sprintf("attempts: %d", newCount))
		}
		return
	}

	// Success: reset retry state
	o.envMutex.Lock()
	if e, ok := o.environments[envID]; ok {
		e.ReconciliationRetryCount = 0
		e.LastReconciliationError = ""
		e.LastReconciliationAt = nil
	}
	o.envMutex.Unlock()

	if o.db != nil {
		if errDB := o.db.UpdateEnvironmentReconciliationState(ctx, envID, 0, "", nil); errDB != nil {
			o.logger.Warn("failed to reset environment reconciliation state", zap.String("env_id", envID), zap.Error(errDB))
		}
	}

	o.logReconciliationEvent(envID, "reconciliation_success", "Environment provisioned successfully", "")
}

// reconcileRunning ensures the main pod exists for a running environment; recreates if missing
func (o *Orchestrator) reconcileRunning(ctx context.Context, env *models.Environment) {
	_, err := o.k8sClient.GetPod(ctx, env.Namespace, "main")
	if err == nil {
		return // Pod exists
	}

	o.logReconciliationEvent(env.ID, "reconciliation_pod_missing", "Main pod not found; recreating", "")

	o.envMutex.RLock()
	envCurrent, exists := o.environments[env.ID]
	o.envMutex.RUnlock()
	if !exists {
		return
	}

	if err := o.ensureMainPod(ctx, envCurrent); err != nil {
		o.logReconciliationEvent(env.ID, "reconciliation_failure", "Failed to recreate main pod", err.Error())
		return
	}

	o.logReconciliationEvent(env.ID, "reconciliation_success", "Main pod recreated successfully", "")
}

// ensureMainPod creates the main pod in an existing namespace and waits for running (used when pod is missing)
func (o *Orchestrator) ensureMainPod(ctx context.Context, env *models.Environment) error {
	envNamespace := env.Namespace
	envImage := env.Image
	envCommand := env.Command
	if len(envCommand) == 0 {
		envCommand = []string{"/bin/sh", "-c", "sleep infinity"}
	}
	envResources := env.Resources
	envEnvVars := env.Env
	envLabels := env.Labels
	envNodeSelector := env.NodeSelector
	envTolerations := env.Tolerations
	envIsolation := env.Isolation

	labels := map[string]string{"app": "agentbox", "env-id": env.ID, "managed-by": "agentbox"}
	for k, v := range envLabels {
		labels[k] = v
	}

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

	runtimeClass := o.config.Kubernetes.RuntimeClass
	if envIsolation != nil && envIsolation.RuntimeClass != "" {
		runtimeClass = envIsolation.RuntimeClass
	}
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
		Name:            "main",
		Namespace:       envNamespace,
		Image:           envImage,
		Command:         envCommand,
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
		return fmt.Errorf("create pod: %w", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, time.Duration(o.config.Timeouts.StartupTimeout)*time.Second)
	defer cancel()

	if err := o.k8sClient.WaitForPodRunning(waitCtx, envNamespace, "main"); err != nil {
		return fmt.Errorf("pod failed to start: %w", err)
	}

	return nil
}

// logReconciliationEvent persists a reconciliation event to the DB for display in environment logs
func (o *Orchestrator) logReconciliationEvent(envID, eventType, message, details string) {
	if o.db == nil {
		return
	}
	ctx := context.Background()
	if _, err := o.db.SaveEnvironmentEvent(ctx, envID, eventType, message, details); err != nil {
		o.logger.Warn("failed to save reconciliation event", zap.String("environment_id", envID), zap.Error(err))
	}
}

// RetryReconciliation resets retry count and triggers one reconciliation attempt (for "Retry" button)
func (o *Orchestrator) RetryReconciliation(ctx context.Context, envID string) error {
	o.envMutex.Lock()
	env, exists := o.environments[envID]
	if !exists {
		o.envMutex.Unlock()
		return fmt.Errorf("environment not found")
	}
	env.ReconciliationRetryCount = 0
	env.LastReconciliationError = ""
	env.LastReconciliationAt = nil
	o.envMutex.Unlock()

	if o.db != nil {
		if err := o.db.UpdateEnvironmentReconciliationState(ctx, envID, 0, "", nil); err != nil {
			o.logger.Error("failed to reset reconciliation state in database", zap.Error(err), zap.String("environment_id", envID))
		}
	}

	o.logReconciliationEvent(envID, "reconciliation_retry", "Manual retry requested", "")

	// Trigger one reconciliation attempt in background
	go func() {
		rctx, cancel := context.WithTimeout(context.Background(), time.Duration(o.config.Timeouts.StartupTimeout)*time.Second)
		defer cancel()
		o.envMutex.RLock()
		envForReconcile, ok := o.environments[envID]
		if !ok {
			o.envMutex.RUnlock()
			return
		}
		envCopy := *envForReconcile
		o.envMutex.RUnlock()
		o.reconcilePendingOrFailed(rctx, &envCopy)
	}()

	return nil
}

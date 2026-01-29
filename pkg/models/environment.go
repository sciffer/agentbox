package models

import "time"

// EnvironmentStatus represents the current state of an environment
type EnvironmentStatus string

const (
	StatusPending     EnvironmentStatus = "pending"
	StatusRunning     EnvironmentStatus = "running"
	StatusTerminating EnvironmentStatus = "terminating"
	StatusTerminated  EnvironmentStatus = "terminated"
	StatusFailed      EnvironmentStatus = "failed"
)

// Toleration represents a Kubernetes toleration for pod scheduling
type Toleration struct {
	Key               string `json:"key,omitempty"`
	Operator          string `json:"operator,omitempty"` // "Exists" or "Equal"
	Value             string `json:"value,omitempty"`
	Effect            string `json:"effect,omitempty"` // "NoSchedule", "PreferNoSchedule", "NoExecute"
	TolerationSeconds *int64 `json:"tolerationSeconds,omitempty"`
}

// NetworkPolicyConfig defines network isolation settings
type NetworkPolicyConfig struct {
	// AllowInternet enables full internet access (default: false)
	AllowInternet bool `json:"allow_internet,omitempty"`
	// AllowedEgressCIDRs specifies allowed outbound IP ranges (e.g., ["10.0.0.0/8", "192.168.1.0/24"])
	AllowedEgressCIDRs []string `json:"allowed_egress_cidrs,omitempty"`
	// AllowedIngressPorts specifies ports to allow inbound traffic (e.g., [8080, 443])
	AllowedIngressPorts []int32 `json:"allowed_ingress_ports,omitempty"`
	// AllowClusterInternal allows traffic to other pods in the cluster (default: false)
	AllowClusterInternal bool `json:"allow_cluster_internal,omitempty"`
}

// SecurityContextConfig defines pod security settings
type SecurityContextConfig struct {
	// RunAsUser specifies the UID to run the container as
	RunAsUser *int64 `json:"run_as_user,omitempty"`
	// RunAsGroup specifies the GID to run the container as
	RunAsGroup *int64 `json:"run_as_group,omitempty"`
	// RunAsNonRoot ensures the container runs as non-root user
	RunAsNonRoot *bool `json:"run_as_non_root,omitempty"`
	// ReadOnlyRootFilesystem mounts the root filesystem as read-only
	ReadOnlyRootFilesystem *bool `json:"read_only_root_filesystem,omitempty"`
	// AllowPrivilegeEscalation controls whether a process can gain more privileges
	AllowPrivilegeEscalation *bool `json:"allow_privilege_escalation,omitempty"`
}

// IsolationConfig defines the isolation level and security settings
type IsolationConfig struct {
	// RuntimeClass specifies the container runtime (e.g., "gvisor", "kata", "runc")
	// Empty string uses the cluster default
	RuntimeClass string `json:"runtime_class,omitempty"`
	// NetworkPolicy defines network isolation settings
	NetworkPolicy *NetworkPolicyConfig `json:"network_policy,omitempty"`
	// SecurityContext defines pod security settings
	SecurityContext *SecurityContextConfig `json:"security_context,omitempty"`
}

// PoolConfig defines standby pod pool settings for an environment
type PoolConfig struct {
	// Enabled indicates whether standby pods should be maintained for this environment
	Enabled bool `json:"enabled,omitempty"`
	// Size is the number of standby pods to maintain (default: 2)
	Size int `json:"size,omitempty"`
	// MinReady is the minimum number of pods that should be ready before accepting executions
	MinReady int `json:"min_ready,omitempty"`
}

// Environment represents an isolated execution environment
type Environment struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Status       EnvironmentStatus `json:"status"`
	Image        string            `json:"image"`
	CreatedAt    time.Time         `json:"created_at"`
	StartedAt    *time.Time        `json:"started_at,omitempty"`
	Resources    ResourceSpec      `json:"resources"`
	Endpoint     string            `json:"endpoint"`
	Namespace    string            `json:"namespace"`
	Metrics      *ResourceMetrics  `json:"metrics,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	Command      []string          `json:"command,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	Timeout      int               `json:"timeout,omitempty"`
	UserID       string            `json:"user_id,omitempty"`
	NodeSelector map[string]string `json:"node_selector,omitempty"`
	Tolerations  []Toleration      `json:"tolerations,omitempty"`
	Isolation    *IsolationConfig  `json:"isolation,omitempty"`
	Pool         *PoolConfig       `json:"pool,omitempty"`

	// Reconciliation retry tracking (for pending/failed environments)
	ReconciliationRetryCount   int        `json:"reconciliation_retry_count,omitempty"`
	LastReconciliationError    string     `json:"last_reconciliation_error,omitempty"`
	LastReconciliationAt      *time.Time `json:"last_reconciliation_at,omitempty"`
	ReconciliationRetriesLeft int       `json:"reconciliation_retries_left,omitempty"` // Computed: max_retries - retry_count (for UI)
}

// EnvironmentEvent is a reconciliation or lifecycle event shown in environment logs
type EnvironmentEvent struct {
	ID            string    `json:"id"`
	EnvironmentID string    `json:"environment_id"`
	EventType     string    `json:"event_type"` // e.g. "reconciliation_start", "reconciliation_success", "reconciliation_failure", "provisioning"
	Message       string    `json:"message"`
	Details       string    `json:"details,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// ResourceSpec defines resource limits and requests
type ResourceSpec struct {
	CPU     string `json:"cpu"`
	Memory  string `json:"memory"`
	Storage string `json:"storage"`
}

// ResourceMetrics contains current resource usage
type ResourceMetrics struct {
	CPUUsage    string `json:"cpu_usage"`
	MemoryUsage string `json:"memory_usage"`
}

// CreateEnvironmentRequest is the request body for creating an environment
type CreateEnvironmentRequest struct {
	Name         string            `json:"name" validate:"required"`
	Image        string            `json:"image" validate:"required"`
	Resources    ResourceSpec      `json:"resources" validate:"required"`
	Timeout      int               `json:"timeout,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	Command      []string          `json:"command,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	NodeSelector map[string]string `json:"node_selector,omitempty"`
	Tolerations  []Toleration      `json:"tolerations,omitempty"`
	Isolation    *IsolationConfig  `json:"isolation,omitempty"`
	Pool         *PoolConfig       `json:"pool,omitempty"`
}

// UpdateEnvironmentRequest is the request body for PATCH /environments/{id} (optional fields only)
type UpdateEnvironmentRequest struct {
	Name         *string            `json:"name,omitempty"`
	Image        *string            `json:"image,omitempty"`
	Resources    *ResourceSpec      `json:"resources,omitempty"`
	Timeout      *int               `json:"timeout,omitempty"`
	Env          *map[string]string `json:"env,omitempty"`
	Command      *[]string          `json:"command,omitempty"`
	Labels       *map[string]string `json:"labels,omitempty"`
	NodeSelector *map[string]string `json:"node_selector,omitempty"`
	Tolerations  *[]Toleration      `json:"tolerations,omitempty"`
	Isolation    *IsolationConfig   `json:"isolation,omitempty"`
	Pool         *PoolConfig        `json:"pool,omitempty"`
}

// ExecRequest is the request body for executing a command in an existing environment
type ExecRequest struct {
	Command []string `json:"command" validate:"required,min=1"`
	Timeout int      `json:"timeout,omitempty"`
}

// EphemeralExecRequest is the request body for executing a command in a new isolated pod
// The pod inherits configuration from the referenced environment (image, resources, isolation, etc.)
// A new pod is created, the command runs, and the pod is deleted automatically
type EphemeralExecRequest struct {
	EnvironmentID string            `json:"environment_id" validate:"required"`
	Command       []string          `json:"command" validate:"required,min=1"`
	Timeout       int               `json:"timeout,omitempty"`
	Env           map[string]string `json:"env,omitempty"` // Additional env vars (merged with environment's)
}

// ExecResponse is the response from executing a command synchronously
type ExecResponse struct {
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	DurationMs int64  `json:"duration_ms"`
}

// ExecutionStatus represents the current state of an async execution
type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"
	ExecutionStatusQueued    ExecutionStatus = "queued"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusCanceled  ExecutionStatus = "canceled"
)

// Execution represents an async command execution
type Execution struct {
	ID            string            `json:"id"`
	EnvironmentID string            `json:"environment_id"`
	Command       []string          `json:"command"`
	Env           map[string]string `json:"env,omitempty"`
	Status        ExecutionStatus   `json:"status"`
	UserID        string            `json:"user_id,omitempty"`
	PodName       string            `json:"pod_name,omitempty"`
	Namespace     string            `json:"namespace,omitempty"`

	// Timestamps
	CreatedAt   time.Time  `json:"created_at"`
	QueuedAt    *time.Time `json:"queued_at,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Result (populated when completed)
	ExitCode   *int   `json:"exit_code,omitempty"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMs *int64 `json:"duration_ms,omitempty"`
}

// ExecutionResponse is the API response for execution status
type ExecutionResponse struct {
	ID            string          `json:"id"`
	EnvironmentID string          `json:"environment_id"`
	Status        ExecutionStatus `json:"status"`
	CreatedAt     time.Time       `json:"created_at"`
	StartedAt     *time.Time      `json:"started_at,omitempty"`
	CompletedAt   *time.Time      `json:"completed_at,omitempty"`
	ExitCode      *int            `json:"exit_code,omitempty"`
	Stdout        string          `json:"stdout,omitempty"`
	Stderr        string          `json:"stderr,omitempty"`
	Error         string          `json:"error,omitempty"`
	DurationMs    *int64          `json:"duration_ms,omitempty"`
}

// ExecutionListResponse is the response for listing executions
type ExecutionListResponse struct {
	Executions []ExecutionResponse `json:"executions"`
	Total      int                 `json:"total"`
}

// ListEnvironmentsResponse is the response for listing environments
type ListEnvironmentsResponse struct {
	Environments []Environment `json:"environments"`
	Total        int           `json:"total"`
	Limit        int           `json:"limit"`
	Offset       int           `json:"offset"`
}

// HealthResponse is the response for health checks
type HealthResponse struct {
	Status     string                 `json:"status"`
	Version    string                 `json:"version"`
	Kubernetes KubernetesHealthStatus `json:"kubernetes"`
	Capacity   ClusterCapacity        `json:"capacity"`
}

// KubernetesHealthStatus represents the k8s cluster health
type KubernetesHealthStatus struct {
	Connected bool   `json:"connected"`
	Version   string `json:"version"`
}

// ClusterCapacity represents available cluster resources
type ClusterCapacity struct {
	TotalNodes      int    `json:"total_nodes"`
	AvailableCPU    string `json:"available_cpu"`
	AvailableMemory string `json:"available_memory"`
}

// ErrorResponse is a standard error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// WebSocketMessage represents messages sent over WebSocket connections
type WebSocketMessage struct {
	Type      string    `json:"type"` // stdin, stdout, stderr, exit
	Data      string    `json:"data"`
	Timestamp time.Time `json:"timestamp"`
	ExitCode  *int      `json:"exit_code,omitempty"`
}

// LogEntry represents a log line from an environment
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Stream    string    `json:"stream"` // stdout or stderr
	Message   string    `json:"message"`
}

// LogsResponse is the response for getting logs
type LogsResponse struct {
	Logs []LogEntry `json:"logs"`
}

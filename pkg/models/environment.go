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
}

// ExecRequest is the request body for executing a command
type ExecRequest struct {
	Command []string `json:"command" validate:"required,min=1"`
	Timeout int      `json:"timeout,omitempty"`
}

// ExecResponse is the response from executing a command
type ExecResponse struct {
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	DurationMs int64  `json:"duration_ms"`
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

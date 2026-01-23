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

// Environment represents an isolated execution environment
type Environment struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Status    EnvironmentStatus `json:"status"`
	Image     string            `json:"image"`
	CreatedAt time.Time         `json:"created_at"`
	StartedAt *time.Time        `json:"started_at,omitempty"`
	Resources ResourceSpec      `json:"resources"`
	Endpoint  string            `json:"endpoint"`
	Namespace string            `json:"namespace"`
	Metrics   *ResourceMetrics  `json:"metrics,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Command   []string          `json:"command,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	Timeout   int               `json:"timeout,omitempty"`
	UserID    string            `json:"user_id,omitempty"`
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
	Name      string            `json:"name" validate:"required"`
	Image     string            `json:"image" validate:"required"`
	Resources ResourceSpec      `json:"resources" validate:"required"`
	Timeout   int               `json:"timeout,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Command   []string          `json:"command,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
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

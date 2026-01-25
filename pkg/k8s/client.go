package k8s

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps the Kubernetes client with custom methods
type Client struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
}

// NewClient creates a new Kubernetes client
func NewClient(kubeconfig string) (*Client, error) {
	var config *rest.Config
	var err error

	if kubeconfig == "" {
		// Use in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
		}
	} else {
		// Use kubeconfig file
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &Client{
		clientset: clientset,
		config:    config,
	}, nil
}

// Clientset returns the underlying Kubernetes clientset
func (c *Client) Clientset() *kubernetes.Clientset {
	return c.clientset
}

// Config returns the Kubernetes REST config
func (c *Client) Config() *rest.Config {
	return c.config
}

// HealthCheck performs a health check against the Kubernetes API
func (c *Client) HealthCheck(ctx context.Context) error {
	_, err := c.clientset.Discovery().ServerVersion()
	return err
}

// GetServerVersion returns the Kubernetes server version
func (c *Client) GetServerVersion(ctx context.Context) (string, error) {
	version, err := c.clientset.Discovery().ServerVersion()
	if err != nil {
		return "", err
	}
	return version.GitVersion, nil
}

// GetClusterCapacity returns cluster capacity information
func (c *Client) GetClusterCapacity(ctx context.Context) (int, string, string, error) {
	nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, "", "", fmt.Errorf("failed to list nodes: %w", err)
	}

	totalNodes := len(nodes.Items)
	var totalCPU int64
	var totalMemory int64

	for _, node := range nodes.Items {
		// Get allocatable resources (what's available for pods)
		cpu := node.Status.Allocatable["cpu"]
		memory := node.Status.Allocatable["memory"]

		totalCPU += cpu.MilliValue()
		totalMemory += memory.Value()
	}

	// Format CPU as millicores (e.g., "50000m")
	cpuStr := fmt.Sprintf("%dm", totalCPU)

	// Format memory (convert bytes to Gi)
	memoryGi := totalMemory / (1024 * 1024 * 1024)
	memoryStr := fmt.Sprintf("%dGi", memoryGi)

	return totalNodes, cpuStr, memoryStr, nil
}

// PodMetrics represents resource usage for a pod
type PodMetrics struct {
	CPUMillicores int64 // CPU usage in millicores
	MemoryBytes   int64 // Memory usage in bytes
}

// GetPodMetrics retrieves CPU and memory metrics for a pod using metrics-server API
func (c *Client) GetPodMetrics(ctx context.Context, namespace, podName string) (*PodMetrics, error) {
	// Use the metrics.k8s.io API
	path := fmt.Sprintf("/apis/metrics.k8s.io/v1beta1/namespaces/%s/pods/%s", namespace, podName)

	result := c.clientset.RESTClient().Get().
		AbsPath(path).
		Do(ctx)

	if err := result.Error(); err != nil {
		return nil, fmt.Errorf("failed to get pod metrics: %w", err)
	}

	raw, err := result.Raw()
	if err != nil {
		return nil, fmt.Errorf("failed to read metrics response: %w", err)
	}

	var metricsResult struct {
		Containers []struct {
			Name  string `json:"name"`
			Usage struct {
				CPU    string `json:"cpu"`
				Memory string `json:"memory"`
			} `json:"usage"`
		} `json:"containers"`
	}

	if err := json.Unmarshal(raw, &metricsResult); err != nil {
		return nil, fmt.Errorf("failed to parse metrics: %w", err)
	}

	metrics := &PodMetrics{}
	for _, container := range metricsResult.Containers {
		// Parse CPU (format: "123456n" for nanocores)
		cpuNano := parseCPUNano(container.Usage.CPU)
		metrics.CPUMillicores += cpuNano / 1_000_000 // Convert nanocores to millicores

		// Parse memory (format: "123456Ki" or "123456789")
		memBytes := parseMemoryBytes(container.Usage.Memory)
		metrics.MemoryBytes += memBytes
	}

	return metrics, nil
}

// parseCPUNano parses CPU string (e.g., "123456789n") to nanocores
func parseCPUNano(cpu string) int64 {
	if cpu == "" {
		return 0
	}

	// Remove 'n' suffix if present (nanocores)
	if cpu[len(cpu)-1] == 'n' {
		cpu = cpu[:len(cpu)-1]
	}

	var value int64
	_, _ = fmt.Sscanf(cpu, "%d", &value) //nolint:errcheck
	return value
}

// parseMemoryBytes parses memory string to bytes
func parseMemoryBytes(memory string) int64 {
	if memory == "" {
		return 0
	}

	var value int64
	var unit string

	// Try to parse with unit suffix
	n, _ := fmt.Sscanf(memory, "%d%s", &value, &unit) //nolint:errcheck
	if n == 1 {
		return value
	}

	// Apply unit multiplier
	switch unit {
	case "Ki":
		return value * 1024
	case "Mi":
		return value * 1024 * 1024
	case "Gi":
		return value * 1024 * 1024 * 1024
	case "K":
		return value * 1000
	case "M":
		return value * 1000 * 1000
	case "G":
		return value * 1000 * 1000 * 1000
	default:
		return value
	}
}

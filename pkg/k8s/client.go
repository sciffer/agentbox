package k8s

import (
	"context"
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

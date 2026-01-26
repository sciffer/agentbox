package mocks

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/sciffer/agentbox/pkg/k8s"
)

// Ensure MockK8sClient implements k8s.ClientInterface
var _ k8s.ClientInterface = (*MockK8sClient)(nil)

// MockK8sClient is a mock implementation of the Kubernetes client for testing
// It implements all methods of k8s.Client for testing purposes
type MockK8sClient struct {
	namespaces       map[string]bool
	pods             map[string]map[string]*corev1.Pod
	quotas           map[string]bool
	policies         map[string]bool
	podLogs          map[string]map[string]string // namespace -> pod -> logs
	healthCheckError bool
	mu               sync.RWMutex
}

// NewMockK8sClient creates a new mock Kubernetes client
func NewMockK8sClient() *MockK8sClient {
	return &MockK8sClient{
		namespaces:       make(map[string]bool),
		pods:             make(map[string]map[string]*corev1.Pod),
		quotas:           make(map[string]bool),
		policies:         make(map[string]bool),
		podLogs:          make(map[string]map[string]string),
		healthCheckError: false,
	}
}

// Clientset returns nil for mock (not needed for unit tests)
func (m *MockK8sClient) Clientset() *kubernetes.Clientset {
	return nil
}

// Config returns nil for mock
func (m *MockK8sClient) Config() *rest.Config {
	return nil
}

// HealthCheck simulates a health check
func (m *MockK8sClient) HealthCheck(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.healthCheckError {
		return fmt.Errorf("health check failed")
	}
	return nil
}

// GetServerVersion returns a mock version
func (m *MockK8sClient) GetServerVersion(ctx context.Context) (string, error) {
	return "v1.28.0", nil
}

// GetClusterCapacity returns mock cluster capacity
func (m *MockK8sClient) GetClusterCapacity(ctx context.Context) (int, string, string, error) {
	// Return mock values: 3 nodes, 50000m CPU, 100Gi memory
	return 3, "50000m", "100Gi", nil
}

// CreateNamespace creates a mock namespace
func (m *MockK8sClient) CreateNamespace(ctx context.Context, name string, labels map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.namespaces[name] {
		return fmt.Errorf("namespace already exists")
	}

	m.namespaces[name] = true
	m.pods[name] = make(map[string]*corev1.Pod)
	return nil
}

// DeleteNamespace deletes a mock namespace
func (m *MockK8sClient) DeleteNamespace(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.namespaces, name)
	delete(m.pods, name)
	return nil
}

// NamespaceExists checks if a namespace exists
func (m *MockK8sClient) NamespaceExists(ctx context.Context, name string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.namespaces[name], nil
}

// CreateResourceQuota creates a mock resource quota
func (m *MockK8sClient) CreateResourceQuota(ctx context.Context, namespace, cpu, memory, storage string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.namespaces[namespace] {
		return fmt.Errorf("namespace not found")
	}

	m.quotas[namespace] = true
	return nil
}

// CreateNetworkPolicy creates a mock network policy
func (m *MockK8sClient) CreateNetworkPolicy(ctx context.Context, namespace string) error {
	return m.CreateNetworkPolicyWithConfig(ctx, namespace, nil)
}

// CreateNetworkPolicyWithConfig creates a mock network policy with config
func (m *MockK8sClient) CreateNetworkPolicyWithConfig(ctx context.Context, namespace string, config *k8s.NetworkPolicyConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.namespaces[namespace] {
		return fmt.Errorf("namespace not found")
	}

	m.policies[namespace] = true
	return nil
}

// CreatePod creates a mock pod
func (m *MockK8sClient) CreatePod(ctx context.Context, spec *k8s.PodSpec) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.namespaces[spec.Namespace] {
		return fmt.Errorf("namespace not found")
	}

	pod := &corev1.Pod{
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
	}

	if m.pods[spec.Namespace] == nil {
		m.pods[spec.Namespace] = make(map[string]*corev1.Pod)
	}

	m.pods[spec.Namespace][spec.Name] = pod
	return nil
}

// GetPod retrieves a mock pod
func (m *MockK8sClient) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if pods, ok := m.pods[namespace]; ok {
		if pod, ok := pods[name]; ok {
			return pod, nil
		}
	}

	return nil, fmt.Errorf("pod not found")
}

// DeletePod deletes a mock pod
func (m *MockK8sClient) DeletePod(ctx context.Context, namespace, name string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pods, ok := m.pods[namespace]; ok {
		delete(pods, name)
		return nil
	}

	return fmt.Errorf("pod not found")
}

// WaitForPodRunning simulates waiting for a pod to be running
func (m *MockK8sClient) WaitForPodRunning(ctx context.Context, namespace, name string) error {
	// In mock, immediately mark as running
	m.mu.Lock()
	defer m.mu.Unlock()

	if pods, ok := m.pods[namespace]; ok {
		if pod, ok := pods[name]; ok {
			pod.Status.Phase = corev1.PodRunning
			return nil
		}
	}

	return fmt.Errorf("pod not found")
}

// WaitForPodCompletion simulates waiting for a pod to complete
func (m *MockK8sClient) WaitForPodCompletion(ctx context.Context, namespace, name string) (*k8s.PodCompletionResult, error) {
	// In mock, immediately mark as succeeded and return
	m.mu.Lock()
	defer m.mu.Unlock()

	if pods, ok := m.pods[namespace]; ok {
		if pod, ok := pods[name]; ok {
			pod.Status.Phase = corev1.PodSucceeded

			// Get logs if available
			logs := "mock execution output\n"
			if podLogs, ok := m.podLogs[namespace]; ok {
				if logContent, ok := podLogs[name]; ok {
					logs = logContent
				}
			}

			return &k8s.PodCompletionResult{
				Phase:    corev1.PodSucceeded,
				ExitCode: 0,
				Logs:     logs,
			}, nil
		}
	}

	return nil, fmt.Errorf("pod not found")
}

// ExecInPod simulates command execution in a pod
func (m *MockK8sClient) ExecInPod(ctx context.Context,
	namespace, podName string,
	command []string,
	stdin io.Reader,
	stdout, stderr io.Writer) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if pods, ok := m.pods[namespace]; ok {
		if _, ok := pods[podName]; ok {
			// Simulate successful execution
			if stdout != nil {
				stdout.Write([]byte("mock output\n"))
			}
			return nil
		}
	}

	return fmt.Errorf("pod not found")
}

// GetPodLogs simulates retrieving pod logs
func (m *MockK8sClient) GetPodLogs(ctx context.Context, namespace, podName string, tailLines *int64) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if we have custom logs set
	if logs, ok := m.podLogs[namespace]; ok {
		if logContent, ok := logs[podName]; ok {
			return logContent, nil
		}
	}

	// Default: check if pod exists
	if pods, ok := m.pods[namespace]; ok {
		if _, ok := pods[podName]; ok {
			return "mock log output\n", nil
		}
	}

	return "", fmt.Errorf("pod not found")
}

// StreamPodLogs simulates streaming pod logs
func (m *MockK8sClient) StreamPodLogs(ctx context.Context, namespace, podName string, tailLines *int64, follow bool) (io.ReadCloser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if we have custom logs set
	var logContent string
	if logs, ok := m.podLogs[namespace]; ok {
		if content, ok := logs[podName]; ok {
			logContent = content
		}
	}

	// Default: check if pod exists
	if logContent == "" {
		if pods, ok := m.pods[namespace]; ok {
			if _, ok := pods[podName]; ok {
				logContent = "mock log output\n"
			}
		}
	}

	if logContent == "" {
		return nil, fmt.Errorf("pod not found")
	}

	// Create a mock stream that implements io.ReadCloser
	// For testing, we'll return the logs as a stream
	return io.NopCloser(strings.NewReader(logContent)), nil
}

// ListPods lists mock pods in a namespace
func (m *MockK8sClient) ListPods(ctx context.Context, namespace, labelSelector string) (*corev1.PodList, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	podList := &corev1.PodList{
		Items: []corev1.Pod{},
	}

	if pods, ok := m.pods[namespace]; ok {
		for _, pod := range pods {
			podList.Items = append(podList.Items, *pod)
		}
	}

	return podList, nil
}

// SetPodRunning manually sets a pod to running state (for testing)
func (m *MockK8sClient) SetPodRunning(namespace, name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pods, ok := m.pods[namespace]; ok {
		if pod, ok := pods[name]; ok {
			pod.Status.Phase = corev1.PodRunning
		}
	}
}

// SetPodFailed manually sets a pod to failed state (for testing)
func (m *MockK8sClient) SetPodFailed(namespace, name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pods, ok := m.pods[namespace]; ok {
		if pod, ok := pods[name]; ok {
			pod.Status.Phase = corev1.PodFailed
		}
	}
}

// GetPodCount returns the number of pods in a namespace
func (m *MockK8sClient) GetPodCount(namespace string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if pods, ok := m.pods[namespace]; ok {
		return len(pods)
	}
	return 0
}

// Reset clears all mock data
func (m *MockK8sClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.namespaces = make(map[string]bool)
	m.pods = make(map[string]map[string]*corev1.Pod)
	m.quotas = make(map[string]bool)
	m.policies = make(map[string]bool)
	m.podLogs = make(map[string]map[string]string)
	m.healthCheckError = false
}

// SetHealthCheckError sets whether health check should fail
func (m *MockK8sClient) SetHealthCheckError(fail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthCheckError = fail
}

// SetPodLogs sets custom logs for a pod
func (m *MockK8sClient) SetPodLogs(namespace, podName, logs string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.podLogs[namespace] == nil {
		m.podLogs[namespace] = make(map[string]string)
	}
	m.podLogs[namespace][podName] = logs
}

// PodSpec is a helper type for creating pods in tests
type PodSpec struct {
	Name      string
	Namespace string
	Image     string
}

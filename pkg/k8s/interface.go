package k8s

import (
	"context"
	"io"

	corev1 "k8s.io/api/core/v1"
)

// ClientInterface defines the interface for Kubernetes client operations
// This allows for easier testing with mocks
type ClientInterface interface {
	HealthCheck(ctx context.Context) error
	GetServerVersion(ctx context.Context) (string, error)
	GetClusterCapacity(ctx context.Context) (int, string, string, error)
	CreateNamespace(ctx context.Context, name string, labels map[string]string) error
	DeleteNamespace(ctx context.Context, name string) error
	NamespaceExists(ctx context.Context, name string) (bool, error)
	CreateResourceQuota(ctx context.Context, namespace, cpu, memory, storage string) error
	CreateNetworkPolicy(ctx context.Context, namespace string) error
	CreatePod(ctx context.Context, spec *PodSpec) error
	GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error)
	DeletePod(ctx context.Context, namespace, name string, force bool) error
	WaitForPodRunning(ctx context.Context, namespace, name string) error
	ExecInPod(ctx context.Context, namespace, podName string, command []string, stdin io.Reader, stdout, stderr io.Writer) error
	GetPodLogs(ctx context.Context, namespace, podName string, tailLines *int64) (string, error)
	StreamPodLogs(ctx context.Context, namespace, podName string, tailLines *int64, follow bool) (io.ReadCloser, error)
	ListPods(ctx context.Context, namespace string, labelSelector string) (*corev1.PodList, error)
}

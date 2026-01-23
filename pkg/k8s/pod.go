package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// PodSpec holds pod creation parameters
type PodSpec struct {
	Name         string
	Namespace    string
	Image        string
	Command      []string
	Env          map[string]string
	CPU          string
	Memory       string
	Storage      string
	RuntimeClass string
	Labels       map[string]string
}

// CreatePod creates a new pod
func (c *Client) CreatePod(ctx context.Context, spec *PodSpec) error {
	// Validate required fields
	if spec.Name == "" {
		return fmt.Errorf("pod name is required")
	}
	if spec.Namespace == "" {
		return fmt.Errorf("pod namespace is required")
	}
	if spec.Image == "" {
		return fmt.Errorf("pod image is required")
	}
	if len(spec.Command) == 0 {
		return fmt.Errorf("pod command is required")
	}

	// Pre-allocate env vars slice with known capacity
	envVars := make([]corev1.EnvVar, 0, len(spec.Env))
	for k, v := range spec.Env {
		// Validate env var key
		if k == "" {
			continue // Skip empty keys
		}
		envVars = append(envVars, corev1.EnvVar{
			Name:  k,
			Value: v,
		})
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: spec.Namespace,
			Labels:    spec.Labels,
		},
		Spec: corev1.PodSpec{
			// Only set RuntimeClass if specified (empty string means use default)
			RuntimeClassName: func() *string {
				if spec.RuntimeClass != "" {
					return &spec.RuntimeClass
				}
				return nil
			}(),
			Containers: []corev1.Container{
				{
					Name:    "main",
					Image:   spec.Image,
					Command: spec.Command,
					Env:     envVars,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:              resource.MustParse(spec.CPU),
							corev1.ResourceMemory:           resource.MustParse(spec.Memory),
							corev1.ResourceEphemeralStorage: resource.MustParse(spec.Storage),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:              resource.MustParse(spec.CPU),
							corev1.ResourceMemory:           resource.MustParse(spec.Memory),
							corev1.ResourceEphemeralStorage: resource.MustParse(spec.Storage),
						},
					},
					Stdin: true,
					TTY:   true,
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	_, err := c.clientset.CoreV1().Pods(spec.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create pod: %w", err)
	}

	return nil
}

// GetPod retrieves a pod
func (c *Client) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod: %w", err)
	}
	return pod, nil
}

// DeletePod deletes a pod
func (c *Client) DeletePod(ctx context.Context, namespace, name string, force bool) error {
	deleteOptions := metav1.DeleteOptions{}
	if force {
		gracePeriod := int64(0)
		deleteOptions.GracePeriodSeconds = &gracePeriod
	}

	err := c.clientset.CoreV1().Pods(namespace).Delete(ctx, name, deleteOptions)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete pod: %w", err)
	}

	return nil
}

// WaitForPodRunning waits for a pod to reach running state
func (c *Client) WaitForPodRunning(ctx context.Context, namespace, name string) error {
	watch, err := c.clientset.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", name),
	})
	if err != nil {
		return fmt.Errorf("failed to watch pod: %w", err)
	}
	defer watch.Stop()

	for {
		select {
		case event := <-watch.ResultChan():
			if event.Object == nil {
				return fmt.Errorf("watch channel closed")
			}

			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}

			if pod.Status.Phase == corev1.PodRunning {
				return nil
			}

			if pod.Status.Phase == corev1.PodFailed {
				return fmt.Errorf("pod failed to start")
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// ExecInPod executes a command in a running pod
func (c *Client) ExecInPod(ctx context.Context, namespace, podName string, command []string, stdin io.Reader, stdout, stderr io.Writer) error {
	req := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: command,
			Stdin:   stdin != nil,
			Stdout:  stdout != nil,
			Stderr:  stderr != nil,
			TTY:     false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(c.config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    false,
	})

	if err != nil {
		return fmt.Errorf("failed to execute command: %w", err)
	}

	return nil
}

// GetPodLogs retrieves logs from a pod
func (c *Client) GetPodLogs(ctx context.Context, namespace, podName string, tailLines *int64) (string, error) {
	opts := &corev1.PodLogOptions{}
	if tailLines != nil {
		opts.TailLines = tailLines
	}

	req := c.clientset.CoreV1().Pods(namespace).GetLogs(podName, opts)
	logs, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get pod logs: %w", err)
	}
	defer logs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, logs)
	if err != nil {
		return "", fmt.Errorf("failed to read logs: %w", err)
	}

	return buf.String(), nil
}

// StreamPodLogs streams logs from a pod, optionally following new logs
func (c *Client) StreamPodLogs(ctx context.Context, namespace, podName string, tailLines *int64, follow bool) (io.ReadCloser, error) {
	opts := &corev1.PodLogOptions{
		Follow: follow,
	}
	if tailLines != nil {
		opts.TailLines = tailLines
	}

	req := c.clientset.CoreV1().Pods(namespace).GetLogs(podName, opts)
	logs, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to stream pod logs: %w", err)
	}

	return logs, nil
}

// ListPods lists all pods in a namespace
func (c *Client) ListPods(ctx context.Context, namespace string, labelSelector string) (*corev1.PodList, error) {
	opts := metav1.ListOptions{}
	if labelSelector != "" {
		opts.LabelSelector = labelSelector
	}

	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	return pods, nil
}

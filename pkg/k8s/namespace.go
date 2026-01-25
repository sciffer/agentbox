package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateNamespace creates a new namespace for an environment
func (c *Client) CreateNamespace(ctx context.Context, name string, labels map[string]string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}

	_, err := c.clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	return nil
}

// DeleteNamespace deletes a namespace and waits for it to be fully removed
func (c *Client) DeleteNamespace(ctx context.Context, name string) error {
	// Use Foreground propagation policy to ensure all resources are deleted
	propagationPolicy := metav1.DeletePropagationForeground
	err := c.clientset.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to delete namespace: %w", err)
	}

	// Wait for namespace to be fully deleted
	return c.waitForNamespaceDeletion(ctx, name)
}

// waitForNamespaceDeletion waits for a namespace to be fully deleted
func (c *Client) waitForNamespaceDeletion(ctx context.Context, name string) error {
	// Use a watch to wait for the namespace to be deleted
	watch, err := c.clientset.CoreV1().Namespaces().Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", name),
	})
	if err != nil {
		return fmt.Errorf("failed to watch namespace deletion: %w", err)
	}
	defer watch.Stop()

	for {
		select {
		case event := <-watch.ResultChan():
			if event.Type == "DELETED" {
				return nil
			}
			// Check if namespace still exists
			_, err := c.clientset.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				return nil
			}
		case <-ctx.Done():
			// Check one more time if namespace is gone
			_, err := c.clientset.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("timeout waiting for namespace deletion: %w", ctx.Err())
		}
	}
}

// NamespaceExists checks if a namespace exists
func (c *Client) NamespaceExists(ctx context.Context, name string) (bool, error) {
	_, err := c.clientset.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// CreateResourceQuota creates resource quotas for a namespace
func (c *Client) CreateResourceQuota(ctx context.Context, namespace, cpu, memory, storage string) error {
	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "environment-quota",
			Namespace: namespace,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				corev1.ResourceLimitsCPU:       resource.MustParse(cpu),
				corev1.ResourceLimitsMemory:    resource.MustParse(memory),
				corev1.ResourceRequestsStorage: resource.MustParse(storage),
			},
		},
	}

	_, err := c.clientset.CoreV1().ResourceQuotas(namespace).Create(ctx, quota, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create resource quota: %w", err)
	}

	return nil
}

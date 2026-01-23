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

// DeleteNamespace deletes a namespace
func (c *Client) DeleteNamespace(ctx context.Context, name string) error {
	err := c.clientset.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete namespace: %w", err)
	}

	return nil
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

package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// NetworkPolicyConfig holds network policy configuration
type NetworkPolicyConfig struct {
	AllowInternet        bool
	AllowedEgressCIDRs   []string
	AllowedIngressPorts  []int32
	AllowClusterInternal bool
}

// CreateNetworkPolicy creates a network policy for isolation (uses default restrictive config)
func (c *Client) CreateNetworkPolicy(ctx context.Context, namespace string) error {
	return c.CreateNetworkPolicyWithConfig(ctx, namespace, nil)
}

// CreateNetworkPolicyWithConfig creates a network policy with custom configuration
func (c *Client) CreateNetworkPolicyWithConfig(ctx context.Context, namespace string, config *NetworkPolicyConfig) error {
	// Default deny all ingress and egress
	policyTypes := []networkingv1.PolicyType{
		networkingv1.PolicyTypeIngress,
		networkingv1.PolicyTypeEgress,
	}

	// Allow DNS egress (required for basic functionality)
	dnsPort := int32(53)
	udpProtocol := corev1.ProtocolUDP
	tcpProtocol := corev1.ProtocolTCP

	// Start with DNS egress rule (always required)
	egressRules := []networkingv1.NetworkPolicyEgressRule{
		{
			// Allow DNS to kube-dns
			Ports: []networkingv1.NetworkPolicyPort{
				{
					Protocol: &udpProtocol,
					Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: dnsPort},
				},
				{
					Protocol: &tcpProtocol,
					Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: dnsPort},
				},
			},
			To: []networkingv1.NetworkPolicyPeer{
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"name": "kube-system",
						},
					},
				},
			},
		},
	}

	// Build ingress rules
	var ingressRules []networkingv1.NetworkPolicyIngressRule

	if config != nil {
		// Allow internet access if enabled
		if config.AllowInternet {
			egressRules = append(egressRules, networkingv1.NetworkPolicyEgressRule{
				// Allow all egress (no restrictions)
				To: []networkingv1.NetworkPolicyPeer{},
			})
		}

		// Add allowed egress CIDRs
		for _, cidr := range config.AllowedEgressCIDRs {
			if cidr == "" {
				continue
			}
			egressRules = append(egressRules, networkingv1.NetworkPolicyEgressRule{
				To: []networkingv1.NetworkPolicyPeer{
					{
						IPBlock: &networkingv1.IPBlock{
							CIDR: cidr,
						},
					},
				},
			})
		}

		// Allow cluster internal traffic if enabled
		if config.AllowClusterInternal {
			// Allow egress to all pods in the cluster
			egressRules = append(egressRules, networkingv1.NetworkPolicyEgressRule{
				To: []networkingv1.NetworkPolicyPeer{
					{
						PodSelector: &metav1.LabelSelector{}, // All pods
					},
				},
			})
			// Allow ingress from all pods in the cluster
			ingressRules = append(ingressRules, networkingv1.NetworkPolicyIngressRule{
				From: []networkingv1.NetworkPolicyPeer{
					{
						PodSelector: &metav1.LabelSelector{}, // All pods
					},
				},
			})
		}

		// Add allowed ingress ports
		if len(config.AllowedIngressPorts) > 0 {
			ports := make([]networkingv1.NetworkPolicyPort, 0, len(config.AllowedIngressPorts))
			for _, port := range config.AllowedIngressPorts {
				ports = append(ports, networkingv1.NetworkPolicyPort{
					Protocol: &tcpProtocol,
					Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: port},
				})
			}
			ingressRules = append(ingressRules, networkingv1.NetworkPolicyIngressRule{
				Ports: ports,
				From:  []networkingv1.NetworkPolicyPeer{}, // From anywhere
			})
		}
	}

	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "isolation-policy",
			Namespace: namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			// Apply to all pods in namespace
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: policyTypes,
			Ingress:     ingressRules,
			Egress:      egressRules,
		},
	}

	_, err := c.clientset.NetworkingV1().NetworkPolicies(namespace).Create(ctx, policy, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create network policy: %w", err)
	}

	return nil
}

// DeleteNetworkPolicy deletes a network policy
func (c *Client) DeleteNetworkPolicy(ctx context.Context, namespace, name string) error {
	err := c.clientset.NetworkingV1().NetworkPolicies(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete network policy: %w", err)
	}
	return nil
}

// UpdateNetworkPolicy updates network policy to allow specific egress
// This method is kept for future use but not currently called
func (c *Client) UpdateNetworkPolicy(ctx context.Context, namespace string, allowedCIDRs []string) error {
	if len(allowedCIDRs) == 0 {
		return fmt.Errorf("at least one CIDR must be provided")
	}

	policy, err := c.clientset.NetworkingV1().NetworkPolicies(namespace).Get(ctx, "isolation-policy", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get network policy: %w", err)
	}

	// Pre-allocate egress rules slice
	newEgressRules := make([]networkingv1.NetworkPolicyEgressRule, 0, len(policy.Spec.Egress)+len(allowedCIDRs))
	newEgressRules = append(newEgressRules, policy.Spec.Egress...)

	// Add CIDR blocks to egress rules
	for _, cidr := range allowedCIDRs {
		if cidr == "" {
			continue // Skip empty CIDRs
		}
		newEgressRules = append(newEgressRules, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR: cidr,
					},
				},
			},
		})
	}

	policy.Spec.Egress = newEgressRules

	_, err = c.clientset.NetworkingV1().NetworkPolicies(namespace).Update(ctx, policy, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update network policy: %w", err)
	}

	return nil
}

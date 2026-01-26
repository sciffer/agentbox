package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sciffer/agentbox/pkg/models"
	"github.com/sciffer/agentbox/pkg/validator"
)

func TestValidateCreateRequest(t *testing.T) {
	v := validator.New(10000, 10*1024*1024*1024, 100*1024*1024*1024, 86400)

	tests := []struct {
		name        string
		request     models.CreateEnvironmentRequest
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid request",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
			},
			expectError: false,
		},
		{
			name: "missing name",
			request: models.CreateEnvironmentRequest{
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
			},
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name: "invalid name format",
			request: models.CreateEnvironmentRequest{
				Name:  "Test_Env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
			},
			expectError: true,
			errorMsg:    "name must be lowercase alphanumeric with hyphens",
		},
		{
			name: "name too long",
			request: models.CreateEnvironmentRequest{
				Name:  "this-is-a-very-long-name-that-exceeds-the-maximum-allowed-length-for-kubernetes-resources",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
			},
			expectError: true,
			errorMsg:    "name must be 63 characters or less",
		},
		{
			name: "missing image",
			request: models.CreateEnvironmentRequest{
				Name: "test-env",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
			},
			expectError: true,
			errorMsg:    "image is required",
		},
		{
			name: "invalid CPU format",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "invalid",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
			},
			expectError: true,
		},
		{
			name: "CPU exceeds limit",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "20000m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
			},
			expectError: true,
		},
		{
			name: "negative CPU",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "-500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
			},
			expectError: true,
		},
		{
			name: "invalid memory format",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "invalid",
					Storage: "1Gi",
				},
			},
			expectError: true,
		},
		{
			name: "timeout exceeds maximum",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Timeout: 100000,
			},
			expectError: true,
		},
		{
			name: "negative timeout",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Timeout: -100,
			},
			expectError: true,
		},
		{
			name: "empty label key",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Labels: map[string]string{
					"": "value",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateCreateRequest(&tt.request)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateResourceSpec(t *testing.T) {
	v := validator.New(10000, 10*1024*1024*1024, 100*1024*1024*1024, 86400)

	tests := []struct {
		name        string
		spec        models.ResourceSpec
		expectError bool
	}{
		{
			name: "valid resources",
			spec: models.ResourceSpec{
				CPU:     "500m",
				Memory:  "512Mi",
				Storage: "1Gi",
			},
			expectError: false,
		},
		{
			name: "CPU in cores",
			spec: models.ResourceSpec{
				CPU:     "2",
				Memory:  "512Mi",
				Storage: "1Gi",
			},
			expectError: false,
		},
		{
			name: "memory in different units",
			spec: models.ResourceSpec{
				CPU:     "500m",
				Memory:  "1Gi",
				Storage: "1Gi",
			},
			expectError: false,
		},
		{
			name: "missing CPU",
			spec: models.ResourceSpec{
				Memory:  "512Mi",
				Storage: "1Gi",
			},
			expectError: true,
		},
		{
			name: "missing memory",
			spec: models.ResourceSpec{
				CPU:     "500m",
				Storage: "1Gi",
			},
			expectError: true,
		},
		{
			name: "missing storage",
			spec: models.ResourceSpec{
				CPU:    "500m",
				Memory: "512Mi",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateResourceSpec(&tt.spec)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateExecRequest(t *testing.T) {
	v := validator.New(10000, 10*1024*1024*1024, 100*1024*1024*1024, 86400)

	tests := []struct {
		name        string
		request     models.ExecRequest
		expectError bool
	}{
		{
			name: "valid exec request",
			request: models.ExecRequest{
				Command: []string{"ls", "-la"},
				Timeout: 30,
			},
			expectError: false,
		},
		{
			name: "empty command",
			request: models.ExecRequest{
				Command: []string{},
			},
			expectError: true,
		},
		{
			name: "negative timeout",
			request: models.ExecRequest{
				Command: []string{"ls"},
				Timeout: -10,
			},
			expectError: true,
		},
		{
			name: "timeout exceeds maximum",
			request: models.ExecRequest{
				Command: []string{"ls"},
				Timeout: 100000,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateExecRequest(&tt.request)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateNodeSelector(t *testing.T) {
	v := validator.New(10000, 10*1024*1024*1024, 100*1024*1024*1024, 86400)

	tests := []struct {
		name        string
		request     models.CreateEnvironmentRequest
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid node selector",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				NodeSelector: map[string]string{
					"kubernetes.io/arch": "amd64",
					"node-type":          "compute",
				},
			},
			expectError: false,
		},
		{
			name: "empty node selector key",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				NodeSelector: map[string]string{
					"": "value",
				},
			},
			expectError: true,
			errorMsg:    "node selector key cannot be empty",
		},
		{
			name: "node selector with nil map (valid)",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				NodeSelector: nil,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateCreateRequest(&tt.request)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateTolerations(t *testing.T) {
	v := validator.New(10000, 10*1024*1024*1024, 100*1024*1024*1024, 86400)

	tests := []struct {
		name        string
		request     models.CreateEnvironmentRequest
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid toleration with Equal operator",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Tolerations: []models.Toleration{
					{
						Key:      "dedicated",
						Operator: "Equal",
						Value:    "agents",
						Effect:   "NoSchedule",
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid toleration with Exists operator",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Tolerations: []models.Toleration{
					{
						Key:      "node.kubernetes.io/not-ready",
						Operator: "Exists",
						Effect:   "NoExecute",
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid toleration with tolerationSeconds",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Tolerations: []models.Toleration{
					{
						Key:               "node.kubernetes.io/unreachable",
						Operator:          "Exists",
						Effect:            "NoExecute",
						TolerationSeconds: ptr(int64(300)),
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid operator",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Tolerations: []models.Toleration{
					{
						Key:      "dedicated",
						Operator: "Invalid",
						Value:    "agents",
						Effect:   "NoSchedule",
					},
				},
			},
			expectError: true,
			errorMsg:    "operator must be 'Exists' or 'Equal'",
		},
		{
			name: "invalid effect",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Tolerations: []models.Toleration{
					{
						Key:      "dedicated",
						Operator: "Equal",
						Value:    "agents",
						Effect:   "InvalidEffect",
					},
				},
			},
			expectError: true,
			errorMsg:    "effect must be",
		},
		{
			name: "Exists operator with value (invalid)",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Tolerations: []models.Toleration{
					{
						Key:      "dedicated",
						Operator: "Exists",
						Value:    "should-be-empty",
						Effect:   "NoSchedule",
					},
				},
			},
			expectError: true,
			errorMsg:    "value must be empty when operator is 'Exists'",
		},
		{
			name: "tolerationSeconds without NoExecute effect (invalid)",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Tolerations: []models.Toleration{
					{
						Key:               "dedicated",
						Operator:          "Equal",
						Value:             "agents",
						Effect:            "NoSchedule",
						TolerationSeconds: ptr(int64(300)),
					},
				},
			},
			expectError: true,
			errorMsg:    "tolerationSeconds can only be set when effect is 'NoExecute'",
		},
		{
			name: "multiple valid tolerations",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Tolerations: []models.Toleration{
					{
						Key:      "dedicated",
						Operator: "Equal",
						Value:    "agents",
						Effect:   "NoSchedule",
					},
					{
						Key:      "gpu",
						Operator: "Exists",
						Effect:   "NoSchedule",
					},
				},
			},
			expectError: false,
		},
		{
			name: "nil tolerations (valid)",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Tolerations: nil,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateCreateRequest(&tt.request)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ptr is a helper function to create a pointer to an int64
func ptr(i int64) *int64 {
	return &i
}

// boolPtr is a helper function to create a pointer to a bool
func boolPtr(b bool) *bool {
	return &b
}

func TestValidateIsolationConfig(t *testing.T) {
	v := validator.New(10000, 10*1024*1024*1024, 100*1024*1024*1024, 86400)

	tests := []struct {
		name        string
		request     models.CreateEnvironmentRequest
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid isolation with runtime class only",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Isolation: &models.IsolationConfig{
					RuntimeClass: "gvisor",
				},
			},
			expectError: false,
		},
		{
			name: "valid isolation with kata runtime",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Isolation: &models.IsolationConfig{
					RuntimeClass: "kata-qemu",
				},
			},
			expectError: false,
		},
		{
			name: "valid isolation with full network policy",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Isolation: &models.IsolationConfig{
					NetworkPolicy: &models.NetworkPolicyConfig{
						AllowInternet:        false,
						AllowedEgressCIDRs:   []string{"10.0.0.0/8", "192.168.0.0/16"},
						AllowedIngressPorts:  []int32{8080, 443},
						AllowClusterInternal: true,
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid isolation with security context",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Isolation: &models.IsolationConfig{
					SecurityContext: &models.SecurityContextConfig{
						RunAsUser:                ptr(1000),
						RunAsGroup:               ptr(1000),
						RunAsNonRoot:             boolPtr(true),
						ReadOnlyRootFilesystem:   boolPtr(true),
						AllowPrivilegeEscalation: boolPtr(false),
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid full isolation config",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Isolation: &models.IsolationConfig{
					RuntimeClass: "gvisor",
					NetworkPolicy: &models.NetworkPolicyConfig{
						AllowInternet:        false,
						AllowedEgressCIDRs:   []string{"10.0.0.0/8"},
						AllowedIngressPorts:  []int32{8080},
						AllowClusterInternal: false,
					},
					SecurityContext: &models.SecurityContextConfig{
						RunAsNonRoot:             boolPtr(true),
						ReadOnlyRootFilesystem:   boolPtr(true),
						AllowPrivilegeEscalation: boolPtr(false),
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid runtime class - uppercase",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Isolation: &models.IsolationConfig{
					RuntimeClass: "GVisor",
				},
			},
			expectError: true,
			errorMsg:    "runtime_class must be lowercase",
		},
		{
			name: "invalid runtime class - too long",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Isolation: &models.IsolationConfig{
					RuntimeClass: "this-runtime-class-name-is-way-too-long-and-exceeds-the-sixty-three-character-limit",
				},
			},
			expectError: true,
			errorMsg:    "runtime_class must be 63 characters or less",
		},
		{
			name: "invalid CIDR format",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Isolation: &models.IsolationConfig{
					NetworkPolicy: &models.NetworkPolicyConfig{
						AllowedEgressCIDRs: []string{"invalid-cidr"},
					},
				},
			},
			expectError: true,
			errorMsg:    "invalid CIDR format",
		},
		{
			name: "invalid CIDR format - missing mask",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Isolation: &models.IsolationConfig{
					NetworkPolicy: &models.NetworkPolicyConfig{
						AllowedEgressCIDRs: []string{"10.0.0.0"},
					},
				},
			},
			expectError: true,
			errorMsg:    "invalid CIDR format",
		},
		{
			name: "invalid port - too high",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Isolation: &models.IsolationConfig{
					NetworkPolicy: &models.NetworkPolicyConfig{
						AllowedIngressPorts: []int32{70000},
					},
				},
			},
			expectError: true,
			errorMsg:    "port must be between 1 and 65535",
		},
		{
			name: "invalid port - zero",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Isolation: &models.IsolationConfig{
					NetworkPolicy: &models.NetworkPolicyConfig{
						AllowedIngressPorts: []int32{0},
					},
				},
			},
			expectError: true,
			errorMsg:    "port must be between 1 and 65535",
		},
		{
			name: "invalid run_as_user - negative",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Isolation: &models.IsolationConfig{
					SecurityContext: &models.SecurityContextConfig{
						RunAsUser: ptr(-1),
					},
				},
			},
			expectError: true,
			errorMsg:    "run_as_user must be non-negative",
		},
		{
			name: "invalid run_as_group - negative",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Isolation: &models.IsolationConfig{
					SecurityContext: &models.SecurityContextConfig{
						RunAsGroup: ptr(-100),
					},
				},
			},
			expectError: true,
			errorMsg:    "run_as_group must be non-negative",
		},
		{
			name: "nil isolation config (valid)",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Isolation: nil,
			},
			expectError: false,
		},
		{
			name: "empty isolation config (valid)",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Isolation: &models.IsolationConfig{},
			},
			expectError: false,
		},
		{
			name: "allow internet with specific CIDRs (valid)",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Isolation: &models.IsolationConfig{
					NetworkPolicy: &models.NetworkPolicyConfig{
						AllowInternet:      true,
						AllowedEgressCIDRs: []string{"10.0.0.0/8"}, // Can also have specific CIDRs
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid security context with user 0 (root)",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Isolation: &models.IsolationConfig{
					SecurityContext: &models.SecurityContextConfig{
						RunAsUser:  ptr(0),
						RunAsGroup: ptr(0),
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateCreateRequest(&tt.request)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePoolConfig(t *testing.T) {
	v := validator.New(10000, 10*1024*1024*1024, 100*1024*1024*1024, 86400)

	tests := []struct {
		name        string
		request     models.CreateEnvironmentRequest
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid pool config - enabled with size",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Pool: &models.PoolConfig{
					Enabled: true,
					Size:    3,
				},
			},
			expectError: false,
		},
		{
			name: "valid pool config - disabled",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Pool: &models.PoolConfig{
					Enabled: false,
					Size:    0,
				},
			},
			expectError: false,
		},
		{
			name: "valid pool config - with min_ready",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Pool: &models.PoolConfig{
					Enabled:  true,
					Size:     5,
					MinReady: 2,
				},
			},
			expectError: false,
		},
		{
			name: "nil pool config (valid)",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Pool: nil,
			},
			expectError: false,
		},
		{
			name: "invalid pool size - negative",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Pool: &models.PoolConfig{
					Enabled: true,
					Size:    -1,
				},
			},
			expectError: true,
			errorMsg:    "pool.size must be non-negative",
		},
		{
			name: "invalid pool size - exceeds maximum",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Pool: &models.PoolConfig{
					Enabled: true,
					Size:    25,
				},
			},
			expectError: true,
			errorMsg:    "pool.size must be 20 or less",
		},
		{
			name: "invalid min_ready - negative",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Pool: &models.PoolConfig{
					Enabled:  true,
					Size:     5,
					MinReady: -1,
				},
			},
			expectError: true,
			errorMsg:    "pool.min_ready must be non-negative",
		},
		{
			name: "invalid min_ready - exceeds size",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Pool: &models.PoolConfig{
					Enabled:  true,
					Size:     3,
					MinReady: 5,
				},
			},
			expectError: true,
			errorMsg:    "pool.min_ready cannot exceed pool.size",
		},
		{
			name: "valid pool at max size",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Pool: &models.PoolConfig{
					Enabled: true,
					Size:    20,
				},
			},
			expectError: false,
		},
		{
			name: "valid pool with min_ready equal to size",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
				Pool: &models.PoolConfig{
					Enabled:  true,
					Size:     3,
					MinReady: 3,
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateCreateRequest(&tt.request)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

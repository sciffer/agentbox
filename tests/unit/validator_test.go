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

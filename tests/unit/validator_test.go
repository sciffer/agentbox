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

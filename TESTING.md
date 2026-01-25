# AgentBox Testing Guide

## Test Structure

```
tests/
├── unit/                    # Unit tests (fast, no external dependencies)
│   ├── api_test.go          # Environment API tests
│   ├── api_auth_test.go     # Authentication API tests
│   ├── user_api_test.go     # User management API tests
│   ├── apikey_api_test.go   # API key management tests
│   ├── auth_test.go         # Auth service tests
│   ├── users_test.go        # User service tests
│   ├── config_test.go       # Configuration tests
│   ├── orchestrator_test.go # Orchestrator tests
│   └── validator_test.go    # Validation tests
├── integration/             # Integration tests (require k8s cluster)
│   └── lifecycle_test.go
├── mocks/                   # Mock implementations
│   └── k8s_mock.go
└── fixtures/                # Test data
```

## Running Tests

### Quick Start

```bash
# Run all unit tests
make test-unit

# Run all tests (unit + integration, requires k8s)
make test

# Run with coverage
make test-coverage
```

### Unit Tests

Unit tests are fast and don't require external dependencies. They use mocks for Kubernetes operations.

```bash
# Run all unit tests
go test ./tests/unit/... -v

# Run specific test
go test ./tests/unit/... -v -run TestValidateCreateRequest

# Run with race detection
go test ./tests/unit/... -v -race

# Run with coverage
go test ./tests/unit/... -v -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Integration Tests

Integration tests require a running Kubernetes cluster. They test the full stack end-to-end.

**Prerequisites:**
- Running Kubernetes cluster (minikube, kind, or real cluster)
- kubectl configured with cluster access
- Appropriate RBAC permissions

```bash
# Start a local cluster (choose one)
minikube start
# or
kind create cluster

# Run integration tests
make test-integration

# Or manually
go test ./tests/integration/... -v -tags=integration

# Skip integration tests
go test ./... -short
```

### Test Coverage

```bash
# Generate coverage report
make test-coverage

# View coverage in terminal
go test ./... -cover

# Detailed coverage by package
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out

# HTML coverage report
go tool cover -html=coverage.out -o coverage.html
open coverage.html
```

## Test Categories

### 1. Validator Tests (`tests/unit/validator_test.go`)

Tests input validation logic:
- CreateRequest validation
- ResourceSpec validation
- ExecRequest validation
- Edge cases and error conditions

**Coverage:** 95%+

Example:
```bash
go test ./tests/unit/... -v -run TestValidate
```

### 2. Config Tests (`tests/unit/config_test.go`)

Tests configuration loading:
- Default values
- Environment variable overrides
- YAML file parsing
- Validation errors

**Coverage:** 90%+

Example:
```bash
go test ./tests/unit/... -v -run TestLoadConfig
```

### 3. Orchestrator Tests (`tests/unit/orchestrator_test.go`)

Tests core business logic:
- Environment creation
- Lifecycle management
- Command execution
- Resource management

**Coverage:** 85%+

Example:
```bash
go test ./tests/unit/... -v -run TestOrchestrator
```

### 4. API Tests (`tests/unit/api_test.go`)

Tests HTTP API endpoints for environments:
- Request/response handling
- Status codes
- Error handling
- Input validation

**Coverage:** 90%+

Example:
```bash
go test ./tests/unit/... -v -run TestAPI
```

### 5. Authentication Tests (`tests/unit/api_auth_test.go`, `tests/unit/auth_test.go`)

Tests authentication functionality:
- Login/logout flows
- JWT token generation and validation
- Password change
- Authentication middleware
- X-API-Key header support

**Coverage:** 90%+

Example:
```bash
go test ./tests/unit/... -v -run TestLogin
go test ./tests/unit/... -v -run TestGetMe
go test ./tests/unit/... -v -run TestChangePassword
```

### 6. User Management Tests (`tests/unit/user_api_test.go`, `tests/unit/users_test.go`)

Tests user management:
- User CRUD operations
- Role-based access control
- Admin vs regular user permissions
- Default admin creation

**Coverage:** 85%+

Example:
```bash
go test ./tests/unit/... -v -run TestListUsers
go test ./tests/unit/... -v -run TestCreateUser
go test ./tests/unit/... -v -run TestGetUser
```

### 7. API Key Tests (`tests/unit/apikey_api_test.go`)

Tests API key management:
- Creating API keys
- Listing API keys
- Revoking API keys
- API key authentication
- Key expiration

**Coverage:** 90%+

Example:
```bash
go test ./tests/unit/... -v -run TestAPIKey
```

### 8. Integration Tests (`tests/integration/lifecycle_test.go`)

Tests full system integration:
- Complete lifecycle (create → exec → delete)
- Multiple concurrent environments
- Resource constraints
- Isolation between environments

**Note:** Requires Kubernetes cluster

Example:
```bash
go test ./tests/integration/... -v -tags=integration -run TestEnvironmentLifecycle
```

## Writing Tests

### Unit Test Template

```go
package unit

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMyFeature(t *testing.T) {
	// Setup
	setup := setupTest(t)
	defer setup.cleanup()

	t.Run("happy path", func(t *testing.T) {
		// Arrange
		input := "test input"
		
		// Act
		result, err := myFunction(input)
		
		// Assert
		require.NoError(t, err)
		assert.Equal(t, expectedResult, result)
	})

	t.Run("error case", func(t *testing.T) {
		// Arrange
		input := "invalid input"
		
		// Act
		result, err := myFunction(input)
		
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected error message")
	})
}
```

### Integration Test Template

```go
// +build integration

package integration

import (
	"context"
	"testing"
	"time"
)

func TestIntegrationFeature(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Setup real dependencies
	orch, cleanup := setupIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("end-to-end flow", func(t *testing.T) {
		// Test with real k8s cluster
	})
}
```

## Test Best Practices

### 1. Use Table-Driven Tests

```go
tests := []struct {
	name        string
	input       Input
	expectError bool
}{
	{"valid input", validInput, false},
	{"invalid input", invalidInput, true},
}

for _, tt := range tests {
	t.Run(tt.name, func(t *testing.T) {
		err := function(tt.input)
		if tt.expectError {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	})
}
```

### 2. Use Mocks for External Dependencies

```go
mockK8s := mocks.NewMockK8sClient()
orchestrator := orchestrator.New(mockK8s, cfg, log)
```

### 3. Test Error Conditions

```go
t.Run("handles missing resource", func(t *testing.T) {
	_, err := orch.GetEnvironment(ctx, "non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
})
```

### 4. Use Cleanup Functions

```go
func setupTest(t *testing.T) (*TestContext, func()) {
	ctx := &TestContext{}
	
	cleanup := func() {
		// Clean up resources
	}
	
	return ctx, cleanup
}
```

### 5. Skip Slow Tests in Short Mode

```go
func TestSlowFeature(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow test in short mode")
	}
	// test code
}
```

## Continuous Integration

### GitHub Actions

```yaml
name: Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run unit tests
        run: make test-unit
      
      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./coverage.out
```

## Troubleshooting

### Tests Fail with "Kubernetes not available"

Integration tests require a Kubernetes cluster:
```bash
# Start local cluster
minikube start
# or
kind create cluster

# Verify connection
kubectl cluster-info
```

### Tests Timeout

Increase timeout for integration tests:
```bash
go test ./tests/integration/... -timeout 10m
```

### Mock Not Working

Ensure you're using the mock client:
```go
mockK8s := mocks.NewMockK8sClient()
// Don't use k8s.NewClient() in unit tests
```

### Coverage Too Low

Check which lines aren't covered:
```bash
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out | grep -v "100.0%"
```

## Test Metrics

Current test coverage by package:

| Package | Coverage | Tests |
|---------|----------|-------|
| pkg/validator | 95%+ | 30+ |
| pkg/orchestrator | 85%+ | 25+ |
| pkg/api | 90%+ | 40+ |
| pkg/auth | 90%+ | 15+ |
| pkg/users | 85%+ | 10+ |
| internal/config | 90%+ | 10+ |
| pkg/k8s | 70%+ | 15+ |
| **Total** | **85%+** | **145+** |

## Performance Testing

### Benchmarks

```bash
# Run all benchmarks
make benchmark

# Run specific benchmark
go test -bench=BenchmarkCreateEnvironment -benchmem ./...

# Compare benchmarks
go test -bench=. -benchmem ./... > old.txt
# Make changes
go test -bench=. -benchmem ./... > new.txt
benchcmp old.txt new.txt
```

### Load Testing

For load testing, use the integration tests with high concurrency:
```bash
go test ./tests/integration/... -v -tags=integration -run TestConcurrent
```

## Contributing Tests

When adding new features:

1. **Write tests first** (TDD approach)
2. **Aim for 80%+ coverage**
3. **Test error conditions**
4. **Add integration tests for user-facing features**
5. **Update this guide** if adding new test categories

Example PR checklist:
- [ ] Unit tests added
- [ ] Integration tests added (if applicable)
- [ ] All tests pass
- [ ] Coverage maintained or improved
- [ ] Tests documented

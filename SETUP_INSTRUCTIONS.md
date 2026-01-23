# AgentBox Setup Instructions

## Files Generated

I've created the following files for you to copy into your project:

### Core Configuration
1. `go.mod` - Go module definition
2. `.gitignore` - Git ignore rules
3. `config/config.yaml` - Default configuration

### Models & Data Structures
4. `pkg/models/environment.go` - Data models

### Internal Packages
5. `internal/config/config.go` - Configuration loader
6. `internal/logger/logger.go` - Structured logging

### Validation
7. `pkg/validator/validator.go` - Input validation

### Kubernetes Client
8. `pkg/k8s/client.go` - K8s client wrapper
9. `pkg/k8s/namespace.go` - Namespace operations
10. `pkg/k8s/pod.go` - Pod operations

### Business Logic
11. `pkg/orchestrator/orchestrator.go` - Environment orchestration

### API Layer
12. `pkg/api/handler.go` - HTTP handlers
13. `pkg/api/router.go` - Route definitions

### Main Application
14. `cmd/server/main.go` - Application entry point

## Setup Steps

### 1. Create Project Structure

```bash
cd ~/projects/agentbox

# Create all directories
mkdir -p cmd/server
mkdir -p pkg/{api,orchestrator,auth,k8s,models,proxy,validator,metrics}
mkdir -p internal/{config,logger}
mkdir -p tests/{unit,integration,fixtures,mocks}
mkdir -p config
```

### 2. Copy Files

Copy each artifact I've generated into the corresponding file:
- Copy the `go.mod` content → save as `go.mod`
- Copy the `.gitignore` content → save as `.gitignore`
- Copy the `config/config.yaml` content → save as `config/config.yaml`
- And so on for each file...

**Important:** In all `.go` files, replace `github.com/sciffer/agentbox` with your actual GitHub username/path.

### 3. Update Import Paths

Find and replace in all `.go` files:
```bash
find . -name "*.go" -type f -exec sed -i '' 's|github.com/sciffer/agentbox|github.com/sciffer/agentbox|g' {} \;
```

Replace `sciffer` with your actual GitHub username.

### 4. Install Dependencies

```bash
# Download Go dependencies
go mod download

# If you get errors, run:
go mod tidy
```

### 5. Fix Compilation Issues

There are a few TODO items that need implementation:
- Network policy creation in orchestrator
- Actual pod exec implementation
- WebSocket attachment handler

For now, these are stubbed out so the code compiles.

### 6. Build the Project

```bash
# Build
go build -o agentbox ./cmd/server

# Or use Make
make build
```

### 7. Test Compilation

```bash
# Verify it compiles
go build ./...

# Run tests (they'll pass with stubs)
go test ./...
```

### 8. Run Locally

```bash
# You need a Kubernetes cluster. Options:
# - minikube start
# - kind create cluster
# - Use existing cluster

# Run the server
./agentbox --config config/config.yaml

# Or
go run ./cmd/server --config config/config.yaml
```

## Quick Test

Once running, test the API:

```bash
# Health check
curl http://localhost:8080/api/v1/health

# Create environment
curl -X POST http://localhost:8080/api/v1/environments \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-env",
    "image": "python:3.11-slim",
    "resources": {
      "cpu": "500m",
      "memory": "512Mi",
      "storage": "1Gi"
    }
  }'
```

## Next Steps

After basic setup works:
1. Implement network policy creation
2. Implement WebSocket attachment
3. Complete pod exec implementation
4. Add authentication
5. Write comprehensive tests
6. Create Kubernetes deployment manifests

## Troubleshooting

### "package not found" errors
Run: `go mod tidy`

### Kubernetes connection errors
- Check kubeconfig path
- Verify cluster is running: `kubectl cluster-info`
- Check RBAC permissions

### Import path errors
- Make sure you replaced `sciffer` with your GitHub username
- Run `go mod tidy` after changing imports

## What's Missing (TODO)

The following features are stubbed and need implementation:
- [ ] Network policy enforcement
- [ ] WebSocket attachment for interactive shells
- [ ] Pod exec with proper I/O streaming
- [ ] Authentication middleware
- [ ] Metrics collection
- [ ] Complete test coverage
- [ ] Kubernetes deployment manifests

Would you like me to generate any of these next?

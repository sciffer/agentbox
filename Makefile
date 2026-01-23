.PHONY: help build test test-unit test-integration test-coverage run clean docker-build docker-run docker-push helm-lint helm-template helm-install helm-upgrade helm-uninstall helm-package lint fmt deploy-dev deploy-prod setup-dev

APP_NAME=agentbox
VERSION?=0.1.0
DOCKER_IMAGE?=agentbox:$(VERSION)

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "Building $(APP_NAME)..."
	go build -o $(APP_NAME) ./cmd/server
	@echo "Build complete: ./$(APP_NAME)"

test: ## Run all tests
	@echo "Running all tests..."
	go test -count=1 ./tests/unit/... -v
	@echo "Tests complete"

test-unit: ## Run unit tests only
	@echo "Running unit tests..."
	go test -count=1 ./tests/unit/... -v
	@echo "Unit tests complete"

test-integration: ## Run integration tests (requires k8s cluster)
	@echo "Running integration tests..."
	@echo "Note: This requires a running Kubernetes cluster"
	go test ./tests/integration/... -v -tags=integration
	@echo "Integration tests complete"

test-coverage: ## Generate and open coverage report
	@echo "Generating coverage report..."
	@echo "Note: Coverage requires testing packages directly, not test packages"
	go test ./pkg/... -coverprofile=coverage.out || go test ./tests/unit/... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
	@which open > /dev/null && open coverage.html || echo "Open coverage.html in your browser"

test-watch: ## Run tests in watch mode (requires entr)
	@echo "Watching for changes..."
	find . -name "*.go" | entr -c go test ./tests/unit/... -v

run: ## Run the application locally
	@echo "Starting $(APP_NAME)..."
	go run ./cmd/server

run-dev: ## Run with development config
	@echo "Starting $(APP_NAME) in development mode..."
	go run ./cmd/server --config config/config.yaml

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -f $(APP_NAME)
	rm -f coverage.out coverage.html
	@echo "Clean complete"

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE) -f Dockerfile .
	@echo "Docker image built: $(DOCKER_IMAGE)"

docker-run: ## Run in Docker
	@echo "Running in Docker..."
	docker run -p 8080:8080 \
		-v ~/.kube/config:/kubeconfig \
		-e AGENTBOX_KUBECONFIG=/kubeconfig \
		$(DOCKER_IMAGE)

docker-push: docker-build ## Build and push Docker image
	@echo "Pushing Docker image..."
	docker push $(DOCKER_IMAGE)
	@echo "Docker image pushed: $(DOCKER_IMAGE)"

helm-lint: ## Lint Helm chart
	@echo "Linting Helm chart..."
	@which helm > /dev/null || (echo "Helm is not installed. Install from https://helm.sh/docs/intro/install/" && exit 1)
	helm lint ./helm/agentbox
	@echo "Helm chart linted"

helm-template: ## Render Helm templates (dry-run)
	@echo "Rendering Helm templates..."
	@which helm > /dev/null || (echo "Helm is not installed. Install from https://helm.sh/docs/intro/install/" && exit 1)
	helm template agentbox ./helm/agentbox
	@echo "Helm templates rendered"

helm-install: ## Install Helm chart
	@echo "Installing Helm chart..."
	@which helm > /dev/null || (echo "Helm is not installed. Install from https://helm.sh/docs/intro/install/" && exit 1)
	helm install agentbox ./helm/agentbox
	@echo "Helm chart installed"

helm-upgrade: ## Upgrade Helm chart
	@echo "Upgrading Helm chart..."
	@which helm > /dev/null || (echo "Helm is not installed. Install from https://helm.sh/docs/intro/install/" && exit 1)
	helm upgrade agentbox ./helm/agentbox
	@echo "Helm chart upgraded"

helm-uninstall: ## Uninstall Helm chart
	@echo "Uninstalling Helm chart..."
	@which helm > /dev/null || (echo "Helm is not installed. Install from https://helm.sh/docs/intro/install/" && exit 1)
	helm uninstall agentbox
	@echo "Helm chart uninstalled"

helm-package: ## Package Helm chart
	@echo "Packaging Helm chart..."
	@which helm > /dev/null || (echo "Helm is not installed. Install from https://helm.sh/docs/intro/install/" && exit 1)
	helm package ./helm/agentbox
	@echo "Helm chart packaged"

lint: ## Run linter
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...
	@echo "Lint complete"

fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...
	@echo "Format complete"

vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...
	@echo "Vet complete"

tidy: ## Tidy go modules
	@echo "Tidying modules..."
	go mod tidy
	@echo "Tidy complete"

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	go mod download
	@echo "Dependencies downloaded"

verify: fmt vet lint test-unit ## Run all verification steps
	@echo "✅ All verification steps passed"

deploy-dev: ## Deploy to dev environment
	@echo "Deploying to dev environment..."
	kubectl apply -k deploy/kubernetes/
	@echo "Deployment complete"

deploy-prod: ## Deploy to production
	@echo "Deploying to production..."
	kubectl apply -k deploy/kubernetes/ -n production
	@echo "Deployment complete"

setup-dev: ## Setup development environment
	@echo "Setting up development environment..."
	./scripts/setup-dev.sh
	@echo "Development environment ready"

install-tools: ## Install development tools
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Tools installed"

benchmark: ## Run benchmarks
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...
	@echo "Benchmarks complete"

.DEFAULT_GOAL := help

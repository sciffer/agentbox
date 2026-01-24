.PHONY: help build test test-unit test-integration test-coverage run clean docker-build docker-run docker-push helm-lint helm-template helm-install helm-upgrade helm-uninstall helm-package lint fmt deploy-dev deploy-prod setup-dev ui-install ui-dev ui-build ui-test ui-lint ui-typecheck

APP_NAME := agentbox
DOCKER_IMAGE := agentbox:latest
DOCKER_REGISTRY := ghcr.io
DOCKER_TAG := latest

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "Building $(APP_NAME)..."
	go build -o $(APP_NAME) ./cmd/server
	@echo "Build complete: ./$(APP_NAME)"

test: test-unit ## Run all tests

test-unit: ## Run unit tests (no k8s required)
	@echo "Running unit tests..."
	go test -count=1 ./tests/unit/... -v

test-integration: ## Run integration tests (requires k8s cluster)
	@echo "Running integration tests..."
	go test -count=1 -tags=integration ./tests/integration/... -v
	@echo "Note: This requires a running Kubernetes cluster"

test-coverage: ## Generate test coverage report
	@echo "Generating coverage report..."
	go test -coverprofile=coverage.out ./pkg/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-watch: ## Run tests in watch mode (requires entr)
	@echo "Watching for changes..."
	find . -name "*.go" -not -path "./vendor/*" | entr -c go test ./tests/unit/...

run: build ## Build and run the application
	@echo "Running $(APP_NAME)..."
	./$(APP_NAME)

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -f $(APP_NAME)
	rm -f coverage.out coverage.html
	@echo "Clean complete"

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE) -f Dockerfile .
	@echo "Docker image built: $(DOCKER_IMAGE)"

docker-run: docker-build ## Build and run Docker container
	@echo "Running Docker container..."
	docker run -p 8080:8080 $(DOCKER_IMAGE)

docker-push: docker-build ## Build and push Docker image
	@echo "Pushing Docker image..."
	docker tag $(DOCKER_IMAGE) $(DOCKER_REGISTRY)/$(DOCKER_IMAGE)
	docker push $(DOCKER_REGISTRY)/$(DOCKER_IMAGE)

helm-lint: ## Lint Helm chart
	@echo "Linting Helm chart..."
	helm lint ./helm/agentbox

helm-template: ## Template Helm chart (dry-run)
	@echo "Templating Helm chart..."
	helm template agentbox ./helm/agentbox

helm-install: ## Install Helm chart
	@echo "Installing Helm chart..."
	helm install agentbox ./helm/agentbox

helm-upgrade: ## Upgrade Helm chart
	@echo "Upgrading Helm chart..."
	helm upgrade agentbox ./helm/agentbox

helm-uninstall: ## Uninstall Helm chart
	@echo "Uninstalling Helm chart..."
	helm uninstall agentbox

helm-package: ## Package Helm chart
	@echo "Packaging Helm chart..."
	helm package ./helm/agentbox
	@echo "Helm chart packaged: agentbox-*.tgz"

lint: ## Run linters
	@echo "Running linters..."
	gofmt -s -l .
	go vet ./...
	golangci-lint run

fmt: ## Format code
	@echo "Formatting code..."
	gofmt -s -w .
	@echo "Code formatted"

deploy-dev: ## Deploy to development environment
	@echo "Deploying to development..."
	# Add deployment commands here

deploy-prod: ## Deploy to production environment
	@echo "Deploying to production..."
	# Add deployment commands here

setup-dev: ## Set up development environment
	@echo "Setting up development environment..."
	@echo "Installing Go dependencies..."
	go mod download
	@echo "Development environment ready"

ui-install: ## Install UI dependencies
	@echo "Installing UI dependencies..."
	cd ui && npm install

ui-dev: ## Start UI development server (port 3000)
	@echo "Starting UI development server..."
	cd ui && npm run dev

ui-build: ## Build UI for production
	@echo "Building UI..."
	cd ui && npm run build
	@echo "UI built: ui/dist/"

ui-test: ## Run UI tests
	@echo "Running UI tests..."
	cd ui && npm run test

ui-lint: ## Run UI linting
	@echo "Running UI lint..."
	cd ui && npm run lint

ui-typecheck: ## Run UI TypeScript check
	@echo "Running UI typecheck..."
	cd ui && npm run typecheck

ui-test-coverage: ## Run UI tests with coverage
	@echo "Running UI tests with coverage..."
	cd ui && npm run test:coverage

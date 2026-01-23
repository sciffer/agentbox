# CI/CD Pipeline Documentation

## Overview

The AgentBox project uses GitHub Actions for continuous integration and deployment. The pipeline automatically runs tests, builds Docker images, packages Helm charts, and creates releases.

## Workflows

### 1. Main Branch CI/CD (`main.yml`)

**Purpose:** Primary workflow for main branch deployments

**Triggers:**
- Push to `main` branch
- Manual workflow dispatch

**Jobs:**

1. **test**
   - Runs all unit tests
   - Generates coverage reports
   - Uploads to Codecov

2. **build-and-push**
   - Builds Docker image (multi-platform: amd64, arm64)
   - Pushes to GitHub Container Registry (GHCR)
   - Tags: `latest`, `main-<sha>`
   - Uses build cache for faster builds

3. **package-helm**
   - Lints Helm chart
   - Packages chart as `.tgz`
   - Uploads as GitHub Actions artifact
   - Retention: 90 days

**Outputs:**
- Docker: `ghcr.io/<owner>/<repo>:latest`
- Helm Chart: Available in Actions artifacts

### 2. Full CI/CD Pipeline (`ci.yml`)

**Purpose:** Comprehensive pipeline for all branches and PRs

**Triggers:**
- Push to `main` or `develop`
- Pull requests to `main` or `develop`
- Version tags (`v*`)

**Jobs:**

1. **test**
   - Unit tests with coverage
   - Format checking (`go fmt`)
   - `go vet` static analysis
   - Deployment tests

2. **build-docker** (main/tags only)
   - Multi-platform Docker build
   - Push to GHCR with multiple tags
   - Semantic versioning support

3. **package-helm** (main/tags only)
   - Helm chart packaging
   - Artifact upload

4. **release** (tags only)
   - Creates GitHub release
   - Attaches Helm chart
   - Generates release notes

5. **security-scan** (main/tags only)
   - Trivy vulnerability scanning
   - Uploads results to GitHub Security

### 3. Test Suite (`test.yml`)

**Purpose:** Comprehensive testing on all branches

**Triggers:**
- Push to any branch
- Pull requests

**Jobs:**

1. **unit-tests**
   - Tests on Go 1.21 and 1.22
   - Race detector enabled
   - Coverage reporting

2. **lint**
   - golangci-lint
   - Format checking
   - `go vet`

3. **helm-lint**
   - Helm chart linting
   - Template validation
   - Resource count verification

4. **docker-build-test**
   - Docker image build test
   - Container structure validation
   - Non-root user verification

### 4. Release Workflow (`release.yml`)

**Purpose:** Automated releases for version tags

**Triggers:**
- Push of version tags (`v*.*.*`)

**Jobs:**

1. **build-and-push**
   - Versioned Docker image
   - Multiple semantic version tags
   - Latest tag for stable releases

2. **package-helm**
   - Updates Chart.yaml with version
   - Packages versioned chart
   - Creates Helm repository index

3. **create-release**
   - Creates GitHub release
   - Attaches Helm chart
   - Generates release notes
   - Includes installation instructions

## Artifacts

### Docker Images

**Location:** GitHub Container Registry (ghcr.io)

**Image Path:** `ghcr.io/<owner>/<repo>/agentbox`

**Tags:**
- `latest` - Latest main branch build
- `main-<sha>` - Specific commit on main
- `v<version>` - Version tags (e.g., `v1.0.0`)
- `v<major>.<minor>` - Minor version (e.g., `v1.0`)
- `v<major>` - Major version (e.g., `v1`)

**Usage:**
```bash
# Pull image
docker pull ghcr.io/<owner>/<repo>/agentbox:latest

# Use in Kubernetes
image: ghcr.io/<owner>/<repo>/agentbox:v1.0.0
```

### Helm Charts

**Location:** GitHub Actions Artifacts

**Format:** `.tgz` files

**Retention:** 90 days

**Download:**
1. Go to Actions tab
2. Select workflow run
3. Download artifact from Artifacts section

**For Releases:**
- Helm charts are attached to GitHub releases
- Download from Releases page
- Includes `index.yaml` for Helm repository

**Usage:**
```bash
# Install from artifact
helm install agentbox ./agentbox-1.0.0.tgz

# Or from release
helm install agentbox https://github.com/<owner>/<repo>/releases/download/v1.0.0/agentbox-1.0.0.tgz
```

## Workflow Permissions

All workflows use minimal required permissions:

- `contents: read` - Read repository code
- `contents: write` - Create releases (release workflow only)
- `packages: write` - Push to GHCR
- `security-events: write` - Upload security scan results

## Secrets

No additional secrets required. All workflows use the automatically provided `GITHUB_TOKEN`.

## Caching

Workflows use GitHub Actions cache for:
- Go module dependencies
- Docker build cache (via buildx cache)

This significantly speeds up subsequent runs.

## Security

### Security Scanning

- **Trivy** scans Docker images for vulnerabilities
- Results uploaded to GitHub Security tab
- SARIF format for integration with security tools

### Best Practices

- Images run as non-root user
- Minimal base images (Alpine)
- No unnecessary packages
- Security contexts configured in Helm chart

## Monitoring

### Workflow Status

Check workflow status:
1. Go to Actions tab in GitHub
2. View workflow runs
3. Click on specific run for details

### Notifications

GitHub will send notifications for:
- Workflow failures
- Security alerts
- Release creation

## Troubleshooting

### Common Issues

1. **Tests Fail**
   - Check test output in Actions
   - Run tests locally: `go test ./tests/unit/...`
   - Verify Go version compatibility

2. **Docker Build Fails**
   - Check Dockerfile syntax
   - Verify all dependencies
   - Check build logs for errors

3. **Helm Chart Issues**
   - Run `helm lint` locally
   - Verify Chart.yaml syntax
   - Check template rendering

4. **Permission Errors**
   - Verify workflow permissions
   - Check repository settings
   - Ensure GITHUB_TOKEN has required scopes

### Debugging

Enable debug logging:
- Add `ACTIONS_STEP_DEBUG: true` to repository secrets
- Or set in workflow: `ACTIONS_STEP_DEBUG: true`

## Local Testing

### Test Workflows Locally

Use [act](https://github.com/nektos/act):

```bash
# Install act
brew install act  # macOS

# Run specific workflow
act push -W .github/workflows/main.yml

# Run with specific event
act push -e .github/workflows/main.yml
```

### Test Docker Build

```bash
# Build locally
docker build -t agentbox:test -f Dockerfile .

# Test container
docker run --rm agentbox:test --help
```

### Test Helm Chart

```bash
# Lint
helm lint ./helm/agentbox

# Template
helm template agentbox ./helm/agentbox

# Package
helm package ./helm/agentbox
```

## Versioning

### Semantic Versioning

Use semantic versioning for tags:
- `v1.0.0` - Major release
- `v1.1.0` - Minor release
- `v1.1.1` - Patch release

### Creating a Release

1. Update version in code/docs
2. Create and push tag:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```
3. Workflow automatically:
   - Builds versioned image
   - Packages Helm chart
   - Creates GitHub release

## Best Practices

1. **Always test locally** before pushing
2. **Use semantic versioning** for releases
3. **Review PR checks** before merging
4. **Monitor security scans** regularly
5. **Keep artifacts retention** reasonable (90 days)
6. **Document breaking changes** in releases
7. **Use feature branches** for development
8. **Merge to main** only after all checks pass

## Next Steps

1. Push code to GitHub
2. Workflows run automatically
3. Monitor Actions tab
4. Download artifacts as needed
5. Create releases for stable versions

---

For more information, see `.github/workflows/README.md`

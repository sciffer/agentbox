# GitHub Actions Workflows

This directory contains GitHub Actions workflows for CI/CD of the AgentBox project.

## Workflows

### 1. `main.yml` - Main Branch CI/CD
**Triggers:** Push to `main` branch, manual dispatch

**Jobs:**
- **test**: Runs all unit tests
- **build-and-push**: Builds and pushes Docker image to GHCR
- **package-helm**: Packages Helm chart and uploads as artifact

**Outputs:**
- Docker image: `ghcr.io/<repo>/agentbox:latest` and `ghcr.io/<repo>/agentbox:main-<sha>`
- Helm chart: Uploaded as GitHub artifact

### 2. `ci.yml` - Full CI/CD Pipeline
**Triggers:** Push to `main`/`develop`, PRs, tags

**Jobs:**
- **test**: Runs tests, linting, formatting checks
- **build-docker**: Builds and pushes Docker image (on main/tags)
- **package-helm**: Packages Helm chart (on main/tags)
- **release**: Creates GitHub release (on tags)
- **security-scan**: Runs Trivy security scanner

### 3. `test.yml` - Test Suite
**Triggers:** Push to any branch, PRs

**Jobs:**
- **unit-tests**: Runs unit tests on multiple Go versions
- **lint**: Runs golangci-lint and format checks
- **helm-lint**: Lints Helm chart
- **docker-build-test**: Tests Docker image build

### 4. `release.yml` - Release Workflow
**Triggers:** Push of version tags (v*.*.*)

**Jobs:**
- **build-and-push**: Builds and pushes versioned Docker image
- **package-helm**: Packages versioned Helm chart
- **create-release**: Creates GitHub release with artifacts

## Usage

### Automatic Triggers

1. **On push to main:**
   - Tests run
   - Docker image built and pushed to GHCR
   - Helm chart packaged and uploaded as artifact

2. **On PR:**
   - Tests run
   - Linting and formatting checked
   - No builds/pushes

3. **On version tag (v1.0.0):**
   - Full release workflow
   - Versioned Docker image
   - GitHub release created
   - Helm chart attached to release

### Manual Triggers

Workflows can be manually triggered from the Actions tab in GitHub.

## Artifacts

### Docker Images
- Location: GitHub Container Registry (ghcr.io)
- Tags:
  - `latest` - Latest main branch build
  - `main-<sha>` - Specific commit on main
  - `v<version>` - Version tags
  - `v<major>.<minor>` - Minor version tags
  - `v<major>` - Major version tags

### Helm Charts
- Location: GitHub Actions Artifacts
- Retention: 90 days
- Format: `.tgz` files
- Also attached to GitHub releases for version tags

## Permissions

Workflows use the following permissions:
- `contents: read` - Read repository
- `contents: write` - Create releases (release workflow only)
- `packages: write` - Push to GHCR
- `security-events: write` - Upload security scan results

## Secrets

No additional secrets required. Uses `GITHUB_TOKEN` automatically provided by GitHub Actions.

## Local Testing

To test workflows locally, use [act](https://github.com/nektos/act):

```bash
# Install act
brew install act  # macOS
# or
curl https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash

# Run workflow
act push -W .github/workflows/main.yml
```

## Troubleshooting

### Docker Build Fails
- Check Dockerfile syntax
- Verify all dependencies are available
- Check build logs for specific errors

### Helm Chart Packaging Fails
- Verify Chart.yaml is valid
- Check that all template files are present
- Run `helm lint` locally

### Tests Fail
- Run tests locally: `go test ./tests/unit/...`
- Check Go version compatibility
- Verify all dependencies are up to date

## Best Practices

1. **Always test locally before pushing**
2. **Use semantic versioning for tags** (v1.0.0, v1.1.0, etc.)
3. **Review PR checks before merging**
4. **Monitor security scan results**
5. **Keep artifacts retention reasonable** (90 days default)

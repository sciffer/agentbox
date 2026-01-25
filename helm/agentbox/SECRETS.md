# Secrets and Environment Variables Configuration Guide

This document explains how to configure secrets and environment variables for the AgentBox Helm chart.

## Overview

AgentBox uses environment variables for configuration. These are divided into:
- **Secrets** - Sensitive values (JWT secret, passwords, OAuth credentials)
- **Configuration** - Non-sensitive settings (ports, timeouts, resource limits)

## Quick Reference

### Required Secrets

| Variable | Description | Default |
|----------|-------------|---------|
| `AGENTBOX_JWT_SECRET` | JWT signing secret (min 32 chars) | Must be set |

### Optional Secrets

| Variable | Description | Default |
|----------|-------------|---------|
| `AGENTBOX_ADMIN_USERNAME` | Initial admin username | `admin` |
| `AGENTBOX_ADMIN_PASSWORD` | Initial admin password | Auto-generated |
| `AGENTBOX_ADMIN_EMAIL` | Initial admin email | None |
| `AGENTBOX_GOOGLE_CLIENT_ID` | Google OAuth client ID | None (disables OAuth) |
| `AGENTBOX_GOOGLE_CLIENT_SECRET` | Google OAuth client secret | None |
| `AGENTBOX_DB_DSN` | PostgreSQL connection string | None (uses SQLite) |

### API Configuration Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `AGENTBOX_HOST` | Server bind address | `0.0.0.0` |
| `AGENTBOX_PORT` | Server port | `8080` |
| `AGENTBOX_LOG_LEVEL` | Log level (debug/info/warn/error) | `info` |
| `AGENTBOX_DB_PATH` | SQLite database path | `/data/agentbox.db` |
| `AGENTBOX_NAMESPACE_PREFIX` | Prefix for sandbox namespaces | `agentbox-` |
| `AGENTBOX_RUNTIME_CLASS` | Kubernetes RuntimeClass | None |
| `AGENTBOX_AUTH_ENABLED` | Enable authentication | `true` |
| `AGENTBOX_JWT_EXPIRY` | JWT token expiry duration | `24h` |
| `AGENTBOX_API_KEY_PREFIX` | Prefix for API keys | `ak_` |
| `AGENTBOX_DEFAULT_CPU_LIMIT` | Default CPU limit for sandboxes | `1000m` |
| `AGENTBOX_DEFAULT_MEMORY_LIMIT` | Default memory limit | `512Mi` |
| `AGENTBOX_DEFAULT_STORAGE_LIMIT` | Default storage limit | `1Gi` |
| `AGENTBOX_MAX_ENVIRONMENTS_PER_USER` | Max sandboxes per user | `10` |
| `AGENTBOX_DEFAULT_TIMEOUT` | Default sandbox timeout (seconds) | `3600` |
| `AGENTBOX_MAX_TIMEOUT` | Maximum sandbox timeout (seconds) | `86400` |
| `AGENTBOX_STARTUP_TIMEOUT` | Sandbox startup timeout (seconds) | `300` |
| `AGENTBOX_METRICS_ENABLED` | Enable metrics collection | `true` |
| `AGENTBOX_METRICS_COLLECTION_INTERVAL` | Metrics interval | `30s` |

### UI Configuration Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `VITE_API_URL` / `API_URL` | Backend API URL | `http://agentbox-api:8080` |
| `VITE_WS_URL` | WebSocket URL | Same as API URL |
| `VITE_GOOGLE_OAUTH_ENABLED` | Show Google OAuth button | `false` |

## Configuration Methods

### Method 1: Chart-Managed Secrets (Development/Testing)

The chart can create a Kubernetes secret with the required values:

```yaml
# values.yaml
secrets:
  create: true
  jwtSecret: "your-jwt-secret-minimum-32-characters-long"
  adminUsername: "admin"
  adminPassword: "secure-password"
  adminEmail: "admin@example.com"
  googleClientId: ""
  googleClientSecret: ""
  databaseDSN: ""  # Leave empty for SQLite
```

### Method 2: Externally Managed Secrets (Production)

For production, disable chart-managed secrets and reference your own:

```yaml
# values.yaml
secrets:
  create: false

api:
  envFrom:
    secretRef: my-external-secret
```

Create the external secret:

```bash
kubectl create secret generic my-external-secret \
  --from-literal=AGENTBOX_JWT_SECRET="$(openssl rand -base64 32)" \
  --from-literal=AGENTBOX_ADMIN_USERNAME="admin" \
  --from-literal=AGENTBOX_ADMIN_PASSWORD="$(openssl rand -base64 16)" \
  --from-literal=AGENTBOX_ADMIN_EMAIL="admin@example.com"
```

## Detailed Configuration

### Server Configuration

```yaml
api:
  env:
    AGENTBOX_HOST: "0.0.0.0"
    AGENTBOX_PORT: "8080"
    AGENTBOX_LOG_LEVEL: "info"  # debug, info, warn, error
```

### Database Configuration

**SQLite (Default):**
```yaml
api:
  env:
    AGENTBOX_DB_PATH: "/data/agentbox.db"
  persistence:
    enabled: true
    size: 1Gi
```

**PostgreSQL:**
```yaml
secrets:
  databaseDSN: "postgresql://user:password@host:5432/agentbox?sslmode=require"

api:
  env:
    AGENTBOX_DB_PATH: ""  # Clear SQLite path
  persistence:
    enabled: false  # Not needed for PostgreSQL
```

### Authentication Configuration

```yaml
api:
  env:
    AGENTBOX_AUTH_ENABLED: "true"
    AGENTBOX_JWT_EXPIRY: "24h"      # Token validity: 1h, 24h, 7d, etc.
    AGENTBOX_API_KEY_PREFIX: "ak_"  # Prefix for generated API keys

secrets:
  jwtSecret: "your-secret-minimum-32-characters"
  adminUsername: "admin"
  adminPassword: "secure-password"
```

### Google OAuth Configuration

1. Create OAuth credentials in [Google Cloud Console](https://console.cloud.google.com/apis/credentials)
2. Configure the secrets:

```yaml
secrets:
  googleClientId: "your-client-id.apps.googleusercontent.com"
  googleClientSecret: "your-client-secret"

ui:
  env:
    VITE_GOOGLE_OAUTH_ENABLED: "true"
```

### Sandbox Resource Limits

```yaml
api:
  env:
    # Default limits for new sandboxes
    AGENTBOX_DEFAULT_CPU_LIMIT: "1000m"      # 1 CPU core
    AGENTBOX_DEFAULT_MEMORY_LIMIT: "512Mi"   # 512 MB RAM
    AGENTBOX_DEFAULT_STORAGE_LIMIT: "1Gi"    # 1 GB storage
    
    # User quotas
    AGENTBOX_MAX_ENVIRONMENTS_PER_USER: "10"
```

### Timeout Configuration

```yaml
api:
  env:
    # Sandbox timeouts (in seconds)
    AGENTBOX_DEFAULT_TIMEOUT: "3600"    # 1 hour default
    AGENTBOX_MAX_TIMEOUT: "86400"       # 24 hours maximum
    AGENTBOX_STARTUP_TIMEOUT: "300"     # 5 minutes to start
```

### Kubernetes Configuration

```yaml
api:
  env:
    AGENTBOX_NAMESPACE_PREFIX: "sandbox-"  # Namespace prefix for sandboxes
    AGENTBOX_RUNTIME_CLASS: "gvisor"       # Use gVisor for isolation (optional)
    AGENTBOX_KUBECONFIG: ""                # Use in-cluster config (default)
```

### Metrics Configuration

```yaml
api:
  env:
    AGENTBOX_METRICS_ENABLED: "true"
    AGENTBOX_METRICS_COLLECTION_INTERVAL: "30s"
```

## UI Configuration

The UI requires the API URL to communicate with the backend:

```yaml
ui:
  env:
    API_URL: "http://agentbox-api:8080"  # Internal service URL
```

For external access or different domains:
```yaml
ui:
  env:
    API_URL: "https://api.agentbox.example.com"
    VITE_WS_URL: "wss://api.agentbox.example.com"
    VITE_GOOGLE_OAUTH_ENABLED: "true"
```

## Example Configurations

### Development

```yaml
# values-dev.yaml
secrets:
  create: true
  jwtSecret: "dev-secret-not-for-production-use"

api:
  env:
    AGENTBOX_LOG_LEVEL: "debug"
    AGENTBOX_AUTH_ENABLED: "true"
    AGENTBOX_DEFAULT_TIMEOUT: "7200"
  persistence:
    enabled: true
    size: 1Gi

ui:
  env:
    API_URL: "http://agentbox-api:8080"
```

### Production

```yaml
# values-production.yaml
secrets:
  create: false  # Use external secret management

api:
  replicaCount: 3
  envFrom:
    secretRef: agentbox-prod-secrets
  env:
    AGENTBOX_LOG_LEVEL: "info"
    AGENTBOX_AUTH_ENABLED: "true"
    AGENTBOX_JWT_EXPIRY: "8h"
    AGENTBOX_DEFAULT_CPU_LIMIT: "2000m"
    AGENTBOX_DEFAULT_MEMORY_LIMIT: "1Gi"
    AGENTBOX_MAX_ENVIRONMENTS_PER_USER: "5"
    AGENTBOX_RUNTIME_CLASS: "gvisor"

ui:
  replicaCount: 2
  env:
    API_URL: "https://api.agentbox.example.com"
    VITE_GOOGLE_OAUTH_ENABLED: "true"
```

Pre-create the production secret:
```bash
kubectl create secret generic agentbox-prod-secrets \
  --from-literal=AGENTBOX_JWT_SECRET="$(openssl rand -base64 64)" \
  --from-literal=AGENTBOX_ADMIN_USERNAME="admin" \
  --from-literal=AGENTBOX_ADMIN_PASSWORD="$(openssl rand -base64 24)" \
  --from-literal=AGENTBOX_ADMIN_EMAIL="admin@yourcompany.com" \
  --from-literal=AGENTBOX_GOOGLE_CLIENT_ID="your-prod-client-id" \
  --from-literal=AGENTBOX_GOOGLE_CLIENT_SECRET="your-prod-client-secret" \
  --from-literal=AGENTBOX_DB_DSN="postgresql://agentbox:password@postgres:5432/agentbox?sslmode=require"
```

### With External Secrets Operator

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: agentbox-secrets
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: vault-backend
    kind: ClusterSecretStore
  target:
    name: agentbox-secrets
  data:
    - secretKey: AGENTBOX_JWT_SECRET
      remoteRef:
        key: agentbox/auth
        property: jwt_secret
    - secretKey: AGENTBOX_ADMIN_PASSWORD
      remoteRef:
        key: agentbox/auth
        property: admin_password
    - secretKey: AGENTBOX_GOOGLE_CLIENT_ID
      remoteRef:
        key: agentbox/oauth
        property: google_client_id
    - secretKey: AGENTBOX_GOOGLE_CLIENT_SECRET
      remoteRef:
        key: agentbox/oauth
        property: google_client_secret
    - secretKey: AGENTBOX_DB_DSN
      remoteRef:
        key: agentbox/database
        property: dsn
```

## Security Best Practices

1. **Never commit secrets to version control**
   ```bash
   # Add to .gitignore
   *-secrets.yaml
   values-*.yaml
   ```

2. **Use strong, randomly generated secrets**
   ```bash
   # Generate JWT secret (minimum 32 characters)
   openssl rand -base64 32
   
   # Generate admin password
   openssl rand -base64 24
   ```

3. **Rotate secrets regularly**
   - JWT secrets should be rotated periodically
   - Use rolling deployments to minimize downtime

4. **Use separate secrets per environment**
   - Different secrets for dev, staging, production
   - Never share production secrets with other environments

5. **Restrict secret access with RBAC**
   ```yaml
   apiVersion: rbac.authorization.k8s.io/v1
   kind: Role
   metadata:
     name: agentbox-secret-reader
   rules:
     - apiGroups: [""]
       resources: ["secrets"]
       resourceNames: ["agentbox-secrets"]
       verbs: ["get"]
   ```

6. **Enable encryption at rest**
   - Configure Kubernetes to encrypt secrets at rest
   - Use a KMS provider for additional security

## Troubleshooting

### Secret not found errors

```bash
# Check if secret exists
kubectl get secret agentbox-secrets

# View secret keys (not values)
kubectl describe secret agentbox-secrets

# Decode a specific value
kubectl get secret agentbox-secrets -o jsonpath='{.data.AGENTBOX_JWT_SECRET}' | base64 -d
```

### Pod not starting due to missing secret

```bash
# Check pod events
kubectl describe pod -l app.kubernetes.io/component=api

# Check if envFrom is correctly configured
kubectl get deployment agentbox-api -o yaml | grep -A5 envFrom
```

### Verify environment variables are set

```bash
# Exec into pod and check environment
kubectl exec -it deployment/agentbox-api -- env | grep AGENTBOX

# Check specific variables
kubectl exec -it deployment/agentbox-api -- env | grep -E "(JWT|ADMIN|GOOGLE|DB)"
```

### Authentication issues

```bash
# Check if JWT secret is set
kubectl exec -it deployment/agentbox-api -- env | grep JWT_SECRET

# Verify auth is enabled
kubectl exec -it deployment/agentbox-api -- env | grep AUTH_ENABLED
```

### Database connection issues

```bash
# Check database configuration
kubectl exec -it deployment/agentbox-api -- env | grep -E "(DB_PATH|DB_DSN)"

# Check if persistence is mounted (for SQLite)
kubectl exec -it deployment/agentbox-api -- ls -la /data/
```

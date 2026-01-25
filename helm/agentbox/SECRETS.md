# Secrets Configuration Guide

This document explains how to configure secrets for the AgentBox Helm chart.

## Overview

AgentBox requires several secrets for secure operation:
- JWT signing secret for authentication tokens
- Optional: Google OAuth credentials for social login
- Optional: Database password (for PostgreSQL deployments)

## Configuration Methods

### Method 1: Chart-Managed Secrets (Development/Testing)

The chart can create a Kubernetes secret with the required values:

```yaml
secrets:
  create: true
  jwtSecret: "your-jwt-secret-here"
  googleClientId: ""           # Optional: Google OAuth client ID
  googleClientSecret: ""       # Optional: Google OAuth client secret
  databasePassword: ""         # Optional: PostgreSQL password
```

### Method 2: Externally Managed Secrets (Production)

For production, disable chart-managed secrets and reference your own:

```yaml
secrets:
  create: false

api:
  envFrom:
    secretRef: my-external-secret  # Your pre-created secret
```

Create the external secret manually:

```bash
kubectl create secret generic my-external-secret \
  --from-literal=JWT_SECRET="$(openssl rand -base64 32)" \
  --from-literal=ADMIN_USERNAME="admin" \
  --from-literal=ADMIN_PASSWORD="secure-password" \
  --from-literal=ADMIN_EMAIL="admin@example.com" \
  --from-literal=GOOGLE_CLIENT_ID="your-client-id" \
  --from-literal=GOOGLE_CLIENT_SECRET="your-client-secret"
```

## Secret Values Reference

### Chart-Managed Secret (`agentbox-secrets`)

When `secrets.create: true`, the chart creates a secret named `agentbox-secrets` with:

| Key | Source | Description |
|-----|--------|-------------|
| `JWT_SECRET` | `secrets.jwtSecret` | JWT signing secret |
| `GOOGLE_CLIENT_ID` | `secrets.googleClientId` | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | `secrets.googleClientSecret` | Google OAuth client secret |
| `DATABASE_PASSWORD` | `secrets.databasePassword` | PostgreSQL password |

### Expected Environment Variables

The API backend expects these environment variables (from secrets or ConfigMap):

| Variable | Description | Required |
|----------|-------------|----------|
| `JWT_SECRET` | Secret key for signing JWT tokens | Yes |
| `ADMIN_USERNAME` | Initial admin username | No (defaults to "admin") |
| `ADMIN_PASSWORD` | Initial admin password | No (auto-generated if not set) |
| `ADMIN_EMAIL` | Initial admin email | No |
| `GOOGLE_CLIENT_ID` | Google OAuth 2.0 client ID | No (disables Google OAuth) |
| `GOOGLE_CLIENT_SECRET` | Google OAuth 2.0 client secret | No |
| `DATABASE_PASSWORD` | PostgreSQL password | No (only for PostgreSQL) |

## Creating Secrets Manually

### JWT Secret

Generate a secure random JWT secret:

```bash
kubectl create secret generic agentbox-secrets \
  --from-literal=JWT_SECRET="$(openssl rand -base64 32)"
```

### Admin Credentials

Create a secret with admin credentials:

```bash
kubectl create secret generic agentbox-admin \
  --from-literal=ADMIN_USERNAME="admin" \
  --from-literal=ADMIN_PASSWORD="$(openssl rand -base64 16)" \
  --from-literal=ADMIN_EMAIL="admin@example.com"
```

### Google OAuth Credentials

If using Google OAuth:

```bash
kubectl create secret generic agentbox-oauth \
  --from-literal=GOOGLE_CLIENT_ID="your-client-id.apps.googleusercontent.com" \
  --from-literal=GOOGLE_CLIENT_SECRET="your-client-secret"
```

### Combined Secret (All-in-One)

For simplicity, create a single secret with all values:

```bash
kubectl create secret generic agentbox-secrets \
  --from-literal=JWT_SECRET="$(openssl rand -base64 32)" \
  --from-literal=ADMIN_USERNAME="admin" \
  --from-literal=ADMIN_PASSWORD="secure-password-here" \
  --from-literal=ADMIN_EMAIL="admin@example.com" \
  --from-literal=GOOGLE_CLIENT_ID="" \
  --from-literal=GOOGLE_CLIENT_SECRET=""
```

## Example Configurations

### Development (Minimal)

```yaml
# values-dev.yaml
secrets:
  create: true
  jwtSecret: "dev-secret-not-for-production"
```

### Production (External Secrets)

```yaml
# values-production.yaml
secrets:
  create: false

api:
  envFrom:
    secretRef: agentbox-prod-secrets
```

Pre-create the secret:
```bash
kubectl create secret generic agentbox-prod-secrets \
  --from-literal=JWT_SECRET="$(openssl rand -base64 64)" \
  --from-literal=ADMIN_USERNAME="admin" \
  --from-literal=ADMIN_PASSWORD="$(openssl rand -base64 24)" \
  --from-literal=ADMIN_EMAIL="admin@yourcompany.com" \
  --from-literal=GOOGLE_CLIENT_ID="your-prod-client-id" \
  --from-literal=GOOGLE_CLIENT_SECRET="your-prod-client-secret"
```

### With External Secrets Operator

If using [External Secrets Operator](https://external-secrets.io/):

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
    - secretKey: JWT_SECRET
      remoteRef:
        key: agentbox/jwt
        property: secret
    - secretKey: GOOGLE_CLIENT_ID
      remoteRef:
        key: agentbox/google-oauth
        property: client_id
    - secretKey: GOOGLE_CLIENT_SECRET
      remoteRef:
        key: agentbox/google-oauth
        property: client_secret
```

## UI Configuration

The UI frontend does not require secrets directly. It communicates with the API backend which handles authentication. Configure the API URL:

```yaml
ui:
  env:
    API_URL: http://agentbox-api:8080  # Internal service URL
```

For external API access (e.g., different domain):
```yaml
ui:
  env:
    API_URL: https://api.agentbox.example.com
```

## Security Best Practices

1. **Never commit secrets to version control**
   - Use `.gitignore` to exclude `*-secrets.yaml` files
   - Use environment variables or secret management tools

2. **Use strong, randomly generated secrets**
   ```bash
   openssl rand -base64 32  # For JWT secret
   openssl rand -base64 24  # For passwords
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
kubectl get secret agentbox-secrets -o jsonpath='{.data.JWT_SECRET}' | base64 -d
```

### Pod not starting due to missing secret

```bash
# Check pod events
kubectl describe pod -l app.kubernetes.io/component=api

# Check if envFrom is correctly configured
kubectl get deployment agentbox-api -o yaml | grep -A5 envFrom
```

### Verify secret is mounted correctly

```bash
# Exec into pod and check environment
kubectl exec -it deployment/agentbox-api -- env | grep -E "(JWT|ADMIN|GOOGLE)"
```

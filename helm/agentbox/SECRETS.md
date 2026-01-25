# Secrets Configuration Guide

This document explains how to configure secrets for the AgentBox Helm chart.

## Overview

AgentBox requires several secrets for secure operation:
- Database credentials (PostgreSQL DSN or SQLite path)
- Admin user credentials
- JWT signing secret
- Optional: Google OAuth credentials

## Configuration Methods

### Method 1: Auto-Generated Secrets (Development)

The chart can automatically create secrets with auto-generated values:

```yaml
secrets:
  createSecrets: true
  # Optional: Override defaults
  adminUsername: "admin"
  adminPassword: "my-secure-password"  # If not set, auto-generated
  jwtSecret: "my-jwt-secret"  # If not set, auto-generated
  dbDSN: "postgresql://user:pass@host/db"  # For PostgreSQL
  # OR
  dbPath: "/data/agentbox.db"  # For SQLite
```

### Method 2: Existing Kubernetes Secrets (Production)

Reference existing Kubernetes secrets:

```yaml
secrets:
  createSecrets: false
  
  # Database secret
  dbSecretName: "agentbox-db-secret"
  dbSecretKey: "dsn"  # Key containing PostgreSQL connection string
  
  # Admin credentials secret
  adminSecretName: "agentbox-admin-secret"
  adminUsernameKey: "username"
  adminPasswordKey: "password"
  adminEmailKey: "email"  # Optional
  
  # JWT secret
  jwtSecretName: "agentbox-jwt-secret"
  jwtSecretKey: "secret"
  
  # Google OAuth (optional)
  googleOAuthSecretName: "agentbox-oauth-secret"
  googleClientIDKey: "client_id"
  googleClientSecretKey: "client_secret"
```

## Creating Secrets Manually

### Database Secret (PostgreSQL)

```bash
kubectl create secret generic agentbox-db-secret \
  --from-literal=dsn="postgresql://user:password@host:5432/agentbox?sslmode=require"
```

### Admin Credentials Secret

```bash
kubectl create secret generic agentbox-admin-secret \
  --from-literal=username="admin" \
  --from-literal=password="secure-password" \
  --from-literal=email="admin@example.com"
```

### JWT Secret

```bash
kubectl create secret generic agentbox-jwt-secret \
  --from-literal=secret="$(openssl rand -base64 32)"
```

### Google OAuth Secret (Optional)

```bash
kubectl create secret generic agentbox-oauth-secret \
  --from-literal=client_id="your-client-id" \
  --from-literal=client_secret="your-client-secret"
```

## Environment Variables

The following environment variables are set from secrets:

| Variable | Source | Description |
|----------|--------|-------------|
| `AGENTBOX_DB_DSN` | `dbSecretName` → `dbSecretKey` | PostgreSQL connection string |
| `AGENTBOX_DB_PATH` | `secrets.dbPath` | SQLite file path |
| `AGENTBOX_ADMIN_USERNAME` | `adminSecretName` → `adminUsernameKey` | Default admin username |
| `AGENTBOX_ADMIN_PASSWORD` | `adminSecretName` → `adminPasswordKey` | Default admin password |
| `AGENTBOX_ADMIN_EMAIL` | `adminSecretName` → `adminEmailKey` | Default admin email (optional) |
| `AGENTBOX_JWT_SECRET` | `jwtSecretName` → `jwtSecretKey` | JWT signing secret |
| `AGENTBOX_JWT_EXPIRY` | `secrets.jwtExpiry` | JWT token expiry (default: "15m") |
| `AGENTBOX_API_KEY_PREFIX` | `secrets.apiKeyPrefix` | API key prefix (default: "ak_live_") |
| `AGENTBOX_METRICS_ENABLED` | `secrets.metricsEnabled` | Enable metrics collection |
| `AGENTBOX_METRICS_COLLECTION_INTERVAL` | `secrets.metricsCollectionInterval` | Collection interval (default: "30s") |
| `AGENTBOX_GOOGLE_CLIENT_ID` | `googleOAuthSecretName` → `googleClientIDKey` | Google OAuth client ID |
| `AGENTBOX_GOOGLE_CLIENT_SECRET` | `googleOAuthSecretName` → `googleClientSecretKey` | Google OAuth client secret |

## Example: Production Deployment

```yaml
# values-production.yaml
secrets:
  createSecrets: false
  
  dbSecretName: "agentbox-postgres-secret"
  dbSecretKey: "connection-string"
  
  adminSecretName: "agentbox-admin-credentials"
  adminUsernameKey: "username"
  adminPasswordKey: "password"
  adminEmailKey: "email"
  
  jwtSecretName: "agentbox-jwt-secret"
  jwtSecretKey: "secret"
  
  metricsEnabled: true
  metricsCollectionInterval: "30s"
  
  apiKeyPrefix: "ak_prod_"
```

Deploy with:
```bash
helm install agentbox ./helm/agentbox -f values-production.yaml
```

## Security Best Practices

1. **Never commit secrets to version control**
2. **Use Kubernetes secrets or external secret management** (e.g., Sealed Secrets, External Secrets Operator)
3. **Rotate secrets regularly**
4. **Use strong, randomly generated passwords**
5. **Enable RBAC** to restrict secret access
6. **Use separate secrets for different environments** (dev, staging, prod)

## Troubleshooting

### Secret not found errors

If you see errors about missing secrets:
1. Verify the secret exists: `kubectl get secret <secret-name>`
2. Check the secret keys: `kubectl describe secret <secret-name>`
3. Ensure the key names match in `values.yaml`

### Auto-generated secrets

When `createSecrets: true`, secrets are auto-generated on first install. To view them:
```bash
kubectl get secret agentbox-secrets -o yaml
kubectl get secret agentbox-secrets -o jsonpath='{.data.jwt-secret}' | base64 -d
```

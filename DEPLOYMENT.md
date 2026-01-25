# AgentBox Deployment Guide

This guide covers deploying AgentBox using Docker and Helm.

## Prerequisites

- Docker 20.10+
- Kubernetes 1.19+
- Helm 3.0+
- kubectl configured to access your cluster
- (Optional) gVisor runtime class installed

## Architecture

AgentBox consists of two services:
- **API Backend** - Go-based REST API server (port 8080)
- **UI Frontend** - React-based web interface (port 8080 in container)

## Building Docker Images

### Backend API

```bash
# Build the image
docker build -t agentbox-api:1.0.0 .

# Test locally
docker run -p 8080:8080 \
  -e AGENTBOX_JWT_SECRET="test-secret-min-32-characters" \
  agentbox-api:1.0.0
```

### UI Frontend

```bash
# Build the image
docker build -t agentbox-ui:1.0.0 ./ui

# Test locally
docker run -p 3000:8080 \
  -e VITE_API_URL=http://localhost:8080 \
  agentbox-ui:1.0.0
```

### Multi-Platform Build

```bash
# Build multi-arch images
docker buildx build --platform linux/amd64,linux/arm64 -t agentbox-api:1.0.0 .
docker buildx build --platform linux/amd64,linux/arm64 -t agentbox-ui:1.0.0 ./ui
```

### Push to Registry

```bash
# Tag and push
docker tag agentbox-api:1.0.0 your-registry/agentbox-api:1.0.0
docker tag agentbox-ui:1.0.0 your-registry/agentbox-ui:1.0.0
docker push your-registry/agentbox-api:1.0.0
docker push your-registry/agentbox-ui:1.0.0
```

## Deploying with Helm

### 1. Prepare Your Cluster

Ensure your cluster has:
- RBAC enabled
- Sufficient resources
- (Optional) gVisor runtime class

### 2. Install gVisor (Optional but Recommended)

```bash
# For GKE
gcloud container clusters update CLUSTER_NAME \
  --sandbox-type=gvisor

# For other clusters, see: https://gvisor.dev/docs/user_guide/install/
```

### 3. Install AgentBox

#### Basic Installation

```bash
helm install agentbox ./helm/agentbox
```

#### With Custom Images

```bash
helm install agentbox ./helm/agentbox \
  --set api.image.repository=your-registry/agentbox-api \
  --set api.image.tag=1.0.0 \
  --set ui.image.repository=your-registry/agentbox-ui \
  --set ui.image.tag=1.0.0
```

#### Development Configuration

```yaml
# dev-values.yaml
secrets:
  create: true
  jwtSecret: "dev-secret-not-for-production-use"
  adminUsername: "admin"
  adminPassword: "admin123"

api:
  env:
    AGENTBOX_LOG_LEVEL: "debug"
    AGENTBOX_AUTH_ENABLED: "true"

ui:
  env:
    API_URL: "http://agentbox-api:8080"
```

```bash
helm install agentbox ./helm/agentbox -f dev-values.yaml
```

### 4. Verify Installation

```bash
# Check pods
kubectl get pods -l app.kubernetes.io/instance=agentbox

# Check services
kubectl get svc -l app.kubernetes.io/instance=agentbox

# Check logs
kubectl logs -l app.kubernetes.io/component=api
kubectl logs -l app.kubernetes.io/component=ui

# Test API
kubectl port-forward svc/agentbox-api 8080:8080
curl http://localhost:8080/api/v1/health
```

## Production Deployment

### Recommended Production Values

```yaml
# production-values.yaml
secrets:
  create: false  # Use external secret management

api:
  replicaCount: 3
  image:
    repository: your-registry/agentbox-api
    tag: "1.0.0"
    pullPolicy: Always
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
    AGENTBOX_METRICS_ENABLED: "true"
  resources:
    limits:
      cpu: 1000m
      memory: 1Gi
    requests:
      cpu: 500m
      memory: 512Mi
  persistence:
    enabled: true
    storageClass: "fast-ssd"
    size: 10Gi

ui:
  replicaCount: 2
  image:
    repository: your-registry/agentbox-ui
    tag: "1.0.0"
    pullPolicy: Always
  env:
    API_URL: "https://api.agentbox.example.com"
    VITE_GOOGLE_OAUTH_ENABLED: "true"
  resources:
    limits:
      cpu: 500m
      memory: 512Mi
    requests:
      cpu: 100m
      memory: 128Mi

ingress:
  enabled: true
  className: "nginx"
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: agentbox.example.com
      paths:
        - path: /
          pathType: Prefix
          service: ui
        - path: /api
          pathType: Prefix
          service: api
  tls:
    - secretName: agentbox-tls
      hosts:
        - agentbox.example.com

rbac:
  create: true

serviceAccount:
  create: true
```

### Create Production Secret

```bash
kubectl create secret generic agentbox-prod-secrets \
  --from-literal=AGENTBOX_JWT_SECRET="$(openssl rand -base64 64)" \
  --from-literal=AGENTBOX_AUTH_SECRET="$(openssl rand -base64 64)" \
  --from-literal=AGENTBOX_ADMIN_USERNAME="admin" \
  --from-literal=AGENTBOX_ADMIN_PASSWORD="$(openssl rand -base64 24)" \
  --from-literal=AGENTBOX_ADMIN_EMAIL="admin@yourcompany.com" \
  --from-literal=AGENTBOX_GOOGLE_CLIENT_ID="your-client-id" \
  --from-literal=AGENTBOX_GOOGLE_CLIENT_SECRET="your-client-secret"
```

### Deploy

```bash
helm install agentbox ./helm/agentbox -f production-values.yaml
```

## Environment Variables Reference

### API Backend

| Variable | Description | Default |
|----------|-------------|---------|
| `AGENTBOX_JWT_SECRET` | JWT signing secret | Required |
| `AGENTBOX_AUTH_SECRET` | Config validation secret | Required |
| `AGENTBOX_HOST` | Server bind address | `0.0.0.0` |
| `AGENTBOX_PORT` | Server port | `8080` |
| `AGENTBOX_LOG_LEVEL` | Log level | `info` |
| `AGENTBOX_DB_PATH` | SQLite database path | `/data/agentbox.db` |
| `AGENTBOX_DB_DSN` | PostgreSQL connection string | None |
| `AGENTBOX_AUTH_ENABLED` | Enable authentication | `true` |
| `AGENTBOX_JWT_EXPIRY` | JWT token expiry | `24h` |
| `AGENTBOX_API_KEY_PREFIX` | API key prefix | `ak_` |
| `AGENTBOX_NAMESPACE_PREFIX` | Sandbox namespace prefix | `agentbox-` |
| `AGENTBOX_RUNTIME_CLASS` | Kubernetes RuntimeClass | None |
| `AGENTBOX_DEFAULT_CPU_LIMIT` | Default CPU limit | `1000m` |
| `AGENTBOX_DEFAULT_MEMORY_LIMIT` | Default memory limit | `512Mi` |
| `AGENTBOX_DEFAULT_STORAGE_LIMIT` | Default storage limit | `1Gi` |
| `AGENTBOX_MAX_ENVIRONMENTS_PER_USER` | Max sandboxes per user | `10` |
| `AGENTBOX_DEFAULT_TIMEOUT` | Default timeout (seconds) | `3600` |
| `AGENTBOX_MAX_TIMEOUT` | Max timeout (seconds) | `86400` |
| `AGENTBOX_STARTUP_TIMEOUT` | Startup timeout (seconds) | `300` |
| `AGENTBOX_METRICS_ENABLED` | Enable metrics | `true` |
| `AGENTBOX_METRICS_COLLECTION_INTERVAL` | Collection interval | `30s` |
| `AGENTBOX_ADMIN_USERNAME` | Initial admin username | `admin` |
| `AGENTBOX_ADMIN_PASSWORD` | Initial admin password | Auto-generated |
| `AGENTBOX_ADMIN_EMAIL` | Initial admin email | None |

### UI Frontend

| Variable | Description | Default |
|----------|-------------|---------|
| `VITE_API_URL` / `API_URL` | Backend API URL | `http://agentbox-api:8080` |
| `VITE_WS_URL` | WebSocket URL | Same as API URL |
| `VITE_GOOGLE_OAUTH_ENABLED` | Show Google OAuth button | `false` |

## Upgrading

```bash
# Update image
helm upgrade agentbox ./helm/agentbox \
  --set api.image.tag=1.0.1 \
  --set ui.image.tag=1.0.1 \
  --reuse-values

# Update with new values
helm upgrade agentbox ./helm/agentbox -f new-values.yaml
```

## Rolling Back

```bash
# List releases
helm history agentbox

# Rollback to previous version
helm rollback agentbox

# Rollback to specific revision
helm rollback agentbox 1
```

## Uninstalling

```bash
# Uninstall Helm release
helm uninstall agentbox

# Clean up created namespaces (sandbox environments)
kubectl get namespaces | grep agentbox- | \
  awk '{print $1}' | xargs kubectl delete namespace

# Clean up PVCs
kubectl delete pvc -l app.kubernetes.io/instance=agentbox
```

## Troubleshooting

### Pod Not Starting

```bash
# Check pod status
kubectl describe pod -l app.kubernetes.io/component=api

# Check events
kubectl get events --sort-by='.lastTimestamp'

# Check logs
kubectl logs -l app.kubernetes.io/component=api
```

### Authentication Issues

```bash
# Verify JWT secret is set
kubectl exec -it deployment/agentbox-api -- env | grep JWT

# Check if auth is enabled
kubectl exec -it deployment/agentbox-api -- env | grep AUTH
```

### RBAC Issues

```bash
# Check service account
kubectl get serviceaccount agentbox

# Check cluster role
kubectl get clusterrole agentbox-cluster-reader

# Check cluster role binding
kubectl get clusterrolebinding agentbox-cluster-reader

# Test permissions
kubectl auth can-i list nodes \
  --as=system:serviceaccount:agentbox:agentbox
```

### Database Issues

```bash
# Check database path/connection
kubectl exec -it deployment/agentbox-api -- env | grep DB

# Check persistence is mounted
kubectl exec -it deployment/agentbox-api -- ls -la /data/
```

### Health Check Failures

```bash
# Test health endpoint manually
kubectl exec -it deployment/agentbox-api -- \
  wget -qO- http://localhost:8080/api/v1/health

# Check probe configuration
kubectl get deployment agentbox-api -o yaml | grep -A10 livenessProbe
```

## Security Best Practices

1. **Use Strong Secrets**
   ```bash
   # Generate strong JWT secret
   openssl rand -base64 64
   ```

2. **Enable Authentication**
   ```yaml
   api:
     env:
       AGENTBOX_AUTH_ENABLED: "true"
   ```

3. **Use Network Policies**
   - The Helm chart creates network policies for sandbox isolation

4. **Use gVisor Runtime**
   ```yaml
   api:
     env:
       AGENTBOX_RUNTIME_CLASS: "gvisor"
   ```

5. **External Secret Management**
   - Use Sealed Secrets, External Secrets Operator, or Vault
   - Never commit secrets to git

6. **Regular Updates**
   - Keep images updated
   - Scan for vulnerabilities
   - Update Helm chart regularly

## Next Steps

- Configure monitoring with Prometheus/Grafana
- Set up log aggregation (ELK, Loki)
- Configure backup for SQLite database
- Set up CI/CD pipeline for automated deployments
- Configure alerts for critical metrics

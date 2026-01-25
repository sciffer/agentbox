# AgentBox Helm Chart

This Helm chart deploys the AgentBox service (API backend and UI frontend) to a Kubernetes cluster.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- RBAC enabled cluster
- gVisor runtime class (optional, for enhanced isolation)

## Installation

### Quick Start

```bash
# Add the chart repository (if using a repository)
helm repo add agentbox https://charts.example.com
helm repo update

# Install with default values
helm install agentbox ./helm/agentbox

# Or install with custom values
helm install agentbox ./helm/agentbox -f my-values.yaml
```

### Build and Install from Local Chart

```bash
# Build Docker images
docker build -t agentbox/api:1.0.0 .
docker build -t agentbox/ui:1.0.0 ./ui

# Load into cluster (for local testing)
kind load docker-image agentbox/api:1.0.0  # For kind
kind load docker-image agentbox/ui:1.0.0
# OR
minikube image load agentbox/api:1.0.0     # For minikube
minikube image load agentbox/ui:1.0.0

# Install chart
helm install agentbox ./helm/agentbox \
  --set api.image.repository=agentbox/api \
  --set api.image.tag=1.0.0 \
  --set ui.image.repository=agentbox/ui \
  --set ui.image.tag=1.0.0
```

## Architecture

AgentBox consists of two components:

1. **API Backend** - Go-based REST API server for managing sandbox environments
2. **UI Frontend** - React-based web interface for interacting with the API

Both components are deployed as separate Kubernetes deployments with their own services.

## Configuration

### Global Settings

| Parameter | Description | Default |
|-----------|-------------|---------|
| `global.imagePullSecrets` | Global image pull secrets | `[]` |

### API Backend Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `api.enabled` | Enable API backend deployment | `true` |
| `api.replicaCount` | Number of API replicas | `1` |
| `api.image.repository` | API image repository | `agentbox/api` |
| `api.image.tag` | API image tag | `latest` |
| `api.image.pullPolicy` | API image pull policy | `IfNotPresent` |
| `api.service.type` | API service type | `ClusterIP` |
| `api.service.port` | API service port | `8080` |
| `api.resources.limits.cpu` | API CPU limit | `500m` |
| `api.resources.limits.memory` | API memory limit | `512Mi` |
| `api.resources.requests.cpu` | API CPU request | `100m` |
| `api.resources.requests.memory` | API memory request | `128Mi` |
| `api.envFrom.secretRef` | Secret containing environment variables | `agentbox-secrets` |
| `api.env.LOG_LEVEL` | Logging level | `info` |
| `api.env.LOG_FORMAT` | Log format (json/text) | `json` |
| `api.env.SERVER_PORT` | Server port | `8080` |
| `api.env.DATABASE_PATH` | SQLite database path | `/data/agentbox.db` |
| `api.persistence.enabled` | Enable persistent storage for SQLite | `true` |
| `api.persistence.storageClass` | Storage class for PVC | `""` (default) |
| `api.persistence.accessMode` | PVC access mode | `ReadWriteOnce` |
| `api.persistence.size` | PVC size | `1Gi` |
| `api.nodeSelector` | Node selector for API pods | `{}` |
| `api.tolerations` | Tolerations for API pods | `[]` |
| `api.affinity` | Affinity rules for API pods | `{}` |

### UI Frontend Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `ui.enabled` | Enable UI frontend deployment | `true` |
| `ui.replicaCount` | Number of UI replicas | `1` |
| `ui.image.repository` | UI image repository | `agentbox/ui` |
| `ui.image.tag` | UI image tag | `latest` |
| `ui.image.pullPolicy` | UI image pull policy | `IfNotPresent` |
| `ui.service.type` | UI service type | `ClusterIP` |
| `ui.service.port` | UI service port | `80` |
| `ui.resources.limits.cpu` | UI CPU limit | `200m` |
| `ui.resources.limits.memory` | UI memory limit | `256Mi` |
| `ui.resources.requests.cpu` | UI CPU request | `50m` |
| `ui.resources.requests.memory` | UI memory request | `64Mi` |
| `ui.env.API_URL` | Backend API URL | `http://agentbox-api:8080` |
| `ui.nodeSelector` | Node selector for UI pods | `{}` |
| `ui.tolerations` | Tolerations for UI pods | `[]` |
| `ui.affinity` | Affinity rules for UI pods | `{}` |

### UI Ingress Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `ui.ingress.enabled` | Enable UI-specific ingress | `false` |
| `ui.ingress.className` | Ingress class name | `nginx` |
| `ui.ingress.annotations` | Ingress annotations | `{}` |
| `ui.ingress.hosts` | Ingress hosts configuration | See below |
| `ui.ingress.tls` | TLS configuration | `[]` |

Default UI ingress host:
```yaml
ui:
  ingress:
    hosts:
      - host: ui.agentbox.local
        paths:
          - path: /
            pathType: Prefix
```

### Combined Ingress Configuration

For deployments where both UI and API share the same domain:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `ingress.enabled` | Enable combined ingress | `false` |
| `ingress.className` | Ingress class name | `nginx` |
| `ingress.annotations` | Ingress annotations | `{}` |
| `ingress.hosts` | Ingress hosts with path-based routing | See below |
| `ingress.tls` | TLS configuration | `[]` |

Default combined ingress configuration:
```yaml
ingress:
  hosts:
    - host: agentbox.local
      paths:
        - path: /
          pathType: Prefix
          service: ui
        - path: /api
          pathType: Prefix
          service: api
```

### Secrets Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `secrets.create` | Create secrets automatically | `true` |
| `secrets.jwtSecret` | JWT signing secret | `change-me-in-production` |
| `secrets.googleClientId` | Google OAuth client ID | `""` |
| `secrets.googleClientSecret` | Google OAuth client secret | `""` |
| `secrets.databasePassword` | Database password (PostgreSQL) | `""` |

### ServiceAccount and RBAC

| Parameter | Description | Default |
|-----------|-------------|---------|
| `serviceAccount.create` | Create service account | `true` |
| `serviceAccount.annotations` | Service account annotations | `{}` |
| `serviceAccount.name` | Service account name | `""` (auto-generated) |
| `rbac.create` | Create RBAC resources | `true` |

## Example Configurations

### Development Configuration

```yaml
# dev-values.yaml
api:
  replicaCount: 1
  image:
    repository: agentbox/api
    tag: dev
    pullPolicy: Always
  env:
    LOG_LEVEL: debug
  persistence:
    enabled: true
    size: 1Gi

ui:
  replicaCount: 1
  image:
    repository: agentbox/ui
    tag: dev
    pullPolicy: Always

secrets:
  create: true
  jwtSecret: "dev-secret-change-in-prod"
```

### Production Configuration

```yaml
# production-values.yaml
api:
  replicaCount: 3
  image:
    repository: your-registry/agentbox-api
    tag: "1.0.0"
    pullPolicy: Always
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
  env:
    LOG_LEVEL: info

ui:
  replicaCount: 2
  image:
    repository: your-registry/agentbox-ui
    tag: "1.0.0"
    pullPolicy: Always
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

secrets:
  create: false  # Use externally managed secrets
```

### UI-Only Deployment

If you only want to deploy the UI (e.g., connecting to an external API):

```yaml
api:
  enabled: false

ui:
  enabled: true
  env:
    API_URL: https://api.agentbox.example.com
  ingress:
    enabled: true
    hosts:
      - host: ui.agentbox.example.com
        paths:
          - path: /
            pathType: Prefix
```

Install with:
```bash
helm install agentbox ./helm/agentbox -f production-values.yaml
```

## RBAC

The chart creates a ClusterRole and ClusterRoleBinding with permissions to:
- Create and manage namespaces
- Create and manage pods
- Create and manage resource quotas
- Create and manage network policies
- Read node information

These permissions are required for AgentBox to manage isolated execution environments.

## Security

### Running as Non-Root

Both containers run as non-root users by default:
- API runs as user ID 1000
- UI (nginx) runs as user ID 101

### Secrets Management

For production, use Kubernetes secrets or external secret management:

```yaml
# Using the built-in secret
secrets:
  create: true
  jwtSecret: "your-secure-jwt-secret"
  googleClientId: "your-google-client-id"
  googleClientSecret: "your-google-client-secret"
```

Or reference external secrets:
```yaml
secrets:
  create: false

api:
  envFrom:
    secretRef: my-external-secret
```

## Monitoring

### Health Checks

The API service includes health check endpoints:
- Liveness: `/api/v1/health`
- Readiness: `/api/v1/health`

The UI service serves static files and nginx handles health checks internally.

### Accessing the Services

```bash
# Port forward API
kubectl port-forward svc/agentbox-api 8080:8080

# Port forward UI
kubectl port-forward svc/agentbox-ui 3000:80

# Test API health endpoint
curl http://localhost:8080/api/v1/health

# Access UI
open http://localhost:3000
```

## Troubleshooting

### Check Pod Status

```bash
# Check all AgentBox pods
kubectl get pods -l app.kubernetes.io/instance=agentbox

# Check API pods
kubectl get pods -l app.kubernetes.io/component=api

# Check UI pods
kubectl get pods -l app.kubernetes.io/component=ui
```

### View Logs

```bash
# API logs
kubectl logs -l app.kubernetes.io/component=api

# UI logs
kubectl logs -l app.kubernetes.io/component=ui
```

### Check Services

```bash
kubectl get svc -l app.kubernetes.io/instance=agentbox
```

### Check Ingress

```bash
kubectl get ingress -l app.kubernetes.io/instance=agentbox
kubectl describe ingress agentbox
```

### Check RBAC

```bash
kubectl get clusterrole -l app.kubernetes.io/instance=agentbox
kubectl get clusterrolebinding -l app.kubernetes.io/instance=agentbox
```

## Uninstallation

```bash
helm uninstall agentbox
```

Note: This will NOT delete:
- Namespaces and resources created by AgentBox for sandbox environments
- PersistentVolumeClaims (if `api.persistence.enabled: true`)

Clean up manually if needed:

```bash
# Delete sandbox namespaces
kubectl get namespaces | grep agentbox-
kubectl delete namespace <namespace-name>

# Delete PVC
kubectl delete pvc -l app.kubernetes.io/instance=agentbox
```

## Values Reference

See `values.yaml` for all available configuration options with detailed comments.

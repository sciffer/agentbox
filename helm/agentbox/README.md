# AgentBox Helm Chart

This Helm chart deploys the AgentBox service to a Kubernetes cluster.

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
# Build Docker image
docker build -t agentbox:1.0.0 .

# Load into cluster (for local testing)
kind load docker-image agentbox:1.0.0  # For kind
# OR
minikube image load agentbox:1.0.0     # For minikube

# Install chart
helm install agentbox ./helm/agentbox \
  --set image.repository=agentbox \
  --set image.tag=1.0.0
```

## Configuration

### Key Configuration Options

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Image repository | `agentbox` |
| `image.tag` | Image tag | `1.0.0` |
| `replicaCount` | Number of replicas | `1` |
| `service.type` | Service type | `ClusterIP` |
| `config.kubernetes.runtime_class` | Runtime class for pods | `gvisor` |
| `config.auth.enabled` | Enable authentication | `false` |
| `rbac.create` | Create RBAC resources | `true` |

### Example: Production Configuration

```yaml
# production-values.yaml
replicaCount: 3

image:
  repository: your-registry/agentbox
  tag: "1.0.0"
  pullPolicy: Always

service:
  type: LoadBalancer

config:
  auth:
    enabled: true
  server:
    log_level: "info"

resources:
  limits:
    cpu: 1000m
    memory: 1Gi
  requests:
    cpu: 500m
    memory: 512Mi

autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10

ingress:
  enabled: true
  className: "nginx"
  hosts:
    - host: agentbox.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: agentbox-tls
      hosts:
        - agentbox.example.com
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

The container runs as user ID 1000 (non-root) by default. This is configured via:
- `podSecurityContext.runAsUser: 1000`
- `securityContext.runAsNonRoot: true`

### Secrets Management

For production, use Kubernetes secrets for sensitive data:

```yaml
secrets:
  authSecret: "your-auth-secret-here"
```

Or use external secret management:
```yaml
env:
  - name: AGENTBOX_AUTH_SECRET
    valueFrom:
      secretKeyRef:
        name: agentbox-secrets
        key: auth-secret
```

## Ingress

To enable ingress:

```yaml
ingress:
  enabled: true
  className: "nginx"  # or your ingress controller
  hosts:
    - host: agentbox.example.com
      paths:
        - path: /
          pathType: Prefix
```

## Monitoring

### Health Checks

The service includes health check endpoints:
- Liveness: `/api/v1/health`
- Readiness: `/api/v1/health`

These are configured as probes in the deployment.

### ServiceMonitor (Prometheus)

To enable Prometheus scraping:

```yaml
serviceMonitor:
  enabled: true
  interval: 30s
```

## Troubleshooting

### Check Pod Status

```bash
kubectl get pods -l app.kubernetes.io/name=agentbox
```

### View Logs

```bash
kubectl logs -l app.kubernetes.io/name=agentbox
```

### Check RBAC

```bash
kubectl get clusterrole agentbox
kubectl get clusterrolebinding agentbox
```

### Test API

```bash
# Port forward
kubectl port-forward svc/agentbox 8080:8080

# Test health endpoint
curl http://localhost:8080/api/v1/health
```

## Uninstallation

```bash
helm uninstall agentbox
```

Note: This will NOT delete the namespaces and resources created by AgentBox. You may need to clean those up manually:

```bash
kubectl get namespaces | grep agentbox-
kubectl delete namespace <namespace-name>
```

## Values Reference

See `values.yaml` for all available configuration options.

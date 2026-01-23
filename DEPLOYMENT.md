# AgentBox Deployment Guide

This guide covers deploying AgentBox using Docker and Helm.

## Prerequisites

- Docker 20.10+
- Kubernetes 1.19+
- Helm 3.0+
- kubectl configured to access your cluster
- (Optional) gVisor runtime class installed

## Building the Docker Image

### Build Locally

```bash
# Build the image
docker build -t agentbox:1.0.0 .

# Test the image locally
docker run -p 8080:8080 agentbox:1.0.0
```

### Build for Different Platforms

```bash
# Build for AMD64
docker buildx build --platform linux/amd64 -t agentbox:1.0.0 .

# Build for ARM64
docker buildx build --platform linux/arm64 -t agentbox:1.0.0 .

# Build multi-arch
docker buildx build --platform linux/amd64,linux/arm64 -t agentbox:1.0.0 .
```

### Push to Registry

```bash
# Tag for your registry
docker tag agentbox:1.0.0 your-registry/agentbox:1.0.0

# Push
docker push your-registry/agentbox:1.0.0
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

#### With Custom Image

```bash
helm install agentbox ./helm/agentbox \
  --set image.repository=your-registry/agentbox \
  --set image.tag=1.0.0 \
  --set image.pullPolicy=Always
```

#### With Custom Configuration

```bash
# Create values file
cat > my-values.yaml <<EOF
config:
  auth:
    enabled: true
  server:
    log_level: "debug"

resources:
  limits:
    cpu: 1000m
    memory: 1Gi
EOF

# Install
helm install agentbox ./helm/agentbox -f my-values.yaml
```

### 4. Verify Installation

```bash
# Check pods
kubectl get pods -l app.kubernetes.io/name=agentbox

# Check service
kubectl get svc agentbox

# Check logs
kubectl logs -l app.kubernetes.io/name=agentbox

# Test API
kubectl port-forward svc/agentbox 8080:8080
curl http://localhost:8080/api/v1/health
```

## Production Deployment

### Recommended Production Values

```yaml
# production-values.yaml
replicaCount: 3

image:
  repository: your-registry/agentbox
  tag: "1.0.0"
  pullPolicy: Always

service:
  type: LoadBalancer  # or use Ingress

config:
  auth:
    enabled: true
  server:
    log_level: "info"
  kubernetes:
    runtime_class: "gvisor"

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
  targetCPUUtilizationPercentage: 80

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

secrets:
  authSecret: "your-secure-auth-secret-here"
```

### Deploy

```bash
helm install agentbox ./helm/agentbox -f production-values.yaml
```

## Upgrading

```bash
# Update image
helm upgrade agentbox ./helm/agentbox \
  --set image.tag=1.0.1 \
  --reuse-values

# Update with new values
helm upgrade agentbox ./helm/agentbox -f new-values.yaml
```

## Rolling Back

```bash
# List releases
helm list

# Rollback to previous version
helm rollback agentbox

# Rollback to specific revision
helm rollback agentbox 1
```

## Uninstalling

```bash
# Uninstall Helm release
helm uninstall agentbox

# Clean up created namespaces (optional)
kubectl get namespaces | grep agentbox- | \
  awk '{print $1}' | xargs kubectl delete namespace
```

## Troubleshooting

### Pod Not Starting

```bash
# Check pod status
kubectl describe pod <pod-name>

# Check events
kubectl get events --sort-by='.lastTimestamp'

# Check logs
kubectl logs <pod-name>
```

### RBAC Issues

```bash
# Check service account
kubectl get serviceaccount agentbox

# Check cluster role
kubectl get clusterrole agentbox

# Check cluster role binding
kubectl get clusterrolebinding agentbox

# Test permissions
kubectl auth can-i create namespaces \
  --as=system:serviceaccount:default:agentbox
```

### Image Pull Errors

```bash
# Check image pull secrets
kubectl get secrets

# For private registries, create secret:
kubectl create secret docker-registry regcred \
  --docker-server=your-registry \
  --docker-username=your-username \
  --docker-password=your-password

# Add to values.yaml:
imagePullSecrets:
  - name: regcred
```

### Health Check Failures

```bash
# Test health endpoint manually
kubectl exec -it <pod-name> -- wget -qO- http://localhost:8080/api/v1/health

# Check if service is binding correctly
kubectl exec -it <pod-name> -- netstat -tlnp
```

## Monitoring

### View Metrics (if Prometheus is installed)

```bash
# Port forward Prometheus
kubectl port-forward svc/prometheus 9090:9090

# Access Prometheus UI
open http://localhost:9090
```

### Check Resource Usage

```bash
# View pod resource usage
kubectl top pods -l app.kubernetes.io/name=agentbox

# View node resource usage
kubectl top nodes
```

## Security Best Practices

1. **Enable Authentication**
   ```yaml
   config:
     auth:
       enabled: true
   secrets:
     authSecret: "strong-random-secret"
   ```

2. **Use Network Policies**
   - Restrict ingress/egress traffic
   - Limit pod-to-pod communication

3. **Use Secrets Management**
   - Use external secret management (e.g., Sealed Secrets, Vault)
   - Never commit secrets to git

4. **Enable Pod Security Policies**
   - Run as non-root (already configured)
   - Drop unnecessary capabilities
   - Use read-only root filesystem where possible

5. **Regular Updates**
   - Keep base images updated
   - Scan images for vulnerabilities
   - Update Helm chart regularly

## Next Steps

- Configure authentication
- Set up monitoring and alerting
- Configure backup for state (if using persistent storage)
- Set up CI/CD pipeline
- Configure ingress with TLS

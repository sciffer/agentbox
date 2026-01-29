# AgentBox - Isolated Execution Orchestration Platform

## Overview

AgentBox is a lightweight, scalable execution orchestration platform designed for running AI agent workloads in isolated environments. It provides fast startup times, strong isolation guarantees, and simple infrastructure requirements.

## Architecture

### System Components

```
┌─────────────────────────────────────────────────────────────┐
│                         API Server                          │
│  (REST API + WebSocket Proxy for interactive access)       │
└──────────────────┬──────────────────────────────────────────┘
                   │
┌──────────────────┴──────────────────────────────────────────┐
│                      Orchestrator                           │
│  (Lifecycle Management + Scheduling + State Management)     │
└──────────────────┬──────────────────────────────────────────┘
                   │
┌──────────────────┴──────────────────────────────────────────┐
│                   Kubernetes Cluster                        │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐           │
│  │ Namespace  │  │ Namespace  │  │ Namespace  │  ...      │
│  │  Agent-1   │  │  Agent-2   │  │  Agent-N   │           │
│  │ ┌────────┐ │  │ ┌────────┐ │  │ ┌────────┐ │           │
│  │ │  Pod   │ │  │ │  Pod   │ │  │ │  Pod   │ │           │
│  │ │(gVisor)│ │  │ │(gVisor)│ │  │ │(gVisor)│ │           │
│  │ └────────┘ │  │ └────────┘ │  │ └────────┘ │           │
│  └────────────┘  └────────────┘  └────────────┘           │
└─────────────────────────────────────────────────────────────┘
```

### Security Model

Each execution environment is isolated via:
- **Kubernetes Namespace** - Logical isolation
- **Network Policies** - Traffic isolation between environments
- **gVisor Runtime** - Syscall-level isolation
- **Resource Quotas** - CPU/Memory limits per environment
- **RBAC** - Fine-grained access control
- **Pod Security Standards** - Restricted mode enforcement

## API Specification

### Base URL
```
http://localhost:8080/api/v1
```

### Authentication

The API supports two authentication methods:

**1. JWT Token (via Authorization header):**
```
Authorization: Bearer <jwt-token>
```

Obtain a JWT token by logging in via `POST /api/v1/auth/login`. Tokens expire based on `AGENTBOX_JWT_EXPIRY` (default: 15m).

**2. API Key (via X-API-Key header or Authorization header):**
```
X-API-Key: ak_live_your-api-key
```
or
```
Authorization: Bearer ak_live_your-api-key
```

Create API keys via `POST /api/v1/api-keys`. API keys can be set to expire or never expire.

**User Roles:**
- `super_admin` - Full access to all features
- `admin` - Can manage users and all resources
- `user` - Standard user access

### Endpoints

#### Authentication Endpoints

##### Login

**POST** `/auth/login`

Authenticates a user and returns a JWT token.

**Request Body:**
```json
{
  "username": "admin",
  "password": "your-password"
}
```

**Response:** `200 OK`
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user": {
    "id": "user-123",
    "username": "admin",
    "email": "admin@example.com",
    "role": "super_admin",
    "status": "active",
    "created_at": "2026-01-22T10:00:00Z"
  },
  "expires_at": "2026-01-22T11:00:00Z"
}
```

##### Logout

**POST** `/auth/logout`

Logs out the current user (client should discard the token).

**Response:** `204 No Content`

##### Get Current User

**GET** `/auth/me`

Returns the currently authenticated user's information.

**Headers:**
```
Authorization: Bearer <token>
```

**Response:** `200 OK`
```json
{
  "id": "user-123",
  "username": "admin",
  "email": "admin@example.com",
  "role": "super_admin",
  "status": "active",
  "created_at": "2026-01-22T10:00:00Z",
  "last_login": "2026-01-22T10:30:00Z"
}
```

##### Change Password

**POST** `/auth/change-password`

Changes the current user's password.

**Headers:**
```
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "current_password": "old-password",
  "new_password": "new-password-min-8-chars"
}
```

**Response:** `200 OK`

#### User Management Endpoints (Admin Only)

##### List Users

**GET** `/users`

Lists all users (admin only).

**Query Parameters:**
- `limit` - Max results (default: 100)
- `offset` - Pagination offset (default: 0)

**Response:** `200 OK`
```json
{
  "users": [
    {
      "id": "user-123",
      "username": "admin",
      "email": "admin@example.com",
      "role": "super_admin",
      "status": "active"
    }
  ],
  "total": 1
}
```

##### Create User

**POST** `/users`

Creates a new user (admin only).

**Request Body:**
```json
{
  "username": "newuser",
  "email": "user@example.com",
  "password": "password123",
  "role": "user",
  "status": "active"
}
```

**Response:** `201 Created`

##### Get User

**GET** `/users/{id}`

Gets a user by ID. Users can view their own profile; admins can view any user.

**Response:** `200 OK`

#### API Key Management Endpoints

##### List API Keys

**GET** `/api-keys`

Lists API keys for the current user.

**Response:** `200 OK`
```json
{
  "api_keys": [
    {
      "id": "key-123",
      "key_prefix": "ak_live_",
      "description": "My API key",
      "created_at": "2026-01-22T10:00:00Z",
      "expires_at": "2026-02-22T10:00:00Z",
      "last_used": "2026-01-22T10:30:00Z"
    }
  ]
}
```

##### Create API Key

**POST** `/api-keys`

Creates a new API key for the current user.

**Request Body:**
```json
{
  "description": "My API key",
  "expires_in": 30
}
```

**Response:** `201 Created`
```json
{
  "id": "key-123",
  "key": "ak_live_abc123...",
  "key_prefix": "ak_live_",
  "description": "My API key",
  "created_at": "2026-01-22T10:00:00Z",
  "expires_at": "2026-02-22T10:00:00Z"
}
```

**Note:** The full `key` is only returned once on creation. Store it securely.

##### Revoke API Key

**DELETE** `/api-keys/{id}`

Revokes an API key.

**Response:** `204 No Content`

#### Environment Endpoints

##### 1. Create Environment

**POST** `/environments`

Creates a new isolated execution environment.

**Request Body:**
```json
{
  "name": "agent-task-123",
  "image": "python:3.11-slim",
  "resources": {
    "cpu": "500m",
    "memory": "512Mi",
    "storage": "1Gi"
  },
  "timeout": 3600,
  "env": {
    "API_KEY": "secret",
    "TASK_ID": "123"
  },
  "command": ["/bin/bash"],
  "labels": {
    "team": "ai-research",
    "project": "agent-framework"
  },
  "node_selector": {
    "kubernetes.io/arch": "amd64",
    "node-type": "compute"
  },
  "tolerations": [
    {
      "key": "dedicated",
      "operator": "Equal",
      "value": "agents",
      "effect": "NoSchedule"
    }
  ],
  "isolation": {
    "runtime_class": "gvisor",
    "network_policy": {
      "allow_internet": false,
      "allowed_egress_cidrs": ["10.0.0.0/8"],
      "allowed_ingress_ports": [8080],
      "allow_cluster_internal": false
    },
    "security_context": {
      "run_as_non_root": true,
      "read_only_root_filesystem": true,
      "allow_privilege_escalation": false
    }
  }
}
```

**Request Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Name of the environment (lowercase alphanumeric with hyphens, max 63 chars) |
| `image` | string | Yes | Container image to use |
| `resources` | object | Yes | Resource limits (cpu, memory, storage) |
| `timeout` | int | No | Max runtime in seconds (default: 3600) |
| `env` | object | No | Environment variables to set |
| `command` | array | No | Command to run (default: sleep infinity) |
| `labels` | object | No | Labels to apply to resources |
| `node_selector` | object | No | Kubernetes node selector for pod scheduling |
| `tolerations` | array | No | Kubernetes tolerations for scheduling on tainted nodes |
| `isolation` | object | No | Isolation and security settings |

**Toleration Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `key` | string | Taint key to match |
| `operator` | string | "Equal" or "Exists" |
| `value` | string | Taint value to match (only for "Equal" operator) |
| `effect` | string | "NoSchedule", "PreferNoSchedule", or "NoExecute" |
| `tolerationSeconds` | int | Seconds to tolerate (only for "NoExecute") |

**Isolation Configuration:**

| Field | Type | Description |
|-------|------|-------------|
| `runtime_class` | string | Container runtime class (e.g., "gvisor", "kata", "runc"). Empty uses cluster default |
| `network_policy` | object | Network isolation settings (see below) |
| `security_context` | object | Pod security settings (see below) |

**Network Policy Fields:**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `allow_internet` | bool | false | Allow full outbound internet access |
| `allowed_egress_cidrs` | array | [] | List of allowed outbound IP ranges (e.g., ["10.0.0.0/8"]) |
| `allowed_ingress_ports` | array | [] | List of ports to allow inbound traffic (e.g., [8080, 443]) |
| `allow_cluster_internal` | bool | false | Allow traffic to/from other pods in the cluster |

**Security Context Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `run_as_user` | int | UID to run the container as |
| `run_as_group` | int | GID to run the container as |
| `run_as_non_root` | bool | Enforce running as non-root user |
| `read_only_root_filesystem` | bool | Mount root filesystem as read-only |
| `allow_privilege_escalation` | bool | Allow processes to gain more privileges |

**Response:** `201 Created`
```json
{
  "id": "env-a1b2c3d4",
  "name": "agent-task-123",
  "status": "pending",
  "created_at": "2026-01-22T10:30:00Z",
  "endpoint": "wss://agentbox.example.com/environments/env-a1b2c3d4/attach",
  "namespace": "agentbox-env-a1b2c3d4"
}
```

#### 2. Get Environment

**GET** `/environments/{id}`

Retrieves environment details and current status.

**Response:** `200 OK`
```json
{
  "id": "env-a1b2c3d4",
  "name": "agent-task-123",
  "status": "running",
  "image": "python:3.11-slim",
  "created_at": "2026-01-22T10:30:00Z",
  "started_at": "2026-01-22T10:30:05Z",
  "resources": {
    "cpu": "500m",
    "memory": "512Mi",
    "storage": "1Gi"
  },
  "endpoint": "wss://agentbox.example.com/environments/env-a1b2c3d4/attach",
  "namespace": "agentbox-env-a1b2c3d4",
  "metrics": {
    "cpu_usage": "120m",
    "memory_usage": "256Mi"
  }
}
```

**Status Values:**
- `pending` - Environment is being created
- `running` - Environment is active and ready
- `terminating` - Environment is shutting down
- `terminated` - Environment has been cleaned up
- `failed` - Environment failed to start

Environment responses may include reconciliation fields: `reconciliation_retry_count`, `last_reconciliation_error`, `last_reconciliation_at`, `reconciliation_retries_left` (for pending/failed environments and the "Retry" button).

#### 3. Update Environment (PATCH)

**PATCH** `/environments/{id}`

Updates environment settings after creation. All request body fields are optional; only provided fields are updated. Requires editor or higher permission (super admins, environment admins, environment owners).

**Request Body (all optional):** `name`, `image`, `resources`, `timeout`, `env`, `command`, `labels`, `node_selector`, `tolerations`, `isolation`, `pool`

**Response:** `200 OK` with the updated environment.

#### 4. Retry Reconciliation

**POST** `/environments/{id}/retry`

Resets reconciliation retry count and triggers one provisioning attempt. Use when an environment is stuck in pending/failed after max automatic retries. Requires editor or higher permission.

**Response:** `202 Accepted` with `{ "status": "retry_triggered" }`.

#### 5. List Environments

**GET** `/environments`

Lists all environments with optional filtering.

**Query Parameters:**
- `status` - Filter by status (e.g., `?status=running`)
- `label` - Filter by label (e.g., `?label=team=ai-research`)
- `limit` - Max results (default: 100)
- `offset` - Pagination offset (default: 0)

**Response:** `200 OK`
```json
{
  "environments": [
    {
      "id": "env-a1b2c3d4",
      "name": "agent-task-123",
      "status": "running",
      "created_at": "2026-01-22T10:30:00Z"
    }
  ],
  "total": 1,
  "limit": 100,
  "offset": 0
}
```

#### 6. Execute Command

**POST** `/environments/{id}/exec`

Executes a command in the environment and returns output.

**Request Body:**
```json
{
  "command": ["python", "-c", "print('hello')"],
  "timeout": 30
}
```

**Response:** `200 OK`
```json
{
  "stdout": "hello\n",
  "stderr": "",
  "exit_code": 0,
  "duration_ms": 145
}
```

#### 7. Attach to Environment (WebSocket)

**WebSocket** `/environments/{id}/attach`

Opens an interactive WebSocket connection for real-time I/O.

**Connection:**
```javascript
const ws = new WebSocket('wss://agentbox.example.com/environments/env-a1b2c3d4/attach');

// Send input
ws.send(JSON.stringify({
  type: "stdin",
  data: "ls -la\n"
}));

// Receive output
ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  // msg.type: "stdout" | "stderr" | "exit"
  // msg.data: output content
};
```

**Message Format:**

Client → Server:
```json
{
  "type": "stdin",
  "data": "command input"
}
```

Server → Client:
```json
{
  "type": "stdout",
  "data": "command output",
  "timestamp": "2026-01-22T10:30:10Z"
}
```

#### 8. Delete Environment

**DELETE** `/environments/{id}`

Terminates and removes an environment.

**Query Parameters:**
- `force` - Force immediate termination (default: false)

**Response:** `204 No Content`

#### 9. Get Environment Logs

**GET** `/environments/{id}/logs`

Retrieves logs from the environment. Includes both pod logs and reconciliation events (reconciliation loop start/success/failure), merged and sorted by time.

**Query Parameters:**
- `tail` - Number of lines from end (e.g., `?tail=100`)
- `follow` - Stream logs (boolean, default: false)
- `timestamps` - Include timestamps (boolean, default: true)

**Response:** `200 OK`
```json
{
  "logs": [
    {
      "timestamp": "2026-01-22T10:30:10Z",
      "stream": "stdout",
      "message": "Application started"
    }
  ]
}
```

#### 8. Health Check

**GET** `/health`

System health status.

**Response:** `200 OK`
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "kubernetes": {
    "connected": true,
    "version": "1.28.0"
  },
  "capacity": {
    "total_nodes": 10,
    "available_cpu": "50000m",
    "available_memory": "100Gi"
  }
}
```

## Configuration

### Environment Variables

**Server Configuration:**
```bash
AGENTBOX_HOST=0.0.0.0              # Server bind address
AGENTBOX_PORT=8080                  # Server port
AGENTBOX_LOG_LEVEL=info             # Log level: debug, info, warn, error
```

**Database Configuration:**
```bash
AGENTBOX_DB_PATH=/data/agentbox.db  # SQLite database path
AGENTBOX_DB_DSN=                    # PostgreSQL connection string (overrides DB_PATH)
```

**Authentication (Required):**
```bash
AGENTBOX_JWT_SECRET=your-secret     # JWT signing secret (min 32 chars)
AGENTBOX_AUTH_SECRET=your-secret    # Config validation secret (same as JWT_SECRET)
AGENTBOX_AUTH_ENABLED=true          # Enable/disable authentication
AGENTBOX_JWT_EXPIRY=24h             # JWT token expiry duration
AGENTBOX_API_KEY_PREFIX=ak_         # Prefix for generated API keys
```

**Admin Credentials:**
```bash
AGENTBOX_ADMIN_USERNAME=admin       # Initial admin username
AGENTBOX_ADMIN_PASSWORD=            # Initial admin password (auto-generated if empty)
AGENTBOX_ADMIN_EMAIL=               # Initial admin email
```

**Kubernetes Configuration:**
```bash
AGENTBOX_KUBECONFIG=                # Path to kubeconfig (empty = in-cluster)
AGENTBOX_NAMESPACE_PREFIX=agentbox- # Prefix for sandbox namespaces
AGENTBOX_RUNTIME_CLASS=gvisor       # RuntimeClass for sandboxes (optional)
```

**Resource Limits (defaults for sandboxes):**
```bash
AGENTBOX_DEFAULT_CPU_LIMIT=1000m    # Default CPU limit
AGENTBOX_DEFAULT_MEMORY_LIMIT=512Mi # Default memory limit
AGENTBOX_DEFAULT_STORAGE_LIMIT=1Gi  # Default storage limit
AGENTBOX_MAX_ENVIRONMENTS_PER_USER=10 # Max sandboxes per user
```

**Timeouts (in seconds):**
```bash
AGENTBOX_DEFAULT_TIMEOUT=3600       # Default sandbox timeout (1 hour)
AGENTBOX_MAX_TIMEOUT=86400          # Maximum sandbox timeout (24 hours)
AGENTBOX_STARTUP_TIMEOUT=300        # Sandbox startup timeout (5 minutes)
```

**Metrics:**
```bash
AGENTBOX_METRICS_ENABLED=true       # Enable metrics collection
AGENTBOX_METRICS_COLLECTION_INTERVAL=30s # Collection interval
```

**Google OAuth (Optional):**
```bash
AGENTBOX_GOOGLE_CLIENT_ID=          # Google OAuth client ID
AGENTBOX_GOOGLE_CLIENT_SECRET=      # Google OAuth client secret
```

### Kubernetes Requirements

**Minimum Version:** 1.25+

**Required Features:**
- RuntimeClass support
- NetworkPolicy support
- ResourceQuota support
- RBAC enabled

**Recommended:**
- gVisor RuntimeClass installed
- Metrics Server for resource monitoring
- CNI plugin with NetworkPolicy support (Calico, Cilium, etc.)

## Resource Model

### Default Resources per Environment

```yaml
resources:
  requests:
    cpu: 100m
    memory: 128Mi
    ephemeral-storage: 500Mi
  limits:
    cpu: 1000m
    memory: 1Gi
    ephemeral-storage: 5Gi
```

### Scaling Considerations

- **Per Node:** ~100-200 environments (depends on resources)
- **Cluster-wide:** 10,000+ environments across multiple nodes
- **Startup Time:** 2-5 seconds per environment
- **Network Overhead:** ~1MB per active WebSocket connection

## Security Best Practices

1. **Image Validation:** Use only trusted, scanned container images
2. **Network Isolation:** Apply strict NetworkPolicies by default
3. **Resource Limits:** Always enforce CPU/memory limits
4. **Secret Management:** Use Kubernetes Secrets, never plain env vars
5. **Runtime Security:** Enable gVisor for syscall filtering
6. **API Authentication:** Always use token-based auth in production
7. **Audit Logging:** Enable audit logs for compliance

## Error Codes

| Code | Description |
|------|-------------|
| 400 | Invalid request parameters |
| 401 | Authentication required |
| 403 | Insufficient permissions |
| 404 | Environment not found |
| 409 | Environment already exists |
| 429 | Rate limit exceeded |
| 500 | Internal server error |
| 503 | Service unavailable (k8s connectivity) |

## Monitoring & Observability

### Metrics (Prometheus format)

```
agentbox_environments_total{status="running|terminated|failed"}
agentbox_environment_creation_duration_seconds
agentbox_environment_lifetime_seconds
agentbox_api_request_duration_seconds{endpoint,method,status}
agentbox_websocket_connections_active
agentbox_kubernetes_api_calls_total{operation,status}
```

### Logging

Structured JSON logs with fields:
- `timestamp`
- `level` (debug, info, warn, error)
- `environment_id`
- `user_id`
- `operation`
- `message`
- `duration_ms`

## Web UI

AgentBox includes a web-based management UI built with React + TypeScript.

### Features

- Dashboard with metrics and charts
- Environment management (create, view, delete)
- Interactive terminal (WebSocket)
- Log streaming (SSE)
- User management (admin)
- API key management
- Settings

### Running the UI

```bash
# Development
cd ui
npm install
npm run dev
# UI available at http://localhost:3000

# Production (Docker)
docker build -t agentbox-ui:latest ./ui
docker run -p 3000:3000 agentbox-ui:latest
```

### Ports

| Service | Port | Description |
|---------|------|-------------|
| Backend API | 8080 | REST API + WebSocket |
| UI (development) | 5173 | Vite dev server |
| UI (production) | 8080 | Nginx (Docker container) |

## Development

### Building from Source

```bash
# Clone repository
git clone https://github.com/yourorg/agentbox.git
cd agentbox

# Build backend
go build -o agentbox ./cmd/server

# Build UI
cd ui && npm install && npm run build

# Run tests
go test ./...

# Run locally
./agentbox --config config.yaml
```

### Project Structure

```
agentbox/
├── cmd/
│   └── server/          # Main entry point
├── pkg/
│   ├── api/             # HTTP handlers
│   ├── orchestrator/    # K8s orchestration logic
│   ├── auth/            # Authentication
│   ├── proxy/           # WebSocket proxy
│   └── models/          # Data models
├── internal/
│   └── k8s/             # Kubernetes client wrapper
├── tests/
│   ├── unit/            # Unit tests
│   └── integration/     # Integration tests
├── docs/                # Documentation
├── deploy/              # Kubernetes manifests
└── go.mod
```

## Deployment

### Docker

```bash
# Build and run backend
docker build -t agentbox:latest .
docker run -p 8080:8080 \
  -v ~/.kube/config:/kubeconfig \
  -e AGENTBOX_KUBECONFIG=/kubeconfig \
  -e AGENTBOX_JWT_SECRET="your-secret-key-min-32-chars" \
  agentbox:latest

# Build and run UI
docker build -t agentbox-ui:latest ./ui
docker run -p 3000:8080 \
  -e VITE_API_URL=http://localhost:8080 \
  agentbox-ui:latest
```

### Docker Compose

```bash
docker-compose up -d
```

### Kubernetes (Helm)

```bash
# Install with Helm
helm install agentbox ./helm/agentbox

# With custom values
helm install agentbox ./helm/agentbox \
  --set ui.enabled=true \
  --set ui.ingress.enabled=true \
  --set ui.ingress.hosts[0].host=ui.example.com

# Upgrade
helm upgrade agentbox ./helm/agentbox
```

### Kubernetes (Manual)

```bash
kubectl apply -f deploy/namespace.yaml
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/deployment.yaml
kubectl apply -f deploy/service.yaml
```

## License

MIT License - see LICENSE file for details
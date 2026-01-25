# AgentBox API Usage Guide

This guide covers how to use the AgentBox API to manage isolated execution environments, execute commands, retrieve logs, and more.

## Base URL

All API endpoints are prefixed with `/api/v1`.

```
https://your-agentbox-server.com/api/v1
```

## Authentication

AgentBox supports two authentication methods:

1. **JWT Tokens** - Obtained by logging in with username/password
2. **API Keys** - Long-lived tokens for programmatic access

### Login with Username/Password

```bash
curl -X POST https://your-server/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "your-username",
    "password": "your-password"
  }'
```

**Response:**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": "2026-01-23T12:00:00Z",
  "user": {
    "id": "user-123",
    "username": "your-username",
    "email": "user@example.com",
    "role": "user"
  }
}
```

### Using the Token

Include the JWT token in the `Authorization` header for all protected endpoints:

```bash
curl -X GET https://your-server/api/v1/environments \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

### Using API Keys

API keys can be used instead of JWT tokens by including them in the `Authorization` header:

```bash
curl -X GET https://your-server/api/v1/environments \
  -H "Authorization: Bearer agentbox_1234567890abcdef..."
```

### Get Current User

```bash
curl -X GET https://your-server/api/v1/auth/me \
  -H "Authorization: Bearer <token>"
```

---

## Environment Management

### Create an Environment

Create a new isolated execution environment.

```bash
curl -X POST https://your-server/api/v1/environments \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-python-env",
    "image": "python:3.11-slim",
    "resources": {
      "cpu": "500m",
      "memory": "512Mi",
      "storage": "1Gi"
    },
    "timeout": 3600,
    "env": {
      "MY_VAR": "my-value"
    },
    "labels": {
      "project": "my-project"
    }
  }'
```

**Request Body Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Name of the environment |
| `image` | string | Yes | Container image to use |
| `resources.cpu` | string | Yes | CPU limit (e.g., "500m", "1") |
| `resources.memory` | string | Yes | Memory limit (e.g., "512Mi", "1Gi") |
| `resources.storage` | string | Yes | Storage limit (e.g., "1Gi", "5Gi") |
| `timeout` | int | No | Lifetime in seconds (default: 3600) |
| `env` | object | No | Environment variables |
| `command` | string[] | No | Custom command to run |
| `labels` | object | No | Labels for filtering |
| `node_selector` | object | No | Kubernetes node selector |
| `tolerations` | array | No | Kubernetes tolerations |
| `isolation` | object | No | Isolation settings (see below) |

**Isolation Settings:**

```json
{
  "isolation": {
    "runtime_class": "gvisor",
    "network_policy": {
      "allow_internet": false,
      "allowed_egress_cidrs": ["10.0.0.0/8"],
      "allowed_ingress_ports": [8080],
      "allow_cluster_internal": false
    },
    "security_context": {
      "run_as_user": 1000,
      "run_as_group": 1000,
      "run_as_non_root": true,
      "read_only_root_filesystem": false,
      "allow_privilege_escalation": false
    }
  }
}
```

**Response:**

```json
{
  "id": "env-abc123",
  "name": "my-python-env",
  "status": "pending",
  "image": "python:3.11-slim",
  "created_at": "2026-01-22T10:00:00Z",
  "resources": {
    "cpu": "500m",
    "memory": "512Mi",
    "storage": "1Gi"
  },
  "endpoint": "env-abc123.agentbox.svc.cluster.local",
  "namespace": "agentbox-env-abc123"
}
```

### List Environments

```bash
curl -X GET "https://your-server/api/v1/environments?status=running&limit=10&offset=0" \
  -H "Authorization: Bearer <token>"
```

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `status` | string | Filter by status: pending, running, terminating, terminated, failed |
| `label` | string | Filter by label selector (e.g., "project=my-project") |
| `limit` | int | Max results to return (default: 100) |
| `offset` | int | Pagination offset (default: 0) |

**Response:**

```json
{
  "environments": [
    {
      "id": "env-abc123",
      "name": "my-python-env",
      "status": "running",
      "image": "python:3.11-slim",
      "created_at": "2026-01-22T10:00:00Z",
      "started_at": "2026-01-22T10:00:15Z",
      "resources": {
        "cpu": "500m",
        "memory": "512Mi",
        "storage": "1Gi"
      },
      "endpoint": "env-abc123.agentbox.svc.cluster.local",
      "namespace": "agentbox-env-abc123"
    }
  ],
  "total": 1,
  "limit": 10,
  "offset": 0
}
```

### Get Environment Details

```bash
curl -X GET https://your-server/api/v1/environments/env-abc123 \
  -H "Authorization: Bearer <token>"
```

**Response:**

```json
{
  "id": "env-abc123",
  "name": "my-python-env",
  "status": "running",
  "image": "python:3.11-slim",
  "created_at": "2026-01-22T10:00:00Z",
  "started_at": "2026-01-22T10:00:15Z",
  "resources": {
    "cpu": "500m",
    "memory": "512Mi",
    "storage": "1Gi"
  },
  "metrics": {
    "cpu_usage": "125m",
    "memory_usage": "256Mi"
  },
  "endpoint": "env-abc123.agentbox.svc.cluster.local",
  "namespace": "agentbox-env-abc123",
  "env": {
    "MY_VAR": "my-value"
  },
  "labels": {
    "project": "my-project"
  }
}
```

### Delete an Environment

```bash
curl -X DELETE "https://your-server/api/v1/environments/env-abc123?force=false" \
  -H "Authorization: Bearer <token>"
```

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `force` | boolean | Force delete even if environment is not gracefully stopping |

**Response:** `204 No Content`

---

## Command Execution

### Execute a Command

Execute a command in a running environment and receive the output.

```bash
curl -X POST https://your-server/api/v1/environments/env-abc123/exec \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "command": ["python", "-c", "print(\"Hello, World!\")"],
    "timeout": 30
  }'
```

**Request Body:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `command` | string[] | Yes | Command and arguments to execute |
| `timeout` | int | No | Timeout in seconds (default: 30) |

**Response:**

```json
{
  "stdout": "Hello, World!\n",
  "stderr": "",
  "exit_code": 0,
  "duration_ms": 125
}
```

### Execute Complex Commands

**Run a shell script:**

```bash
curl -X POST https://your-server/api/v1/environments/env-abc123/exec \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "command": ["sh", "-c", "echo $PATH && ls -la /tmp"]
  }'
```

**Install packages and run code:**

```bash
curl -X POST https://your-server/api/v1/environments/env-abc123/exec \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "command": ["sh", "-c", "pip install requests && python -c \"import requests; print(requests.__version__)\""],
    "timeout": 120
  }'
```

**Write and execute a file:**

```bash
# First, write the file
curl -X POST https://your-server/api/v1/environments/env-abc123/exec \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "command": ["sh", "-c", "cat > /tmp/script.py << EOF\nimport sys\nprint(f\"Python {sys.version}\")\nEOF"]
  }'

# Then execute it
curl -X POST https://your-server/api/v1/environments/env-abc123/exec \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "command": ["python", "/tmp/script.py"]
  }'
```

---

## Logs

### Get Environment Logs

Retrieve logs from an environment.

```bash
curl -X GET "https://your-server/api/v1/environments/env-abc123/logs?tail=100&timestamps=true" \
  -H "Authorization: Bearer <token>"
```

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `tail` | int | Number of lines from the end (default: all) |
| `timestamps` | boolean | Include timestamps (default: true) |
| `follow` | boolean | Stream logs in real-time (default: false) |

**Response (non-streaming):**

```json
{
  "logs": [
    {
      "timestamp": "2026-01-22T10:00:15Z",
      "stream": "stdout",
      "message": "Starting application..."
    },
    {
      "timestamp": "2026-01-22T10:00:16Z",
      "stream": "stdout",
      "message": "Ready to accept connections"
    }
  ]
}
```

### Stream Logs (Server-Sent Events)

Stream logs in real-time using Server-Sent Events (SSE).

```bash
curl -N -X GET "https://your-server/api/v1/environments/env-abc123/logs?follow=true&tail=10" \
  -H "Authorization: Bearer <token>"
```

**SSE Response:**

```
data: {"timestamp":"2026-01-22T10:00:15Z","stream":"stdout","message":"Line 1"}

data: {"timestamp":"2026-01-22T10:00:16Z","stream":"stdout","message":"Line 2"}

data: {"timestamp":"2026-01-22T10:00:17Z","stream":"stderr","message":"Warning: something happened"}
```

**JavaScript Example:**

```javascript
const eventSource = new EventSource(
  'https://your-server/api/v1/environments/env-abc123/logs?follow=true',
  {
    headers: {
      'Authorization': 'Bearer <token>'
    }
  }
);

eventSource.onmessage = (event) => {
  const log = JSON.parse(event.data);
  console.log(`[${log.stream}] ${log.message}`);
};

eventSource.onerror = (error) => {
  console.error('SSE error:', error);
  eventSource.close();
};
```

---

## WebSocket Attachment

Attach to an environment's terminal via WebSocket for interactive access.

### Connect via WebSocket

```javascript
const ws = new WebSocket(
  'wss://your-server/api/v1/environments/env-abc123/attach',
  ['Authorization', 'Bearer <token>']
);

ws.onopen = () => {
  console.log('Connected to environment');
  // Send input
  ws.send(JSON.stringify({
    type: 'stdin',
    data: 'ls -la\n'
  }));
};

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  switch (msg.type) {
    case 'stdout':
      process.stdout.write(msg.data);
      break;
    case 'stderr':
      process.stderr.write(msg.data);
      break;
    case 'exit':
      console.log('Process exited with code:', msg.exit_code);
      break;
  }
};

ws.onerror = (error) => {
  console.error('WebSocket error:', error);
};
```

**Message Types:**

| Type | Direction | Description |
|------|-----------|-------------|
| `stdin` | Client → Server | Send input to the environment |
| `stdout` | Server → Client | Standard output from the environment |
| `stderr` | Server → Client | Standard error from the environment |
| `exit` | Server → Client | Process exit notification |

---

## API Key Management

### List Your API Keys

```bash
curl -X GET https://your-server/api/v1/api-keys \
  -H "Authorization: Bearer <token>"
```

**Response:**

```json
{
  "api_keys": [
    {
      "id": "key-abc123",
      "description": "CI/CD Pipeline Key",
      "created_at": "2026-01-20T10:00:00Z",
      "expires_at": "2026-04-20T10:00:00Z",
      "last_used_at": "2026-01-22T09:30:00Z"
    }
  ]
}
```

### Create an API Key

```bash
curl -X POST https://your-server/api/v1/api-keys \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "description": "CI/CD Pipeline Key",
    "expires_in": 90,
    "permissions": [
      {
        "environment_id": "env-abc123",
        "permission": "write"
      }
    ]
  }'
```

**Request Body:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `description` | string | No | Description of the key's purpose |
| `expires_in` | int | No | Days until expiration (null = never) |
| `permissions` | array | No | Environment-specific permissions |

**Permission Levels:**
- `read` - Can view environment and logs
- `write` - Can execute commands
- `admin` - Can delete environment

**Response:**

```json
{
  "id": "key-xyz789",
  "key": "agentbox_abc123def456...",
  "description": "CI/CD Pipeline Key",
  "created_at": "2026-01-22T10:00:00Z",
  "expires_at": "2026-04-22T10:00:00Z",
  "permissions": [
    {
      "environment_id": "env-abc123",
      "permission": "write"
    }
  ]
}
```

> **Important:** The `key` field is only returned once at creation time. Store it securely!

### Revoke an API Key

```bash
curl -X DELETE https://your-server/api/v1/api-keys/key-abc123 \
  -H "Authorization: Bearer <token>"
```

**Response:** `204 No Content`

---

## Health Check

Check the API server and Kubernetes cluster status.

```bash
curl -X GET https://your-server/api/v1/health
```

**Response:**

```json
{
  "status": "healthy",
  "version": "1.0.0",
  "kubernetes": {
    "connected": true,
    "version": "v1.28.0"
  },
  "capacity": {
    "total_nodes": 3,
    "available_cpu": "24",
    "available_memory": "48Gi"
  }
}
```

---

## Error Handling

All errors follow a consistent format:

```json
{
  "error": "error type",
  "message": "detailed error message",
  "code": 400
}
```

**Common HTTP Status Codes:**

| Code | Description |
|------|-------------|
| 200 | Success |
| 201 | Created |
| 204 | No Content (successful deletion) |
| 400 | Bad Request - Invalid input |
| 401 | Unauthorized - Missing or invalid token |
| 403 | Forbidden - Insufficient permissions |
| 404 | Not Found - Resource doesn't exist |
| 500 | Internal Server Error |
| 503 | Service Unavailable - Cluster unhealthy |

---

## Complete Workflow Example

Here's a complete example of creating an environment, executing code, getting logs, and cleaning up:

```bash
#!/bin/bash
SERVER="https://your-server/api/v1"
TOKEN="your-jwt-token"

# 1. Create an environment
echo "Creating environment..."
ENV_RESPONSE=$(curl -s -X POST "$SERVER/environments" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "python-sandbox",
    "image": "python:3.11-slim",
    "resources": {
      "cpu": "500m",
      "memory": "512Mi",
      "storage": "1Gi"
    },
    "timeout": 1800
  }')

ENV_ID=$(echo $ENV_RESPONSE | jq -r '.id')
echo "Created environment: $ENV_ID"

# 2. Wait for environment to be ready
echo "Waiting for environment to be ready..."
while true; do
  STATUS=$(curl -s -X GET "$SERVER/environments/$ENV_ID" \
    -H "Authorization: Bearer $TOKEN" | jq -r '.status')
  
  if [ "$STATUS" = "running" ]; then
    echo "Environment is running!"
    break
  elif [ "$STATUS" = "failed" ]; then
    echo "Environment failed to start"
    exit 1
  fi
  
  echo "Status: $STATUS, waiting..."
  sleep 2
done

# 3. Execute some commands
echo "Installing dependencies..."
curl -s -X POST "$SERVER/environments/$ENV_ID/exec" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "command": ["pip", "install", "requests", "numpy"],
    "timeout": 120
  }' | jq

echo "Running Python code..."
curl -s -X POST "$SERVER/environments/$ENV_ID/exec" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "command": ["python", "-c", "import numpy as np; print(f\"NumPy version: {np.__version__}\"); arr = np.array([1,2,3,4,5]); print(f\"Array mean: {arr.mean()}\")"]
  }' | jq

# 4. Get logs
echo "Fetching logs..."
curl -s -X GET "$SERVER/environments/$ENV_ID/logs?tail=50" \
  -H "Authorization: Bearer $TOKEN" | jq

# 5. Delete environment
echo "Cleaning up..."
curl -s -X DELETE "$SERVER/environments/$ENV_ID" \
  -H "Authorization: Bearer $TOKEN"

echo "Done!"
```

---

## Rate Limits

Default rate limits (configurable per deployment):

| Endpoint Category | Limit |
|-------------------|-------|
| Authentication | 10 requests/minute |
| Environment Creation | 20 requests/minute |
| Command Execution | 100 requests/minute |
| Other Operations | 200 requests/minute |

Rate limit headers are included in responses:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1706011200
```

---

## Best Practices

1. **Use API Keys for automation** - Create dedicated API keys for CI/CD pipelines with limited permissions.

2. **Set appropriate timeouts** - Use realistic timeouts for command execution to avoid hanging requests.

3. **Clean up environments** - Delete environments when done to free cluster resources.

4. **Use labels for organization** - Label environments with project names, users, or purposes for easy filtering.

5. **Monitor environment status** - Poll the environment status after creation before executing commands.

6. **Handle WebSocket disconnections** - Implement reconnection logic for long-running WebSocket sessions.

7. **Limit resource requests** - Request only the resources your workload needs to maximize cluster utilization.

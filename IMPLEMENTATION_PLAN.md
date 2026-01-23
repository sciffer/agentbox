# AgentBox Implementation Plan

## Status: Compilation Errors Fixed ✅

All compilation errors have been resolved. The codebase now builds successfully.

## Phase 1: Critical Fixes (COMPLETED ✅)

### ✅ Fixed Logger Calls
- Updated all logger calls to use `zap.Field` values (`zap.String()`, `zap.Error()`, etc.)
- Fixed in: `pkg/proxy/proxy.go`, `pkg/orchestrator/orchestrator.go`, `pkg/api/handler.go`, `pkg/api/websocket.go`, `cmd/server/main.go`

### ✅ Fixed Router Signature
- Updated `NewRouter()` to accept `proxyHandler` parameter
- Wired WebSocket attachment endpoint: `/api/v1/environments/{id}/attach`

### ✅ Fixed Type Conversions
- Fixed `PodPhase` to string conversion in orchestrator

---

## Phase 2: Missing Core Features (HIGH PRIORITY)

### 1. Logs Endpoint Implementation
**Priority:** High  
**Status:** Not Started  
**Estimated Effort:** 2-3 hours

**Requirements:**
- Implement `GET /api/v1/environments/{id}/logs`
- Support query parameters:
  - `tail` - Number of lines from end (default: all)
  - `follow` - Stream logs (boolean, default: false)
  - `timestamps` - Include timestamps (boolean, default: true)
- Use `k8sClient.GetPodLogs()` for log retrieval
- Return `LogsResponse` with array of `LogEntry`

**Implementation Steps:**
1. Add `GetLogs` handler method in `pkg/api/handler.go`
2. Add route in `pkg/api/router.go`
3. Implement log streaming for `follow=true` (optional, can be v2)
4. Add tests in `tests/unit/api_test.go`

**Files to Modify:**
- `pkg/api/handler.go` - Add `GetLogs` method
- `pkg/api/router.go` - Add route
- `pkg/k8s/pod.go` - Already has `GetPodLogs()` method

---

### 2. Health Check Implementation
**Priority:** High  
**Status:** Partially Implemented (hardcoded values)  
**Estimated Effort:** 1-2 hours

**Requirements:**
- Query Kubernetes API for real cluster status
- Get actual Kubernetes version
- Calculate cluster capacity (nodes, CPU, memory)
- Return proper health status based on connectivity

**Implementation Steps:**
1. Create `GetClusterCapacity()` method in `pkg/k8s/client.go`
2. Update `HealthCheck` handler to use real data
3. Handle errors gracefully (return unhealthy if k8s unavailable)
4. Add caching to avoid excessive API calls (optional)

**Files to Modify:**
- `pkg/api/handler.go` - Update `HealthCheck` method
- `pkg/k8s/client.go` - Add `GetClusterCapacity()` method

**Kubernetes API Calls Needed:**
- `clientset.CoreV1().Nodes().List()` - Get node information
- `clientset.Discovery().ServerVersion()` - Get version (already exists)

---

### 3. Label Selector Implementation
**Priority:** Medium  
**Status:** Stub (always returns true)  
**Estimated Effort:** 1 hour

**Requirements:**
- Implement proper Kubernetes label selector parsing
- Support formats: `key=value`, `key!=value`, `key in (value1,value2)`
- Use Kubernetes label selector library or implement basic parsing

**Implementation Steps:**
1. Use `k8s.io/apimachinery/pkg/labels` package
2. Parse selector string into `labels.Selector`
3. Use `selector.Matches()` to check labels
4. Update `matchesLabelSelector()` in `pkg/orchestrator/orchestrator.go`

**Files to Modify:**
- `pkg/orchestrator/orchestrator.go` - Implement `matchesLabelSelector()`

**Example:**
```go
import "k8s.io/apimachinery/pkg/labels"

func matchesLabelSelector(envLabels map[string]string, selectorStr string) bool {
    selector, err := labels.Parse(selectorStr)
    if err != nil {
        return false
    }
    return selector.Matches(labels.Set(envLabels))
}
```

---

## Phase 3: Authentication & Security (MEDIUM PRIORITY)

### 4. Authentication Middleware
**Priority:** Medium  
**Status:** Stub (returns "anonymous")  
**Estimated Effort:** 4-6 hours

**Requirements:**
- Implement JWT or API key authentication
- Extract user ID from request context
- Add authentication middleware to router
- Support Bearer token format: `Authorization: Bearer <token>`

**Implementation Options:**

**Option A: JWT Authentication**
- Use `github.com/golang-jwt/jwt/v5`
- Validate JWT tokens
- Extract user ID from token claims
- Support token refresh (optional)

**Option B: API Key Authentication**
- Simple API key validation
- Store keys in config or environment variables
- Map keys to user IDs

**Implementation Steps:**
1. Create `pkg/auth/auth.go` with authentication logic
2. Create middleware function
3. Add middleware to router (except `/health`)
4. Update `getUserIDFromContext()` in `pkg/api/handler.go`
5. Add tests

**Files to Create:**
- `pkg/auth/auth.go` - Authentication logic
- `pkg/auth/middleware.go` - HTTP middleware

**Files to Modify:**
- `pkg/api/router.go` - Add auth middleware
- `pkg/api/handler.go` - Update `getUserIDFromContext()`

---

### 5. WebSocket Origin Checking
**Priority:** Low (Security)  
**Status:** TODO comment  
**Estimated Effort:** 1 hour

**Requirements:**
- Implement proper origin validation for WebSocket connections
- Check against allowed origins from config
- Prevent CSRF attacks

**Implementation Steps:**
1. Add `allowed_origins` to config
2. Update `upgrader.CheckOrigin` in `pkg/proxy/proxy.go`
3. Validate origin against config

**Files to Modify:**
- `internal/config/config.go` - Add `AllowedOrigins` field
- `config/config.yaml` - Add allowed origins
- `pkg/proxy/proxy.go` - Implement origin checking

---

## Phase 4: Observability & Monitoring (MEDIUM PRIORITY)

### 6. Prometheus Metrics
**Priority:** Medium  
**Status:** Not Implemented  
**Estimated Effort:** 3-4 hours

**Requirements:**
- Expose metrics at `/metrics` endpoint
- Track:
  - `agentbox_environments_total{status}`
  - `agentbox_environment_creation_duration_seconds`
  - `agentbox_api_request_duration_seconds{endpoint,method,status}`
  - `agentbox_websocket_connections_active`
  - `agentbox_kubernetes_api_calls_total{operation,status}`

**Implementation Steps:**
1. Add `github.com/prometheus/client_golang` dependency
2. Create `pkg/metrics/metrics.go`
3. Add metrics collection throughout codebase
4. Add `/metrics` endpoint to router
5. Add middleware for HTTP request metrics

**Files to Create:**
- `pkg/metrics/metrics.go` - Metrics definitions and collection

**Files to Modify:**
- `pkg/api/router.go` - Add `/metrics` endpoint
- `pkg/orchestrator/orchestrator.go` - Add metrics
- `pkg/api/handler.go` - Add request duration metrics
- `pkg/proxy/proxy.go` - Add WebSocket connection metrics

---

### 7. Enhanced Logging
**Priority:** Low  
**Status:** Basic implementation exists  
**Estimated Effort:** 1-2 hours

**Requirements:**
- Add request ID tracking
- Add correlation IDs for tracing
- Improve log context in handlers

**Implementation Steps:**
1. Add request ID middleware
2. Add correlation ID to logger context
3. Update handlers to use request ID

---

## Phase 5: State Management (LOW PRIORITY)

### 8. Persistent State Storage
**Priority:** Low  
**Status:** In-memory only  
**Estimated Effort:** 8-12 hours

**Requirements:**
- Store environment state in database or etcd
- Persist across restarts
- Support state recovery

**Implementation Options:**

**Option A: SQL Database (PostgreSQL)**
- Use `database/sql` or GORM
- Store environment metadata
- Support queries and filtering

**Option B: etcd**
- Use `go.etcd.io/etcd/client/v3`
- Store as key-value
- Support watch for changes

**Option C: Kubernetes CRDs**
- Create custom resources
- Use Kubernetes as source of truth
- No additional infrastructure

**Recommendation:** Start with Option C (CRDs) as it's most aligned with the architecture.

---

## Phase 6: Deployment & Operations (MEDIUM PRIORITY)

### 9. Dockerfile
**Priority:** Medium  
**Status:** Not Created  
**Estimated Effort:** 1 hour

**Requirements:**
- Multi-stage build
- Small image size
- Proper user permissions
- Health check

**Files to Create:**
- `Dockerfile`
- `.dockerignore`

---

### 10. Kubernetes Manifests
**Priority:** Medium  
**Status:** Not Created  
**Estimated Effort:** 2-3 hours

**Requirements:**
- Deployment manifest
- Service manifest
- RBAC (ServiceAccount, Role, RoleBinding)
- ConfigMap for configuration
- Optional: Ingress

**Files to Create:**
- `deploy/kubernetes/namespace.yaml`
- `deploy/kubernetes/rbac.yaml`
- `deploy/kubernetes/deployment.yaml`
- `deploy/kubernetes/service.yaml`
- `deploy/kubernetes/configmap.yaml`
- `deploy/kubernetes/kustomization.yaml`

---

### 11. Helm Chart
**Priority:** Low  
**Status:** Not Created  
**Estimated Effort:** 4-6 hours

**Requirements:**
- Helm chart structure
- Configurable values
- Support for different environments

**Files to Create:**
- `charts/agentbox/Chart.yaml`
- `charts/agentbox/values.yaml`
- `charts/agentbox/templates/*.yaml`

---

## Phase 7: Testing & Quality (ONGOING)

### 12. Test Coverage Improvements
**Priority:** Medium  
**Status:** Tests exist but may need updates  
**Estimated Effort:** 4-6 hours

**Requirements:**
- Update existing tests after fixes
- Add tests for new features
- Maintain 85%+ coverage
- Add integration tests for WebSocket

**Files to Review/Update:**
- `tests/unit/api_test.go` - Add logs endpoint tests
- `tests/unit/orchestrator_test.go` - Add label selector tests
- `tests/integration/lifecycle_test.go` - Add WebSocket tests

---

### 13. CI/CD Pipeline
**Priority:** Medium  
**Status:** Not Created  
**Estimated Effort:** 2-3 hours

**Requirements:**
- GitHub Actions workflow
- Run tests on PR
- Build and push Docker image
- Deploy to staging (optional)

**Files to Create:**
- `.github/workflows/ci.yml`
- `.github/workflows/cd.yml` (optional)

---

## Phase 8: Documentation (ONGOING)

### 14. API Documentation
**Priority:** Low  
**Status:** README has basic docs  
**Estimated Effort:** 2-3 hours

**Requirements:**
- OpenAPI/Swagger specification
- Interactive API docs
- Example requests/responses

**Implementation:**
- Use `github.com/swaggo/swag` for Swagger
- Or create OpenAPI spec manually

---

### 15. Architecture Documentation
**Priority:** Low  
**Status:** Basic architecture in README  
**Estimated Effort:** 2-3 hours

**Requirements:**
- Detailed architecture diagrams
- Sequence diagrams for key flows
- Deployment architecture

---

## Recommended Implementation Order

### Sprint 1 (Week 1): Core Features
1. ✅ Fix compilation errors (DONE)
2. Implement logs endpoint
3. Implement health check
4. Implement label selector

### Sprint 2 (Week 2): Security & Auth
5. Implement authentication middleware
6. WebSocket origin checking
7. Security review

### Sprint 3 (Week 3): Observability
8. Prometheus metrics
9. Enhanced logging
10. Monitoring dashboard setup (optional)

### Sprint 4 (Week 4): Deployment
11. Dockerfile
12. Kubernetes manifests
13. CI/CD pipeline

### Sprint 5 (Week 5+): Polish
14. State persistence (if needed)
15. Helm chart
16. Documentation improvements
17. Performance optimization

---

## Questions to Resolve

1. **Authentication Method:** JWT or API keys? (Recommendation: Start with API keys, add JWT later)
2. **State Persistence:** Is in-memory acceptable for MVP, or need persistence? (Recommendation: Start in-memory, add persistence if needed)
3. **Metrics:** Is Prometheus required for MVP? (Recommendation: Yes, but can be simplified)
4. **Deployment Target:** Where will this run? (Affects manifests and config)

---

## Success Criteria

- [x] Code compiles without errors
- [ ] All API endpoints implemented and tested
- [ ] Authentication working
- [ ] Health check returns real data
- [ ] Logs endpoint functional
- [ ] Metrics exposed
- [ ] Docker image builds
- [ ] Kubernetes deployment works
- [ ] 85%+ test coverage maintained
- [ ] Documentation complete

---

## Notes

- All TODOs in code have been documented here
- Priority levels are relative and can be adjusted based on requirements
- Estimated effort is for a single developer
- Some features can be simplified for MVP and enhanced later

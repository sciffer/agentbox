# AgentBox UI Implementation Plan

## Overview

This document outlines the plan for implementing a web-based UI for AgentBox that provides visual management of environments, sandboxes, users, API keys, and system configuration.

## Architecture

### Technology Stack

**Frontend:**
- **Framework:** React 18+ with TypeScript
- **UI Library:** Material-UI (MUI) v5 or Ant Design
- **State Management:** React Query (TanStack Query) for server state, Zustand/Context API for UI state
- **Routing:** React Router v6
- **HTTP Client:** Axios with interceptors for auth
- **Form Handling:** React Hook Form with Zod validation
- **Real-time Updates:** WebSocket client for live environment status, SSE for log streaming
- **Build Tool:** Vite
- **Styling:** CSS Modules or Styled Components

**Backend Integration:**
- REST API: Existing `/api/v1/*` endpoints
- WebSocket: Existing `/api/v1/environments/{id}/attach` for terminal
- SSE: Existing `/api/v1/environments/{id}/logs?follow=true` for logs
- New Auth Endpoints: `/api/v1/auth/*` (to be implemented)

### Project Structure

```
agentbox-ui/
├── src/
│   ├── components/          # Reusable UI components
│   │   ├── common/          # Buttons, inputs, modals, etc.
│   │   ├── layout/          # Header, sidebar, footer
│   │   └── charts/          # Data visualization
│   ├── pages/              # Page components
│   │   ├── Dashboard.tsx
│   │   ├── Environments.tsx
│   │   ├── EnvironmentDetail.tsx
│   │   ├── Users.tsx
│   │   ├── APIKeys.tsx
│   │   ├── Settings.tsx
│   │   └── Login.tsx
│   ├── features/           # Feature-based modules
│   │   ├── auth/
│   │   │   ├── components/
│   │   │   ├── hooks/
│   │   │   ├── services/
│   │   │   └── types.ts
│   │   ├── environments/
│   │   ├── users/
│   │   └── apiKeys/
│   ├── hooks/              # Custom React hooks
│   ├── services/           # API service layer
│   ├── store/             # State management
│   ├── utils/             # Helper functions
│   ├── types/             # TypeScript types
│   └── App.tsx
├── public/
└── package.json
```

## Authentication System

### Authentication Methods

1. **Local Authentication (Username/Password)**
   - Default admin user: `admin` / configurable via `AGENTBOX_ADMIN_PASSWORD` env var
   - Password hashing: bcrypt with salt rounds
   - JWT tokens for session management
   - Refresh tokens for long-lived sessions

2. **API Key Authentication**
   - API keys stored in database (hashed)
   - Clients authenticate using `Authorization: Bearer <api-key>` header
   - API keys can be scoped to specific users
   - Usage tracking and rate limiting per API key
   - API keys can be used for programmatic access (CLI, scripts, etc.)

3. **Google OAuth 2.0**
   - Optional integration via environment variables (not K8s secrets)
   - OAuth credentials: `AGENTBOX_GOOGLE_CLIENT_ID`, `AGENTBOX_GOOGLE_CLIENT_SECRET`
   - OAuth flow: Authorization Code with PKCE
   - User linking: Google account → AgentBox user
   - Auto-provisioning: Create user on first Google login (if enabled)

### Authentication Flow

```
1. User visits /login
2. Selects auth method (Local or Google)
3. For Local:
   - Enter username/password
   - Backend validates and returns JWT
4. For Google:
   - Redirect to Google OAuth
   - Google redirects back with code
   - Backend exchanges code for user info
   - Backend creates/links user and returns JWT
5. Store JWT in httpOnly cookie or localStorage
6. Redirect to dashboard
```

### Backend Auth Endpoints (to be implemented)

```
POST   /api/v1/auth/login          # Local login (returns JWT)
POST   /api/v1/auth/logout         # Logout (invalidates token)
POST   /api/v1/auth/refresh        # Refresh token
GET    /api/v1/auth/me             # Get current user (JWT or API key)
GET    /api/v1/auth/google/url     # Get Google OAuth URL
POST   /api/v1/auth/google/callback # Handle Google callback
POST   /api/v1/auth/change-password # Change password
```

**Authentication Middleware:**
- Supports both JWT tokens (for UI sessions) and API keys (for programmatic access)
- Checks `Authorization` header: `Bearer <token-or-key>`
- Validates JWT signature and expiration
- Looks up API key in database and validates
- Sets user context for downstream handlers

## User Management System

### User Roles & Permissions

**System Roles:**
1. **Super Admin**
   - Full system access
   - User management
   - System configuration
   - All environment access

2. **Admin**
   - User management (limited)
   - Environment management
   - API key management

3. **User**
   - Own environment management
   - View own API keys
   - Limited system access

4. **Viewer**
   - Read-only access
   - View environments (assigned)
   - No create/modify permissions

**Permission Model:**
- **System-wide permissions:** Role-based (RBAC)
- **Per-environment permissions:** Fine-grained access control
  - Owner: Full control
  - Editor: Create, read, update (no delete)
  - Viewer: Read-only
  - No Access: Cannot see environment

### User Management Features

**Admin Dashboard → Users Page:**
- List all users with filters (role, status, search)
- Create new users
- Edit user details (email, role, status)
- Reset passwords
- Enable/disable users
- View user activity logs
- Assign environment permissions
- Bulk operations (enable/disable multiple users)

**User Profile Page:**
- View own profile
- Change password
- Manage API keys
- View assigned environments
- View activity history

## Dashboard Pages

### 1. Dashboard (Home)

**Overview Cards:**
- Total Environments
- Running Environments
- Total Users
- System Health Status
- Resource Usage (CPU, Memory, Storage)
- Average Sandbox Start Time

**Global Metrics Charts:**
- Total Running Sandboxes Over Time (line chart)
- Active CPU Usage Over Time (area chart)
- Active Memory Usage Over Time (area chart)
- Average Time to Start Sandbox Over Time (line chart)
- Environment Status Distribution (pie chart)
- Environment Creation Over Time (line chart)
- Resource Usage Trends (area chart)
- Active Users Over Time (bar chart)

**Per-Environment Metrics (Selectable):**
- Running Sandboxes Over Time (for selected environment)
- CPU Usage Over Time (for selected environment)
- Memory Usage Over Time (for selected environment)
- Average Start Time Over Time (for selected environment)
- Environment-specific resource trends

**Metrics Time Range Selector:**
- Last hour
- Last 24 hours
- Last 7 days
- Last 30 days
- Custom date range

**Recent Activity:**
- Latest environment creations
- Recent user logins
- System events

**Quick Actions:**
- Create Environment
- View All Environments
- Manage Users

### 2. Environments Page

**Environment List View:**
- Table with columns:
  - Name
  - Status (badge with color)
  - Image
  - Resources (CPU/Memory)
  - Created At
  - Owner
  - Actions (View, Edit, Delete, Terminal, Logs)

**Filters:**
- Status (Pending, Running, Terminated, Failed)
- Owner/User
- Labels
- Date Range

**Actions:**
- Create Environment (modal/form)
- Bulk Delete
- Export to CSV
- Refresh

**Environment Detail View:**
- Overview tab:
  - Environment details
  - Resource usage (real-time metrics)
  - Status history
- Terminal tab:
  - WebSocket terminal integration
  - Full terminal emulator
- Logs tab:
  - Log viewer with streaming
  - Filter by stream (stdout/stderr)
  - Search/filter logs
  - Download logs
- Settings tab:
  - Edit environment config
  - Manage permissions
  - Resource limits
  - Labels

### 3. Users Page

**User List:**
- Table with columns:
  - Username
  - Email
  - Role
  - Status (Active/Inactive)
  - Last Login
  - Created At
  - Actions (Edit, Disable, Delete)

**User Creation/Edit Modal:**
- Username
- Email
- Password (with strength indicator)
- Role selection
- Status toggle
- Environment permissions (multi-select with role per env)

### 4. API Keys Page

**API Key Management:**
- List user's API keys
- Create new API key (with description)
- Revoke API key
- View usage statistics
- Copy key to clipboard
- Show creation date and last used

**Admin View:**
- View all API keys across users
- Filter by user
- Bulk revoke

### 5. Settings Page

**System Settings (Admin only):**
- Kubernetes Configuration
- Default Resource Limits
- Environment Timeouts
- OAuth Configuration:
  - Enable/disable Google OAuth
  - OAuth Client ID/Secret (from K8s secret)
  - Redirect URLs
- Email Configuration (for notifications)
- System Limits:
  - Max environments per user
  - Max concurrent environments
  - Resource quotas

**User Settings:**
- Profile information
- Password change
- Notification preferences
- Theme (light/dark mode)

### 6. Login Page

**Design:**
- Clean, centered login form
- Logo/branding
- Two tabs: "Local Login" and "Google Login"
- Forgot password link
- Remember me checkbox

**Local Login Form:**
- Username input
- Password input (with show/hide toggle)
- Submit button
- Error messages

**Google Login:**
- "Sign in with Google" button
- Redirects to Google OAuth

## UI Components

### Common Components

1. **DataTable**
   - Sorting
   - Filtering
   - Pagination
   - Row selection
   - Actions menu

2. **StatusBadge**
   - Color-coded status indicators
   - Icons for different states

3. **ResourceDisplay**
   - CPU, Memory, Storage visualization
   - Progress bars
   - Usage percentages

4. **TerminalEmulator**
   - xterm.js integration
   - WebSocket connection
   - Copy/paste support
   - Resizable

5. **LogViewer**
   - Virtual scrolling for performance
   - Stream filtering (stdout/stderr)
   - Search functionality
   - Auto-scroll toggle
   - Timestamp display toggle

6. **EnvironmentCard**
   - Environment summary
   - Quick actions
   - Status indicator
   - Resource usage

7. **PermissionMatrix**
   - Visual permission editor
   - Per-environment permissions
   - Role-based presets

## Backend Changes Required

### New API Endpoints

**Authentication:**
```
POST   /api/v1/auth/login
POST   /api/v1/auth/logout
POST   /api/v1/auth/refresh
GET    /api/v1/auth/me
GET    /api/v1/auth/google/url
POST   /api/v1/auth/google/callback
POST   /api/v1/auth/change-password
```

**User Management:**
```
GET    /api/v1/users              # List users (admin)
POST   /api/v1/users              # Create user (admin)
GET    /api/v1/users/{id}         # Get user
PUT    /api/v1/users/{id}         # Update user (admin)
DELETE /api/v1/users/{id}         # Delete user (admin)
POST   /api/v1/users/{id}/reset-password
POST   /api/v1/users/{id}/enable
POST   /api/v1/users/{id}/disable
GET    /api/v1/users/{id}/environments
PUT    /api/v1/users/{id}/permissions
```

**API Keys:**
```
GET    /api/v1/api-keys            # List user's API keys
POST   /api/v1/api-keys            # Create API key
DELETE /api/v1/api-keys/{id}       # Revoke API key
GET    /api/v1/api-keys/{id}/usage # Get usage stats
```

**System:**
```
GET    /api/v1/system/stats        # Dashboard statistics
GET    /api/v1/system/config       # System configuration (admin)
PUT    /api/v1/system/config       # Update config (admin)
GET    /api/v1/system/health       # Enhanced health check
```

**Metrics:**
```
GET    /api/v1/metrics/global      # Global metrics (all environments)
GET    /api/v1/metrics/environment/{id}  # Per-environment metrics
GET    /api/v1/metrics/running-sandboxes?start=...&end=...&env_id=...
GET    /api/v1/metrics/cpu-usage?start=...&end=...&env_id=...
GET    /api/v1/metrics/memory-usage?start=...&end=...&env_id=...
GET    /api/v1/metrics/start-time?start=...&end=...&env_id=...
```

### Database Schema

**Database Support:**
- **Development/Testing:** SQLite (file-based, no server required)
- **Production:** PostgreSQL (external database)
- **Schema Management:** Automatic migration/validation on service startup
- **Migration Tool:** Use `golang-migrate` or similar for schema versioning

**Database Configuration:**
- SQLite: `AGENTBOX_DB_PATH=/path/to/agentbox.db` (default: `./agentbox.db`)
- PostgreSQL: `AGENTBOX_DB_DSN=postgres://user:pass@host:port/dbname?sslmode=disable`
- Auto-detect: If DSN provided, use PostgreSQL; otherwise use SQLite

**Users Table:**
```sql
CREATE TABLE users (
    id TEXT PRIMARY KEY, -- UUID as string for SQLite compatibility
    username VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE,
    password_hash TEXT, -- NULL if OAuth only
    role VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL, -- active, inactive, suspended
    google_id VARCHAR(255) UNIQUE, -- NULL if local only
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_login TIMESTAMP
);

CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_google_id ON users(google_id);
```

**API Keys Table:**
```sql
CREATE TABLE api_keys (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_hash TEXT UNIQUE NOT NULL, -- SHA-256 hash of the API key
    key_prefix TEXT NOT NULL, -- First 8 chars for display (e.g., "ak_live_")
    description TEXT,
    last_used TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP,
    revoked_at TIMESTAMP
);

CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_revoked ON api_keys(revoked_at);
```

**Environment Permissions Table:**
```sql
CREATE TABLE environment_permissions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    environment_id VARCHAR(255) NOT NULL,
    permission VARCHAR(50) NOT NULL, -- owner, editor, viewer
    granted_by TEXT REFERENCES users(id),
    granted_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, environment_id)
);

CREATE INDEX idx_env_perms_user_id ON environment_permissions(user_id);
CREATE INDEX idx_env_perms_env_id ON environment_permissions(environment_id);
```

**Metrics Table (for dashboard charts):**
```sql
CREATE TABLE metrics (
    id TEXT PRIMARY KEY,
    environment_id VARCHAR(255), -- NULL for global metrics
    metric_type VARCHAR(50) NOT NULL, -- running_sandboxes, cpu_usage, memory_usage, start_time
    value REAL NOT NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_metrics_env_id ON metrics(environment_id);
CREATE INDEX idx_metrics_type ON metrics(metric_type);
CREATE INDEX idx_metrics_timestamp ON metrics(timestamp);
CREATE INDEX idx_metrics_env_type_time ON metrics(environment_id, metric_type, timestamp);
```

**Schema Version Table:**
```sql
CREATE TABLE schema_version (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### Environment Variables Configuration

**All settings configurable via environment variables:**

```bash
# Database Configuration
AGENTBOX_DB_DSN=postgres://user:pass@host:port/dbname?sslmode=disable
# OR for SQLite:
AGENTBOX_DB_PATH=/path/to/agentbox.db

# Default Admin User
AGENTBOX_ADMIN_USERNAME=admin
AGENTBOX_ADMIN_PASSWORD=changeme  # Must be changed on first login
AGENTBOX_ADMIN_EMAIL=admin@example.com

# Google OAuth (optional)
AGENTBOX_GOOGLE_CLIENT_ID=your-client-id
AGENTBOX_GOOGLE_CLIENT_SECRET=your-client-secret
AGENTBOX_GOOGLE_REDIRECT_URL=https://your-domain.com/api/v1/auth/google/callback
AGENTBOX_GOOGLE_ENABLED=true

# JWT Configuration
AGENTBOX_JWT_SECRET=your-jwt-secret-key
AGENTBOX_JWT_EXPIRY=15m
AGENTBOX_JWT_REFRESH_EXPIRY=7d

# API Key Configuration
AGENTBOX_API_KEY_PREFIX=ak_live_
AGENTBOX_API_KEY_LENGTH=32

# System Limits
AGENTBOX_MAX_ENVIRONMENTS_PER_USER=10
AGENTBOX_MAX_CONCURRENT_ENVIRONMENTS=100
AGENTBOX_DEFAULT_CPU_LIMIT=1
AGENTBOX_DEFAULT_MEMORY_LIMIT=1Gi
AGENTBOX_DEFAULT_STORAGE_LIMIT=10Gi

# Metrics Collection
AGENTBOX_METRICS_ENABLED=true
AGENTBOX_METRICS_RETENTION_DAYS=30
AGENTBOX_METRICS_COLLECTION_INTERVAL=30s
```

**Benefits:**
- No need to read Kubernetes secrets in application code
- Simple configuration via environment variables
- Works with any deployment method (K8s, Docker, systemd, etc.)
- Secrets can be injected via K8s secrets, ConfigMaps, or external secret managers
- Transparent to the application layer

## Security Considerations

1. **JWT Tokens:**
   - Short-lived access tokens (15 minutes)
   - Long-lived refresh tokens (7 days)
   - Stored in httpOnly cookies (preferred) or secure localStorage
   - Token rotation on refresh

2. **Password Security:**
   - Minimum 8 characters
   - Require complexity (if desired)
   - Bcrypt hashing with salt
   - Password change on first login for admin

3. **API Keys:**
   - SHA-256 hashing
   - Show only on creation
   - Expiration dates
   - Usage tracking

4. **CORS:**
   - Configure allowed origins
   - Credentials support

5. **Rate Limiting:**
   - Login attempts (5 per minute)
   - API requests (per user/IP)

6. **Input Validation:**
   - Sanitize all user inputs
   - Validate on both client and server

## Implementation Phases

### Phase 1: Foundation (Week 1-2)
- [ ] Set up React project with Vite
- [ ] Configure routing and basic layout
- [ ] Implement authentication UI (login page)
- [ ] Create API service layer
- [ ] Set up state management
- [ ] Implement JWT token handling

### Phase 2: Authentication Backend (Week 2-3)
- [ ] Implement local authentication endpoints
- [ ] Add JWT token generation/validation
- [ ] Create user model and storage
- [ ] Implement password hashing
- [ ] Add middleware for protected routes
- [ ] Create default admin user

### Phase 3: Core UI Pages (Week 3-4)
- [ ] Dashboard page with statistics
- [ ] Environments list page
- [ ] Environment detail page
- [ ] Basic CRUD operations
- [ ] Status indicators and badges

### Phase 4: User Management (Week 4-5)
- [ ] Users list page
- [ ] User creation/edit forms
- [ ] Permission management UI
- [ ] User management backend endpoints
- [ ] Role-based access control

### Phase 5: Advanced Features (Week 5-6)
- [ ] Terminal integration (WebSocket)
- [ ] Log streaming (SSE)
- [ ] API key management
- [ ] Settings page
- [ ] Real-time updates

### Phase 6: OAuth Integration (Week 6-7)
- [ ] Google OAuth backend
- [ ] OAuth UI flow
- [ ] User linking logic
- [ ] K8s secret integration
- [ ] Testing and documentation

### Phase 7: Polish & Testing (Week 7-8)
- [ ] Error handling and user feedback
- [ ] Loading states
- [ ] Responsive design
- [ ] Accessibility improvements
- [ ] E2E testing
- [ ] Performance optimization
- [ ] Documentation

## Deployment

### Docker Image
- Multi-stage build
- Nginx for serving static files
- Environment-based configuration

### Kubernetes Deployment
- ConfigMap for UI configuration
- Service for UI
- Ingress for external access
- Integration with existing AgentBox backend

### Environment Variables
```bash
REACT_APP_API_URL=https://api.agentbox.com
REACT_APP_WS_URL=wss://api.agentbox.com
REACT_APP_GOOGLE_CLIENT_ID=your-client-id
REACT_APP_ENABLE_OAUTH=true
```

## Future Enhancements

1. **Multi-factor Authentication (MFA)**
   - TOTP support
   - SMS verification

2. **Audit Logging**
   - User action tracking
   - Environment change history
   - Security event logging

3. **Notifications**
   - Email notifications
   - In-app notifications
   - Webhook support

4. **Advanced Analytics**
   - Usage dashboards
   - Cost tracking
   - Performance metrics

5. **Multi-tenancy**
   - Organization/team support
   - Resource isolation
   - Billing integration

6. **Mobile App**
   - React Native app
   - Push notifications
   - Mobile-optimized views

## Notes

- The UI should be responsive and work on desktop, tablet, and mobile
- Dark mode support should be included
- All API calls should have proper error handling
- Loading states should be shown for async operations
- The UI should gracefully handle backend unavailability
- Internationalization (i18n) can be added in future phases

# AgentBox UI Implementation Plan

## Implementation Status

> **Status: IMPLEMENTED**
> 
> The UI has been implemented as a React + TypeScript application with all core features.

### Completed Features

- [x] React 18 + TypeScript + Vite project setup
- [x] Material-UI component library
- [x] React Router v6 for routing
- [x] React Query for data fetching
- [x] Zustand for state management
- [x] Axios HTTP client with interceptors
- [x] JWT authentication handling
- [x] Login page (local auth)
- [x] Dashboard with metrics charts (Recharts)
- [x] Environments list/detail pages
- [x] Terminal integration (xterm.js + WebSocket)
- [x] Log streaming (SSE)
- [x] User management page
- [x] API key management page
- [x] Settings page
- [x] Docker image with nginx
- [x] Kubernetes deployment via Helm

### Pending Features

- [ ] Google OAuth integration (backend ready, UI flow pending)
- [ ] Dark mode toggle
- [ ] Responsive design improvements
- [ ] E2E testing
- [ ] Internationalization (i18n)

---

## Overview

This document outlines the plan for implementing a web-based UI for AgentBox that provides visual management of environments, sandboxes, users, API keys, and system configuration.

## Architecture

### Technology Stack

**Frontend:**
- **Framework:** React 18+ with TypeScript
- **UI Library:** Material-UI (MUI) v5
- **State Management:** React Query (TanStack Query) for server state, Zustand for UI state
- **Routing:** React Router v6
- **HTTP Client:** Axios with interceptors for auth
- **Form Handling:** React Hook Form with Zod validation
- **Real-time Updates:** WebSocket client for live environment status, SSE for log streaming
- **Build Tool:** Vite
- **Terminal:** xterm.js
- **Charts:** Recharts

**Backend Integration:**
- REST API: Existing `/api/v1/*` endpoints
- WebSocket: Existing `/api/v1/environments/{id}/attach` for terminal
- SSE: Existing `/api/v1/environments/{id}/logs?follow=true` for logs
- Auth Endpoints: `/api/v1/auth/*` (implemented)

### Project Structure

```
ui/
├── src/
│   ├── components/          # Reusable UI components
│   │   ├── common/          # TerminalView, LogViewer
│   │   └── layout/          # Layout component
│   ├── pages/               # Page components
│   │   ├── LoginPage.tsx
│   │   ├── DashboardPage.tsx
│   │   ├── EnvironmentsPage.tsx
│   │   ├── EnvironmentDetailPage.tsx
│   │   ├── UsersPage.tsx
│   │   ├── APIKeysPage.tsx
│   │   └── SettingsPage.tsx
│   ├── services/            # API service layer
│   │   └── api.ts
│   ├── store/               # State management
│   │   └── authStore.ts
│   ├── types/               # TypeScript types
│   └── App.tsx
├── public/
├── Dockerfile
├── nginx.conf
└── package.json
```

## Deployment

### Docker

The UI is packaged as a Docker image with nginx:

```bash
# Build UI Docker image
docker build -t agentbox-ui:latest ./ui

# Run UI container
docker run -p 3000:3000 \
  -e VITE_API_URL=http://backend:8080/api/v1 \
  agentbox-ui:latest
```

### Kubernetes (Helm)

The UI is included in the Helm chart:

```yaml
# values.yaml
ui:
  enabled: true
  replicaCount: 1
  image:
    repository: agentbox-ui
    tag: "1.0.0"
  service:
    type: ClusterIP
    port: 3000
  ingress:
    enabled: true
    hosts:
      - host: ui.agentbox.local
        paths:
          - path: /
            pathType: Prefix
```

Deploy with:

```bash
helm install agentbox ./helm/agentbox
```

### Ports

| Service | Port | Description |
|---------|------|-------------|
| Backend API | 8080 | REST API + WebSocket |
| UI (dev) | 3000 | Vite dev server |
| UI (prod) | 3000 | Nginx |

### Environment Variables

```bash
# UI Configuration
VITE_API_URL=http://localhost:8080/api/v1
VITE_WS_URL=ws://localhost:8080
VITE_GOOGLE_OAUTH_ENABLED=false
```

## Authentication System

### Authentication Methods

1. **Local Authentication (Username/Password)** - Implemented
   - Default admin user: `admin` / configurable via `AGENTBOX_ADMIN_PASSWORD` env var
   - Password hashing: bcrypt with salt rounds
   - JWT tokens for session management

2. **API Key Authentication** - Implemented
   - API keys stored in database (hashed)
   - Clients authenticate using `Authorization: Bearer <api-key>` header

3. **Google OAuth 2.0** - Backend Ready
   - Optional integration via environment variables
   - OAuth credentials: `AGENTBOX_GOOGLE_CLIENT_ID`, `AGENTBOX_GOOGLE_CLIENT_SECRET`

### Backend Auth Endpoints (Implemented)

```
POST   /api/v1/auth/login          # Local login (returns JWT)
POST   /api/v1/auth/logout         # Logout
GET    /api/v1/auth/me             # Get current user
POST   /api/v1/auth/change-password # Change password
```

## User Management System

### User Roles & Permissions

**System Roles:**
1. **Super Admin** - Full system access
2. **Admin** - User management, environment management
3. **User** - Own environment management
4. **Viewer** - Read-only access

## Dashboard Pages

### 1. Dashboard (Home) - Implemented

- Total Environments count
- Running/Pending/Failed counts
- Metrics charts (Running Sandboxes Over Time)
- Quick stats cards

### 2. Environments Page - Implemented

- Environment list with status badges
- Create environment modal
- Delete environment
- View environment details

### 3. Environment Detail Page - Implemented

- Overview tab with details
- Terminal tab (WebSocket)
- Logs tab (SSE streaming)

### 4. Users Page - Implemented

- User list (admin only)
- Create user modal
- Role management

### 5. API Keys Page - Implemented

- List API keys
- Create new API key
- Revoke API key
- Copy key to clipboard

### 6. Settings Page - Implemented

- User profile
- Change password

### 7. Login Page - Implemented

- Local login form
- Google OAuth button (placeholder)

## Development

### Local Development

```bash
cd ui
npm install
npm run dev
```

The UI will be available at http://localhost:3000

### Building for Production

```bash
cd ui
npm run build
```

### Make Commands

```bash
make ui-install  # Install dependencies
make ui-dev      # Start dev server
make ui-build    # Build for production
```

## Future Enhancements

1. **Multi-factor Authentication (MFA)**
2. **Audit Logging**
3. **Notifications** (email, in-app)
4. **Advanced Analytics**
5. **Multi-tenancy**
6. **Mobile App**
7. **Dark Mode**
8. **Internationalization (i18n)**

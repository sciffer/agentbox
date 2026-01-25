# AgentBox UI

React-based web UI for AgentBox sandbox management platform.

## Development

```bash
# Install dependencies
npm install

# Start development server (runs on port 5173)
npm run dev

# Build for production
npm run build

# Preview production build
npm run preview

# Run tests
npm run test

# Run linting
npm run lint

# Type check
npm run typecheck
```

## Configuration

### Development

Set environment variables in `.env` or `.env.local`:

```bash
# Backend API URL (required)
VITE_API_URL=http://localhost:8080

# WebSocket URL (optional, defaults to API URL)
VITE_WS_URL=ws://localhost:8080

# Enable Google OAuth button (optional)
VITE_GOOGLE_OAUTH_ENABLED=false
```

### Production (Docker)

Environment variables for the Docker container:

```bash
# Backend API URL - passed to nginx for proxying
VITE_API_URL=http://agentbox-api:8080

# WebSocket URL (optional)
VITE_WS_URL=ws://agentbox-api:8080

# Enable Google OAuth in UI
VITE_GOOGLE_OAUTH_ENABLED=true
```

## Features

- **Authentication** - JWT-based login with optional Google OAuth
- **Dashboard** - Metrics and charts for system overview
- **Environment Management** - Create, view, delete sandboxes
- **Interactive Terminal** - WebSocket-based terminal access
- **Log Streaming** - Real-time log viewing via SSE
- **User Management** - Admin panel for user CRUD
- **API Key Management** - Generate and manage API keys
- **Settings** - Configuration options

## Docker

### Build

```bash
docker build -t agentbox-ui:latest .
```

### Run

```bash
# Development
docker run -p 3000:8080 \
  -e VITE_API_URL=http://localhost:8080 \
  agentbox-ui:latest

# Production with external API
docker run -p 80:8080 \
  -e VITE_API_URL=https://api.agentbox.example.com \
  -e VITE_GOOGLE_OAUTH_ENABLED=true \
  agentbox-ui:latest
```

## Ports

| Environment | Port | Description |
|-------------|------|-------------|
| Development | 5173 | Vite dev server |
| Production (container) | 8080 | Nginx server |

## Tech Stack

- **Framework**: React 18+ with TypeScript
- **Build Tool**: Vite
- **UI Components**: Material-UI (MUI)
- **State Management**: Zustand (UI state), React Query (server state)
- **HTTP Client**: Axios
- **Terminal**: xterm.js
- **Charts**: Recharts
- **Testing**: Vitest + React Testing Library

## Project Structure

```
ui/
├── src/
│   ├── components/      # Reusable UI components
│   │   ├── common/      # Common components (LogViewer, etc.)
│   │   └── layout/      # Layout components (Header, Sidebar)
│   ├── pages/           # Page components
│   ├── services/        # API client and services
│   ├── store/           # Zustand stores
│   ├── types/           # TypeScript interfaces
│   └── test/            # Test utilities
├── public/              # Static assets
├── nginx.conf           # Nginx configuration
├── Dockerfile           # Production Docker image
└── package.json
```

## API Integration

The UI communicates with the backend API at the URL specified by `VITE_API_URL`:

- **REST API**: `/api/v1/*` - CRUD operations
- **WebSocket**: `/api/v1/environments/{id}/attach` - Terminal access
- **SSE**: `/api/v1/environments/{id}/logs?follow=true` - Log streaming

## Kubernetes Deployment

See the Helm chart in `helm/agentbox/` for Kubernetes deployment:

```bash
helm install agentbox ./helm/agentbox \
  --set ui.enabled=true \
  --set ui.env.API_URL=http://agentbox-api:8080
```

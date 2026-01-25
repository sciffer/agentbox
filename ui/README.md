# AgentBox UI

React-based web UI for AgentBox sandbox management platform.

## Development

```bash
# Install dependencies
npm install

# Start development server (runs on port 3000)
npm run dev

# Build for production
npm run build

# Preview production build
npm run preview
```

## Configuration

Set environment variables in `.env`:

```bash
VITE_API_URL=http://localhost:8080/api/v1
```

## Features

- Authentication (JWT-based)
- Dashboard with metrics
- Environment management
- User management (admin)
- API key management
- Terminal integration (WebSocket)
- Log streaming (SSE)
- Settings page

## Port

The UI runs on **port 3000** by default (development).

In production, it can be served on the same port as the backend (8080) or a separate port.

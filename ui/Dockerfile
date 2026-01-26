# Build stage
FROM node:20-alpine AS builder

# Set working directory
WORKDIR /app

# Configure npm for better network behavior
RUN npm config set fetch-retries 5 && \
    npm config set fetch-retry-mintimeout 20000 && \
    npm config set fetch-retry-maxtimeout 120000 && \
    npm config set fetch-timeout 300000

# Copy package files (include lockfile for faster, deterministic builds)
COPY package.json package-lock.json* ./

# Install dependencies
# Use npm ci if lockfile exists (faster), otherwise fall back to npm install
RUN if [ -f package-lock.json ]; then \
        npm ci --legacy-peer-deps; \
    else \
        npm install --legacy-peer-deps; \
    fi

# Copy source code
COPY . .

# Build the application
RUN npm run build

# Production stage with nginx-unprivileged (runs as non-root)
FROM nginxinc/nginx-unprivileged:alpine

# Switch to root to set up files
USER root

# Install envsubst for runtime config
RUN apk add --no-cache gettext

# Copy nginx configuration template
COPY nginx.conf /etc/nginx/nginx.conf.template

# Copy built files from builder
COPY --from=builder /app/dist /usr/share/nginx/html

# Create directories and set permissions for non-root user (nginx user is 101)
RUN mkdir -p /var/cache/nginx /var/run /tmp/nginx && \
    chown -R 101:101 /var/cache/nginx /var/run /tmp/nginx && \
    chown -R 101:101 /usr/share/nginx/html && \
    chown 101:101 /etc/nginx/nginx.conf.template && \
    chmod 755 /usr/share/nginx/html

# Create the runtime config directory with write permissions
RUN touch /etc/nginx/nginx.conf && \
    chown 101:101 /etc/nginx/nginx.conf && \
    chmod 644 /etc/nginx/nginx.conf

# Create entrypoint script that runs before nginx
RUN cat > /docker-entrypoint.d/99-configure-env.sh << 'SCRIPT'
#!/bin/sh
set -e

# Generate nginx config from template
# VITE_API_URL is the internal API URL for nginx proxy (e.g., http://agentbox-api:8080)
envsubst '$VITE_API_URL' < /etc/nginx/nginx.conf.template > /tmp/nginx.conf
cp /tmp/nginx.conf /etc/nginx/nginx.conf

# Create runtime config for frontend JavaScript
# Browser always uses relative URL so nginx can proxy the requests
# BROWSER_API_URL can override if direct browser-to-API access is needed
BROWSER_URL="${BROWSER_API_URL:-/api/v1}"

cat > /usr/share/nginx/html/config.js << EOF
window.AGENTBOX_CONFIG = {
  API_URL: "${BROWSER_URL}",
  WS_URL: "${VITE_WS_URL:-}",
  GOOGLE_OAUTH_ENABLED: "${VITE_GOOGLE_OAUTH_ENABLED:-false}"
};
EOF

echo "Runtime configuration generated:"
echo "  Nginx proxy URL: ${VITE_API_URL:-not set}"
echo "  Browser API URL: ${BROWSER_URL}"
SCRIPT

RUN chmod +x /docker-entrypoint.d/99-configure-env.sh

# Switch back to non-root user
USER 101

# Expose port (nginx-unprivileged uses 8080 by default)
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ || exit 1

# Use default entrypoint which runs scripts in /docker-entrypoint.d/
CMD ["nginx", "-g", "daemon off;"]

# Build stage
FROM node:20-alpine AS builder

# Set working directory
WORKDIR /app

# Copy package files
COPY package.json ./

# Install dependencies
RUN npm install --legacy-peer-deps

# Copy source code
COPY . .

# Build the application
RUN npm run build

# Production stage with nginx
FROM nginx:alpine

# Install envsubst for runtime config
RUN apk add --no-cache gettext

# Copy nginx configuration
COPY nginx.conf /etc/nginx/nginx.conf.template

# Copy built files from builder
COPY --from=builder /app/dist /usr/share/nginx/html

# Create runtime config script
RUN echo '#!/bin/sh' > /docker-entrypoint.d/99-configure-env.sh && \
    echo 'envsubst "\$VITE_API_URL" < /etc/nginx/nginx.conf.template > /etc/nginx/nginx.conf' >> /docker-entrypoint.d/99-configure-env.sh && \
    echo '# Create runtime config for frontend' >> /docker-entrypoint.d/99-configure-env.sh && \
    echo 'cat > /usr/share/nginx/html/config.js << EOF' >> /docker-entrypoint.d/99-configure-env.sh && \
    echo 'window.AGENTBOX_CONFIG = {' >> /docker-entrypoint.d/99-configure-env.sh && \
    echo '  API_URL: "${VITE_API_URL:-/api/v1}",' >> /docker-entrypoint.d/99-configure-env.sh && \
    echo '  WS_URL: "${VITE_WS_URL:-}",' >> /docker-entrypoint.d/99-configure-env.sh && \
    echo '  GOOGLE_OAUTH_ENABLED: "${VITE_GOOGLE_OAUTH_ENABLED:-false}"' >> /docker-entrypoint.d/99-configure-env.sh && \
    echo '};' >> /docker-entrypoint.d/99-configure-env.sh && \
    echo 'EOF' >> /docker-entrypoint.d/99-configure-env.sh && \
    chmod +x /docker-entrypoint.d/99-configure-env.sh

# Expose port
EXPOSE 3000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:3000/ || exit 1

# Start nginx
CMD ["nginx", "-g", "daemon off;"]

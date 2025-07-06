# Multi-stage Dockerfile for AC³ Astronomy GUI
# Stage 1: Build the React application
FROM node:20-alpine as build

# Set working directory
WORKDIR /app

# Copy package files
COPY package*.json ./

# Install dependencies
RUN npm ci

# Copy source code
COPY . .

# Build the application
RUN npm run build

# Stage 2: Serve the application with nginx
FROM nginx:alpine

# Copy built application from build stage
COPY --from=build /app/dist /usr/share/nginx/html

# Copy custom nginx configuration
COPY nginx.conf /etc/nginx/nginx.conf

# Create necessary directories for nginx with proper permissions for OpenShift
RUN mkdir -p /tmp/nginx_client_temp \
    /tmp/nginx_proxy_temp \
    /tmp/nginx_fastcgi_temp \
    /tmp/nginx_uwsgi_temp \
    /tmp/nginx_scgi_temp \
    /var/log/nginx \
    /var/run \
    && chmod -R 777 /tmp/nginx_client_temp \
    && chmod -R 777 /tmp/nginx_proxy_temp \
    && chmod -R 777 /tmp/nginx_fastcgi_temp \
    && chmod -R 777 /tmp/nginx_uwsgi_temp \
    && chmod -R 777 /tmp/nginx_scgi_temp \
    && chmod -R 777 /var/log/nginx \
    && chmod -R 777 /var/run \
    && chmod -R 777 /usr/share/nginx/html

# Set permissions for OpenShift compatibility - make everything world-writable
RUN chmod -R g+rwx /tmp/nginx_client_temp \
    && chmod -R g+rwx /tmp/nginx_proxy_temp \
    && chmod -R g+rwx /tmp/nginx_fastcgi_temp \
    && chmod -R g+rwx /tmp/nginx_uwsgi_temp \
    && chmod -R g+rwx /tmp/nginx_scgi_temp \
    && chmod -R g+rwx /var/log/nginx \
    && chmod -R g+rwx /var/run \
    && chmod -R g+rwx /usr/share/nginx/html

# Expose port 8080
EXPOSE 8080

# Add health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/ || exit 1

# Start nginx
CMD ["nginx", "-g", "daemon off;"] 
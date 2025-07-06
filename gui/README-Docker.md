# AC³ Astronomy GUI - Docker Setup

This directory contains the Docker configuration for the AC³ Astronomy GUI React application.

## Files Overview

- `Dockerfile` - Multi-stage build for the React application
- `nginx.conf` - Nginx configuration for serving the app and proxying API calls
- `docker-compose.yml` - Full stack deployment with backend services
- `docker-compose.dev.yml` - Development setup (GUI only)
- `.dockerignore` - Excludes unnecessary files from Docker build context

## Quick Start

### Option 1: Development (GUI Only)

Build and run just the GUI container:

```bash
# Build the image
docker build -t ucm-gui:latest .

# Run the container
docker run -p 3000:80 ucm-gui:latest
```

Or use docker-compose:

```bash
# Run with docker-compose
docker-compose -f docker-compose.dev.yml up --build
```

The GUI will be available at `http://localhost:3000`

### Option 2: Full Stack

To run the complete UCM application stack:

```bash
# Start all services
docker-compose up --build

# Run in background
docker-compose up -d --build
```

This will start:
- GUI at `http://localhost:3000`
- Backend API at `http://localhost:8080`
- RabbitMQ Management at `http://localhost:15672` (admin/password)

## Environment Variables

The GUI supports the following environment variables:

- `VITE_API_URL` - Backend API URL (default: `http://localhost:8080/api`)

## Building for Production

```bash
# Build production image
docker build -t ucm-gui:production .

# Run production container
docker run -p 80:80 \
  -e VITE_API_URL=https://your-backend-api.com/api \
  ucm-gui:production
```

## Nginx Configuration

The included nginx configuration provides:

- **Static file serving** with optimal caching
- **API proxying** to backend services
- **Client-side routing** support for React Router
- **CORS handling** for cross-origin requests
- **Security headers** for production use
- **Health check endpoint** at `/health`
- **Large file upload support** (up to 1GB for astronomical data)

## Development Notes

### API Proxy Configuration

The nginx configuration includes an API proxy that forwards requests from `/api/*` to the backend service. Adjust the proxy_pass URL in `nginx.conf` if your backend runs on a different host/port.

### File Upload Limits

The configuration supports large file uploads (up to 1GB) which is necessary for astronomical FITS files. Adjust `client_max_body_size` in `nginx.conf` if needed.

### Health Checks

The container includes health checks that verify nginx is running and serving content. The health check endpoint is available at `/health`.

## Troubleshooting

### Common Issues

1. **Build fails with npm install**
   - Ensure you have a stable internet connection
   - Try clearing npm cache: `docker build --no-cache -t ucm-gui:latest .`

2. **API calls fail**
   - Check the `VITE_API_URL` environment variable
   - Verify the backend service is running and accessible
   - Check nginx logs: `docker logs <container_name>`

3. **Static files not loading**
   - Ensure the build process completed successfully
   - Check that the dist directory was created during build

### Viewing Logs

```bash
# View container logs
docker logs <container_name>

# Follow logs in real-time
docker logs -f <container_name>

# View nginx access logs
docker exec <container_name> cat /var/log/nginx/access.log
```

## Customization

### Modifying Nginx Configuration

Edit `nginx.conf` to:
- Change API proxy settings
- Adjust caching policies
- Modify security headers
- Change upload limits

### Adding Environment Variables

Add environment variables to:
- `docker-compose.yml` for full stack deployment
- `docker-compose.dev.yml` for development
- Pass them directly with `docker run -e VAR=value`

## Security Considerations

The nginx configuration includes several security headers:
- `X-Frame-Options: SAMEORIGIN`
- `X-Content-Type-Options: nosniff`
- `X-XSS-Protection: 1; mode=block`
- `Referrer-Policy: strict-origin-when-cross-origin`

For production deployment, consider:
- Using HTTPS with proper SSL certificates
- Implementing rate limiting
- Adding authentication layers
- Restricting CORS origins 
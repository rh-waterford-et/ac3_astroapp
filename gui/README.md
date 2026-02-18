# GUI

React web interface for the astronomical data processing system.

## Features

- Dataset management (create, delete, upload files)
- Processing pipeline monitoring
- Aladin Lite sky viewer integration
- Result visualisation gallery
- File browser with pagination

## Tech Stack

- React 19
- Vite 7
- CSS (no UI framework)

## Development

### Prerequisites

- Node.js 18+
- npm

### Install Dependencies

```bash
npm install
```

### Run Development Server

```bash
npm run dev
```

Access at `http://localhost:5173`

### Build for Production

```bash
npm run build
```

Output in `dist/`.

## Deployment

### Build and Deploy Container

```bash
make rebuild   # Build, push, deploy
```

Image: `quay.io/bcapper30/ac3-gui:latest`

### Manual Steps

```bash
podman build --platform linux/amd64 -t quay.io/bcapper30/ac3-gui:latest .
podman push quay.io/bcapper30/ac3-gui:latest
oc rollout restart deployment/gui -n uc3-applications
```

## Structure

```
src/
├── components/
│   ├── aladin/       # Sky viewer components
│   ├── pipeline/     # Processing pipeline UI
│   ├── ui/           # Shared UI components
│   └── upload/       # File upload components
├── contexts/         # React contexts
├── hooks/            # Custom React hooks
│   ├── data/         # Data fetching hooks
│   ├── gallery/      # Gallery state hooks
│   └── ui/           # UI interaction hooks
├── services/
│   └── api.js        # Backend API client
├── styles/           # CSS modules
└── utils/            # Utility functions
```

## API Integration

The frontend communicates with the backend at `/api/`. Key endpoints:

| Endpoint | Purpose |
|----------|---------|
| `/api/datasets` | List/create datasets |
| `/api/datasets/files` | List files with pagination |
| `/api/datasets/upload` | Upload files |
| `/api/queue/trigger` | Start processing |

## Configuration

API base URL is configured in `src/services/api.js`. In production, nginx proxies `/api/` to the backend service.

### Nginx Config

See `nginx.conf` for production routing.

## Environment Variables

Set at build time in Dockerfile or runtime via nginx:

| Variable | Description |
|----------|-------------|
| `VITE_API_URL` | Backend API URL (development) |

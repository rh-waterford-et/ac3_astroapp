# Astroapp Backend

Go-based backend for distributed astronomical data processing using RabbitMQ message queues.

## Architecture

```
                    producer_to_processor_queue
┌──────────────┐  ─────────────────────────────>  ┌─────────────────┐
│   Producer   │         RabbitMQ                 │    Processor    │
│              │  <─────────────────────────────  │ (receiver+app)  │
└──────────────┘    processor_to_producer_queue   └─────────────────┘
       │                                                   │
       v                                                   v
   S3 Input                                           S3 Output
```

Two RabbitMQ queues handle bidirectional communication:
- `producer_to_processor_queue` - Jobs sent from producer to processors
- `processor_to_producer_queue` - Results returned from processors to producer

## Message Types

Different applications use different message formats based on file characteristics:

| Application | Message Type | Description |
|-------------|--------------|-------------|
| Starlight | Standard | Text-based messages with file content embedded |
| pPXF | Binary | Binary messages to preserve .fits file integrity |
| Voronoi | S3 Reference | Only S3 metadata sent; files too large for RabbitMQ, processor downloads directly from S3 |

## Directory Structure

```
application/
├── cmd/ucm/          # Main entry point
├── pkg/
│   ├── api/          # HTTP handlers and REST API
│   ├── producer/     # Job creation (standard, binary, s3-reference)
│   ├── receiver/     # Job consumption and processing
│   ├── sender/       # Message publishing to queues
│   ├── s3bucket/     # S3 client operations
│   ├── watcher/      # File system monitoring
│   ├── metrics/      # Prometheus metrics
│   └── server/       # HTTP server setup
├── deployments/      # Kubernetes manifests
└── hscaler/          # Horizontal pod autoscaler predictor
```

## Build and Deploy

```bash
cd application
make build     # Build container image
make push      # Push to registry
make deploy    # Restart deployments
make rebuild   # All of the above
```

## Deployment Files

| File | Description |
|------|-------------|
| `namespace.yaml` | Kubernetes namespace |
| `rabbitmq.yaml` | RabbitMQ deployment and service |
| `producer.yaml` | Producer deployment (multi-container) |
| `processor.yaml` | Processor deployment (scientific app + receiver) |
| `consumer.yaml` | Standalone consumer |
| `volume.yaml` | Persistent volume definitions |
| `hpa.yaml` | Horizontal Pod Autoscaler |

## API Endpoints

The backend exposes a REST API on port 8080:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/datasets` | GET | List available datasets |
| `/api/datasets/files` | GET | List files in dataset |
| `/api/datasets/upload` | POST | Upload files to dataset |
| `/api/queue/trigger` | POST | Trigger processing for dataset |
| `/api/admin/restart-rabbitmq` | POST | Restart queue infrastructure |

## Scientific Applications

### Starlight
Stellar population synthesis fitting. Uses standard text messages. Located in `starlight/`.

### pPXF
Penalized Pixel-Fitting for stellar kinematics. Uses binary messages to prevent .fits file corruption. Located in `ppxf/`.

### Voronoi
Voronoi binning for spectral data. Uses S3 references due to large datacube file sizes. Processor downloads files directly from S3 rather than receiving content via RabbitMQ. Located in `voronoi/`.

## Configuration

Environment variables (set in deployment YAML):

| Variable | Description |
|----------|-------------|
| `RABBITMQ_HOST` | RabbitMQ service hostname |
| `RABBITMQ_USER` | Queue username |
| `RABBITMQ_PASS` | Queue password |
| `S3_ENDPOINT` | S3-compatible endpoint URL |
| `S3_BUCKET_NAME` | Target bucket name |
| `AWS_ACCESS_KEY_ID` | S3 access key |
| `AWS_SECRET_ACCESS_KEY` | S3 secret key |

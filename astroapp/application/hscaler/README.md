# HScaler Orchestrator

A Python-based system for predictive job time estimation with Prometheus integration.

## Components

### 1. Predictor Client
- Queries Prometheus for `num_processors`, `job_size`, and `queue_len` metrics
- Sends these metrics to a prediction API
- Receives `predicted_job_time` from the API
- Maintains a sliding window average of predictions
- Exports the averaged value to Prometheus

### 2. Model API
- Provides a prediction API endpoint using a trained machine learning model
- Accepts job metrics and returns predicted job times
- Uses XGBoost model for accurate predictions based on historical data
- Includes health check endpoint that reports model status
- Deployed as a standalone pod in Kubernetes

## Local Installation

### Predictor Client
```bash
pip install -r requirements.txt
python predictor_client.py
```

### Model API
```bash
cd mock_api
pip install -r requirements-mock-api.txt
python mock_api.py
```

**Note**: The model file (`model.pkl`) must be present in the same directory as `mock_api.py`.

## Configuration

Edit `config.yaml` (local) or `k8s/configmap.yaml` (Kubernetes) to configure:

- **predictor_api.url**: Your prediction API endpoint (default: `http://model-api:5000/predict` in k8s)
  - The API uses a trained XGBoost model to predict job execution times
- **prometheus.url**: Your Prometheus server URL
- **prometheus_queries**: PromQL queries for each metric
- **query_interval_seconds**: How often to query the API
- **averaging_window_size**: Number of predictions to average
- **exporter.port**: Port for the Prometheus metrics exporter

## How It Works

1. Predictor client queries Prometheus for metrics every `query_interval_seconds`
2. Client sends POST request to prediction API with JSON payload
3. API uses trained XGBoost model to predict job time based on:
   - `processor_count`: Number of available processors
   - `job_size_mb`: Size of the job in megabytes
   - `queue_ahead_length`: Number of jobs waiting in queue
4. API returns `predicted_job_time` in seconds
5. Client updates sliding window average
6. Client exports average as `predicted_job_time_avg` metric to Prometheus

## Kubernetes Deployment

### Build and Push Docker Images

```bash
# Build predictor-client image
docker build -t predictor-client:latest .

# Build model-api image
docker build -f Dockerfile.mock_api -t model-api:latest .

# Tag for your registry (optional)
docker tag predictor-client:latest your-registry.com/predictor-client:latest
docker tag model-api:latest your-registry.com/model-api:latest

# Push to registry (optional)
docker push your-registry.com/predictor-client:latest
docker push your-registry.com/model-api:latest
```

### Prepare Model File

Ensure your trained model file (`model.pkl`) is in the `mock_api/` directory before building the Docker image.

### Update ConfigMap

Edit `k8s/configmap.yaml` to set:
- Prediction API endpoint (default: `http://model-api:5000/predict`)
- Prometheus server URL (e.g., `http://prometheus-server:9090`)
- Actual PromQL queries for your metrics

### Deploy to Kubernetes

```bash
# Deploy all components using kustomize
kubectl apply -k k8s/

# Or deploy individual manifests
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
kubectl apply -f k8s/model-api-deployment.yaml
kubectl apply -f k8s/model-api-service.yaml
```

### Verify Deployment

```bash
# Check all pods
kubectl get pods -l app.kubernetes.io/part-of=hscaler-orchestrator

# Check predictor-client pod
kubectl get pods -l app=predictor-client

# Check model-api pod
kubectl get pods -l app=model-api

# View predictor-client logs
kubectl logs -l app=predictor-client -f

# View model-api logs
kubectl logs -l app=model-api -f

# Check services
kubectl get svc predictor-client model-api
```

### Test the Model API

```bash
# Port-forward to test the API
kubectl port-forward svc/model-api 5000:5000

# In another terminal, test the health endpoint
curl http://localhost:5000/health

# Test the prediction endpoint
curl -X POST http://localhost:5000/predict \
  -H "Content-Type: application/json" \
  -d '{"num_processors": 4, "job_size": 10.5, "queue_len": 5}'
```

Expected response:
```json
{
  "predicted_job_time": 284.23,
  "metadata": {
    "num_processors": 4,
    "job_size": 10.5,
    "queue_len": 5
  }
}
```

### Update Configuration

```bash
# Edit the ConfigMap
kubectl edit configmap predictor-client-config

# Restart the deployment to pick up changes
kubectl rollout restart deployment predictor-client
```

## Prometheus Scrape Configuration

### Local Setup
Add this to your Prometheus configuration to scrape the exported metric:

```yaml
scrape_configs:
  - job_name: 'predictor_client'
    static_configs:
      - targets: ['localhost:8000']
```

### Kubernetes Setup (with Prometheus Operator)
The deployment includes annotations for automatic scraping:
```yaml
prometheus.io/scrape: "true"
prometheus.io/port: "8000"
prometheus.io/path: "/metrics"
```

Alternatively, add a ServiceMonitor:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: predictor-client
spec:
  selector:
    matchLabels:
      app: predictor-client
  endpoints:
  - port: metrics
    interval: 30s
```

## API Contract

### Request to Prediction API
```json
{
  "num_processors": 4.0,
  "job_size": 1024.0,
  "queue_len": 5.0
}
```

### Response from Prediction API
```json
{
  "predicted_job_time": 42.5
}
```

## Exported Metric

- `predicted_job_time_avg`: Averaged predicted job time over the configured window

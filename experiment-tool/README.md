# Experiment Tool

Go CLI tool for orchestrating astronomical data processing experiments and collecting performance metrics.

## Purpose

- Submit datasets to the processing backend
- Monitor S3 for job completion
- Collect timing metrics (queue time, processing time, total latency)
- Generate CSV reports for analysis

## Build and Deploy

```bash
cd experiment-tool
make build     # Build container image
make push      # Push to registry
make deploy    # Restart deployment
make rebuild   # All of the above
```

Image: `quay.io/bcapper30/uc3-experiment-tool:latest`

## Usage

### Access the Container

```bash
oc exec -it deployment/experiment-tool -n uc3-applications -- /bin/sh
```

### Run an Experiment

```bash
./uc3-experiment start --config /app/configs/scenarios/phase1-start-interval-baseline/test01-burst-5proc-10datasets.yaml
```

### Retrieve Results

```bash
POD=$(oc get pod -n uc3-applications -l app=experiment-tool -o jsonpath='{.items[0].metadata.name}')
oc cp uc3-applications/$POD:/app/experiment-data ./experiment-results
```

## Configuration

YAML configuration files in `configs/scenarios/`. Example:

```yaml
name: "experiment-name"
description: "Description"

scaling:
  enable_scaling: true
  processor_count: 5
  stabilize_time: "30s"

workload:
  datasets:
    - path: "starlight/input/NGC7020"
      name: "NGC7020"
  processor_type: "starlight"
  start_interval: "0s"

metrics:
  output_directory: "/app/experiment-data"

infrastructure:
  namespace: "uc3-applications"
  deployment_name: "ucm-processor-deployment"
  uc3_api_base_url: "http://uc3-backend-service:8080"
  trigger_url: "http://uc3-backend-service:8081"
```

## Output Files

| File | Description |
|------|-------------|
| `experiment_summary.csv` | Overall experiment statistics |
| `dataset_timeline.csv` | Timeline of dataset processing |
| `all_jobs_detailed.csv` | Per-job timing metrics |
| `training_data.csv` | ML training data format |
| `experiment.log` | Full execution log |

## Directory Structure

```
experiment-tool/
├── cmd/uc3-experiment/   # CLI entry point and commands
├── pkg/
│   ├── api/              # Backend API client
│   ├── collector/        # S3 monitoring and metrics collection
│   ├── config/           # Configuration loading
│   ├── orchestrator/     # Experiment orchestration
│   └── scaler/           # Kubernetes scaling operations
├── configs/              # Experiment configurations
│   ├── defaults/         # Default settings
│   └── scenarios/        # Experiment scenarios
├── scripts/              # Utility scripts
└── deployments/          # Kubernetes manifests
```

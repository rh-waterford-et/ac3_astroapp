# UC3 Experiment Tool - Quick Start Guide

## 🚀 Fast Track (HPA Users)

### 1. Access Container
```bash
oc exec -it deployment/experiment-tool -n uc3-applications -- /bin/sh
```

### 2. Create HPA-Compatible Config
```bash
cat > /app/my-experiment.yaml << 'EOF'
name: "my-hpa-experiment"
description: "Experiment with HPA-managed scaling"

scaling:
  enable_scaling: false     # ⚠️ IMPORTANT: Disable when using HPA
  processor_count: 3        # For reporting only
  stabilize_time: "30s"     # Wait for HPA stabilization

workload:
  dataset: "/app/datasets/NGC7020"
  processor_type: "starlight"

metrics:
  output_directory: "/app/experiment-data"

infrastructure:
  namespace: "uc3-applications"
  deployment_name: "ucm-processor-deployment"
  uc3_api_base_url: "http://uc3-backend-service:8080"
  redis_host: "redis"
  redis_port: 6379
EOF
```

### 3. Run Experiment
```bash
oc exec -it deployment/experiment-tool -n uc3-applications -- /bin/sh
./uc3-experiment start --config /app/configs/scenarios/phase1-start-interval-baseline/test01-burst-5proc-10datasets.yaml
```

### 4. View Results
```bash
ls -lh /app/experiment-data/
cat /app/experiment-data/experiment_summary.csv
```

### 5. Copy Results (from your local terminal)
```bash
# Get pod name
POD=$(oc get pod -n uc3-applications -l app=experiment-tool -o jsonpath='{.items[0].metadata.name}')

# Copy all data
oc cp uc3-applications/$POD:/app/experiment-data ./my-experiment-results
```

---

## 📋 Essential Commands

### Access & Navigation
```bash
# Access container
oc exec -it deployment/experiment-tool -n uc3-applications -- /bin/sh

# Change to app directory
cd /app

# Check version
./uc3-experiment version
```

### Running Experiments
```bash
# Quick test (1 processor, manual scaling)
./uc3-experiment start --config configs/scenarios/quick-test.yaml

# Multi-dataset test
./uc3-experiment start --config configs/scenarios/multi-dataset-test.yaml

# Custom config
./uc3-experiment start --config /app/my-config.yaml
```

### Monitoring
```bash
# Watch logs (from another terminal)
oc logs -f deployment/experiment-tool -n uc3-applications

# Check processor deployment
kubectl get deployment ucm-processor-deployment -n uc3-applications

# Monitor experiment data
watch ls -lh /app/experiment-data/
```

### Results
```bash
# List output files
ls /app/experiment-data/

# View summary
cat /app/experiment-data/experiment_summary.csv

# View timeline
cat /app/experiment-data/dataset_timeline.csv

# Copy to local (from local terminal, not in container)
oc cp uc3-applications/<pod-name>:/app/experiment-data ./results
```

---

## 🔧 Configuration Templates

### Minimal (HPA-Compatible)
```yaml
name: "test"
scaling:
  enable_scaling: false
  stabilize_time: "30s"
workload:
  dataset: "/app/datasets/NGC7020"
  processor_type: "starlight"
metrics:
  output_directory: "/app/experiment-data"
infrastructure:
  namespace: "uc3-applications"
  deployment_name: "ucm-processor-deployment"
  uc3_api_base_url: "http://uc3-backend-service:8080"
  redis_host: "redis"
  redis_port: 6379
```

### Multi-Dataset (HPA-Compatible)
```yaml
name: "multi-test"
scaling:
  enable_scaling: false
  stabilize_time: "30s"
workload:
  datasets:
    - name: "/app/datasets/NGC7020"
      processor_type: "starlight"
    - name: "/app/datasets/NGC7025"
      processor_type: "starlight"
  dataset_start_interval: "30s"
  failure_strategy: "continue"
metrics:
  output_directory: "/app/experiment-data"
infrastructure:
  namespace: "uc3-applications"
  deployment_name: "ucm-processor-deployment"
  uc3_api_base_url: "http://uc3-backend-service:8080"
  redis_host: "redis"
  redis_port: 6379
```

### Manual Scaling (No HPA)
```yaml
name: "manual-test"
scaling:
  enable_scaling: true      # Experiment tool controls scaling
  processor_count: 5
  stabilize_time: "60s"
workload:
  dataset: "/app/datasets/NGC7020"
  processor_type: "starlight"
metrics:
  output_directory: "/app/experiment-data"
infrastructure:
  namespace: "uc3-applications"
  deployment_name: "ucm-processor-deployment"
  uc3_api_base_url: "http://uc3-backend-service:8080"
  redis_host: "redis"
  redis_port: 6379
```

---

## ⚠️ Important Notes

1. **HPA Users**: ALWAYS set `enable_scaling: false` to avoid conflicts
2. **Dataset Paths**: Use `/app/datasets/<dataset-name>` inside container
3. **Output Location**: Results go to `/app/experiment-data/`
4. **Long Runs**: Use `screen` or `tmux` to avoid disconnections
5. **Monitoring**: Always watch logs in a separate terminal

---

## 📖 Full Documentation

See `CONTAINER_USAGE.md` for comprehensive documentation including:
- All available commands
- Troubleshooting guide
- Advanced workflows
- Environment variables
- Best practices


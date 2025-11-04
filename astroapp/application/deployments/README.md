# UC3 Application Deployment

Modern, consolidated deployment configuration for the UC3 astrophysics application on OpenShift/Kubernetes.

## 🚀 Quick Start

### Basic Deployment
```bash
./deploy-unified.sh
```

### Deploy with Monitoring
```bash
./deploy-unified.sh --monitoring
```

### Deploy Everything
```bash
./deploy-unified.sh --all
```

### Dry Run (preview changes)
```bash
./deploy-unified.sh --dry-run --all
```

## 📋 Deployment Options

| Flag | Description |
|------|-------------|
| `-m, --monitoring` | Deploy Prometheus & Grafana monitoring stack |
| `-h, --hscaler` | Deploy HScaler predictor client for auto-scaling |
| `-k, --mock` | Deploy Mock API for testing predictions |
| `-a, --all` | Deploy everything (monitoring + hscaler) |
| `-d, --dry-run` | Preview deployment without applying changes |
| `--help` | Show help message |

## 📁 File Structure

### Core Configuration
| File | Description |
|------|-------------|
| `namespace.yaml` | uc3-applications namespace |
| `secrets.yaml` | **Shared secrets** (RabbitMQ, AWS/S3, Redis) |
| `configmap.yaml` | **Shared configuration** (common env vars) |
| `rbac.yaml` | **Consolidated RBAC** (ServiceAccounts + RoleBindings) |
| `storage.yaml` | **Consolidated storage** (PVs + PVCs) |

### Infrastructure Services
| File | Description |
|------|-------------|
| `deployment_rabbitmq.yaml` | RabbitMQ message queue (Deployment + Service) |
| `deployment_redis.yaml` | Redis cache (Deployment + Service) |

### Backend Services
| File | Description |
|------|-------------|
| `deployment_producer.yaml` | Producer pods (watcher, http-server, receiver, aggregator) |
| `deployment_processor.yaml` | Processor pods (STARLIGHT, PPXF, receiver, watcher) |
| `service_backend.yaml` | Backend API service & OpenShift route |
| `service_monitor_metrics.yaml` | Metrics service |

### Frontend
| File | Description |
|------|-------------|
| `deployment_frontend.yaml` | React GUI (Deployment + Service + Route) |

### Monitoring (Optional)
| File | Description |
|------|-------------|
| `cluster-monitoring-config.yaml` | OpenShift cluster monitoring integration |
| `prometheus_rules.yaml` | Prometheus alerting rules |
| `deployment_prometheus.yaml` | Prometheus server |
| `deployment_grafana.yaml` | Grafana dashboards |
| `deployment_pod_metrics_exporter.yaml` | Pod-level metrics exporter |

### Auto-Scaling (Optional)
| File | Description |
|------|-------------|
| `hscaler.yaml` | **HScaler** (ConfigMap + Deployment + Service + ServiceMonitor) |
| `hscaler-mock.yaml` | **Mock API** for testing (Deployment + Service) |

### Legacy Scripts (for reference)
| File | Description |
|------|-------------|
| `deploy.sh` | Legacy basic deployment script |
| `deploy_with_monitoring.sh` | Legacy monitoring deployment script |
| `deploy_hscaler.sh` | Legacy hscaler deployment script |
| `deploy_hscaler_mockapi.sh` | Legacy mock API deployment script |

## 🎯 Key Improvements

### ✅ Consolidated Resources
- **Secrets**: All secrets in `secrets.yaml` (previously scattered across files)
- **ConfigMap**: Shared config in `configmap.yaml` (eliminates duplication)
- **RBAC**: All ServiceAccounts + RoleBindings in `rbac.yaml`
- **Storage**: All PVs + PVCs in `storage.yaml`
- **HScaler**: All hscaler resources in single `hscaler.yaml` file
- **Mock API**: Combined into single `hscaler-mock.yaml` file

### ✅ Security Enhancements
- Removed hardcoded secrets from producer deployment
- Centralized secret management in `secrets.yaml`
- All deployments now reference shared secrets

### ✅ Configuration Management
- Eliminated 100+ lines of duplicated environment variables
- Single source of truth for S3, Redis, RabbitMQ configuration
- Easier to update configurations across all services

### ✅ Unified Deployment
- Single script with flags for different deployment scenarios
- Dry-run capability for testing
- Clear step-by-step output with colored logging
- Comprehensive help documentation

## 🌐 Access Endpoints

After deployment, get access URLs:

### Frontend
```bash
oc get route uc3-frontend-route -n uc3-applications
```

### Backend API
```bash
oc get route uc3-backend-api -n uc3-applications
```

### Monitoring (if deployed with `-m`)
```bash
# Prometheus
oc get route prometheus-server -n uc3-applications

# Grafana (credentials: admin/admin)
oc get route grafana -n uc3-applications

# Pod Metrics
oc get route pod-metrics-exporter -n uc3-applications
```

## 📊 Monitoring

### Check Deployment Status
```bash
oc get all -n uc3-applications
```

### View Pod Logs
```bash
# Producer logs
oc logs -f deployment/ucm-producer-deployment -c watcher-producer -n uc3-applications

# Processor logs
oc logs -f deployment/ucm-processor -c receiver -n uc3-applications

# HScaler logs (if deployed)
oc logs -f deployment/predictor-client -n uc3-applications
```

### Port Forwarding (for local testing)
```bash
# Prometheus
oc port-forward svc/prometheus-server 9090:9090 -n uc3-applications

# Grafana
oc port-forward svc/grafana 3000:3000 -n uc3-applications

# RabbitMQ Management UI
oc port-forward svc/rabbitmq 15672:15672 -n uc3-applications
```

## 🔧 Troubleshooting

### Check Pod Status
```bash
oc get pods -n uc3-applications -w
```

### Describe Resources
```bash
oc describe pod <pod-name> -n uc3-applications
oc describe pvc -n uc3-applications
```

### View Events
```bash
oc get events -n uc3-applications --sort-by='.lastTimestamp'
```

### Delete Deployment
```bash
oc delete namespace uc3-applications
```

## 📖 Architecture

### Producer Pod (4 containers)
- **watcher-producer**: Monitors S3 for new data
- **http-server**: REST API server
- **receiver-producer**: Consumes messages from RabbitMQ
- **aggregator**: Aggregates metrics to Redis

### Processor Pod (4 containers)
- **starlight**: STARLIGHT astrophysics processing
- **ppxf**: PPXF (penalized pixel fitting) processing
- **receiver**: Receives processing tasks from RabbitMQ
- **watcher**: Monitors processing completion

### Data Flow
1. Producer watcher monitors S3 for new FITS files
2. Files are queued via RabbitMQ
3. Processor receiver picks up tasks
4. STARLIGHT/PPXF containers process data
5. Results uploaded to S3
6. Metrics aggregated in Redis
7. Frontend displays progress via API

## 🔐 Security Notes

⚠️ **IMPORTANT**: The `secrets.yaml` file contains sensitive credentials:
- AWS/S3 access keys
- RabbitMQ passwords
- Redis passwords

In production:
1. Use **Sealed Secrets** or **External Secrets Operator**
2. Store secrets in a secure vault (e.g., HashiCorp Vault, AWS Secrets Manager)
3. Never commit real secrets to version control
4. Rotate credentials regularly

## 🤝 Contributing

When adding new resources:
1. Use shared `secrets.yaml` and `configmap.yaml` where possible
2. Combine related resources (Deployment + Service) in single files
3. Follow naming convention: `uc3-<component>-<resource>`
4. Update this README
5. Add deployment step to `deploy-unified.sh`

## 📝 Legacy Files

The following files are kept for backward compatibility but are superseded by consolidated versions:
- Individual hscaler files → `hscaler.yaml`
- Individual volume files → `storage.yaml`
- Individual RBAC files → `rbac.yaml`
- Old deployment scripts → `deploy-unified.sh`

These can be removed once migration is verified.

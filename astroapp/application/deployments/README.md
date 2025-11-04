# UC3 Application Deployment

## 🚀 Quick Deploy

```bash
./deploy.sh
```

## 📁 Files

**Core Deployments:**
- `namespace.yaml` - uc3-applications namespace
- `deployment.yaml` - Main backend services
- `deployment_starlight.yaml` - STARLIGHT processing
- `deployment_frontend.yaml` - React frontend GUI
- `deployment_rabbitmq.yaml` - Message queue

**Storage:**
- `volume.yaml` & `volume-producer.yaml` - Persistent volumes
- `volumeclaim.yaml` & `volumeclaim-producer.yaml` - Volume claims

**Security:**
- `starlight-sa.yaml` - Service account
- `rolebinding.yaml` - RBAC permissions

**Services:**
- `service_backend.yaml` - Backend API service and route

**Monitoring:**
- `deployment_rabbitmq-exporter.yaml` - RabbitMQ metrics (optional)

## 🌐 Access

After deployment:
- **Frontend**: `oc get route uc3-frontend-route -n uc3-applications`
- **API**: `oc get route uc3-backend-api -n uc3-applications`

## 📊 Status

```bash
oc get all -n uc3-applications
```

## 🔧 Frontend-Backend Communication

- Frontend: `quay.io/bcapper30/uc3-apps:latest`
- Backend: `quay.io/bcapper30/uc3-backend:latest`
- API URL: Configured in frontend environment variables
- CORS: Enabled in backend for cross-origin requests 
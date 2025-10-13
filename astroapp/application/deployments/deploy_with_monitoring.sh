#!/bin/bash

echo "🚀 Deploying UC3 Application with Prometheus Monitoring"
echo "========================================================"

# Create namespace
echo "🏗️ Creating namespace..."
oc apply -f namespace.yaml

# Deploy storage resources
echo "📦 Deploying storage..."
oc apply -f volume.yaml
oc apply -f volume-producer.yaml
oc apply -f volumeclaim.yaml
oc apply -f volumeclaim-producer.yaml

# Deploy service account and RBAC
echo "🔐 Deploying security..."
oc apply -f starlight-sa.yaml
oc apply -f rolebinding.yaml

# Deploy RabbitMQ
echo "🐰 Deploying RabbitMQ..."
oc apply -f deployment_rabbitmq.yaml

# Deploy Redis
echo "🔴 Deploying Redis..."
oc apply -f deployment_redis.yaml

# Deploy Prometheus monitoring stack
echo "📊 Deploying Prometheus monitoring..."
oc apply -f deployment_prometheus.yaml
oc apply -f deployment_pod_metrics_exporter.yaml
oc apply -f prometheus_rules.yaml

# Deploy Grafana
echo "📈 Deploying Grafana..."
oc apply -f deployment_grafana.yaml

# Deploy backend services
echo "⚙️ Deploying backend..."
oc apply -f deployment_producer.yaml
oc apply -f deployment_processor.yaml
oc apply -f service_backend.yaml

# Deploy frontend
echo "🌐 Deploying frontend..."
oc apply -f deployment_frontend.yaml

echo "✅ Deployment complete!"
echo ""
echo "📊 Monitoring endpoints:"
echo "Prometheus: oc get route prometheus-server -n uc3-applications"
echo "Grafana: oc get route grafana -n uc3-applications"
echo "Pod Metrics: oc get route pod-metrics-exporter -n uc3-applications"
echo ""
echo "📊 Check status:"
echo "oc get pods -n uc3-applications"
echo "oc get routes -n uc3-applications"
echo ""
echo "🔍 View pod metrics:"
echo "oc port-forward svc/prometheus-server 9090:9090 -n uc3-applications"
echo "Then visit: http://localhost:9090"
echo ""
echo "📈 View Grafana dashboard:"
echo "oc port-forward svc/grafana 3000:3000 -n uc3-applications"
echo "Then visit: http://localhost:3000 (admin/admin)"
echo ""
echo "📋 Import Grafana dashboard:"
echo "1. Go to http://localhost:3000"
echo "2. Click '+' -> Import"
echo "3. Upload the file: grafana/grafana-dashboard.json"

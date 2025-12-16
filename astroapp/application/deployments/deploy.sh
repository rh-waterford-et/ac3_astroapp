#!/bin/bash

echo "🚀 Deploying UC3 Application to OpenShift"
echo "=========================================="

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
oc apply -f rbac.yaml

# Deploy RabbitMQ
echo "🐰 Deploying RabbitMQ..."
oc apply -f rabbitmq.yaml

# Deploy Redis
echo "🔴 Deploying Redis..."
oc apply -f redis.yaml

# Deploy backend services
echo "⚙️ Deploying backend..."
oc apply -f producer.yaml
oc apply -f processor.yaml
oc apply -f service_monitor_metrics.yaml

#Deploy Connector
echo "🔧 Deploying connector..."
oc apply -f consumer.yaml
oc apply -f transfer.yaml
oc apply -f vault.yaml

# Deploy frontend
echo "🌐 Deploying frontend..."
oc apply -f frontend.yaml

# Deploy Scaling
# echo "🔄 Deploying scaling..."
# oc apply -f hscaler.yaml
# oc apply -f hscaler-model.yaml
# oc apply -f hpa.yaml
# oc apply -f prom-adapter.yaml


echo "✅ Deployment complete!"
echo ""
echo "📊 Check status:"
echo "oc get pods -n uc3-applocations"
echo "oc get routes -n uc3-applocations" 
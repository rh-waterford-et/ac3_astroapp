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
oc apply -f starlight-sa.yaml
oc apply -f rolebinding.yaml

# Deploy RabbitMQ
echo "🐰 Deploying RabbitMQ..."
oc apply -f deployment_rabbitmq.yaml

# Deploy backend services
echo "⚙️ Deploying backend..."
oc apply -f deployment.yaml
oc apply -f deployment_starlight.yaml
oc apply -f service_backend.yaml

# Deploy frontend
echo "🌐 Deploying frontend..."
oc apply -f deployment_frontend.yaml

echo "✅ Deployment complete!"
echo ""
echo "📊 Check status:"
echo "oc get pods -n uc3-applications"
echo "oc get routes -n uc3-applications" 
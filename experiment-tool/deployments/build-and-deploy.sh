#!/bin/bash

# Build and deploy script for UC3 Experiment Tool

set -e

IMAGE_REGISTRY="quay.io/bcapper30"
IMAGE_NAME="uc3-experiment-tool"
IMAGE_TAG="${1:-latest}"
FULL_IMAGE="${IMAGE_REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"

echo "Building UC3 Experiment Tool container..."
echo "Image: ${FULL_IMAGE}"

# Build the container image from the ucm_app root directory to access datasets
echo "🔨 Building container image..."
cd ../../../
podman build --platform linux/amd64 -f ac3_astroapp/experiment-tool/Dockerfile -t ${FULL_IMAGE} .

# Push to registry
echo "📤 Pushing to registry..."
podman push ${FULL_IMAGE}

# Deploy to OpenShift
echo "🚀 Deploying to OpenShift..."
cd ac3_astroapp/experiment-tool/deployments

# Apply RBAC permissions first
echo "🔐 Applying RBAC permissions..."
oc apply -f experiment-rbac.yaml

# Apply the main deployment
oc apply -f experiment-tool.yaml

# Wait for deployment to be ready
echo "⏳ Waiting for deployment to be ready..."
oc rollout status deployment/experiment-tool -n uc3-applications

echo "✅ Deployment complete!"
echo "📊 Check status with: oc get pods -n uc3-applications -l app=experiment-tool"
echo "📋 View logs with: oc logs -f deployment/experiment-tool -n uc3-applications" 
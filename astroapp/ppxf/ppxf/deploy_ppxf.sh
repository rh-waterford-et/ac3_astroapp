#!/bin/bash

set -e

# Configuration
REGISTRY="quay.io/bcapper30"
IMAGE_NAME="uc3-ppxf"
TAG="latest"
FULL_IMAGE_NAME="${REGISTRY}/${IMAGE_NAME}:${TAG}"

echo "Building pPXF container..."
echo "Registry: ${REGISTRY}"
echo "Image: ${IMAGE_NAME}"
echo "Tag: ${TAG}"
echo "Full image name: ${FULL_IMAGE_NAME}"

# Build the container image
echo "Building container image..."
podman build --platform linux/amd64 -t "${FULL_IMAGE_NAME}" -f Dockerfile .

# Check if build was successful
if [ $? -eq 0 ]; then
    echo "✅ Container build successful!"
else
    echo "❌ Container build failed!"
    exit 1
fi

# Push the image to registry
echo "Pushing image to registry..."
podman push "${FULL_IMAGE_NAME}"

# Check if push was successful
if [ $? -eq 0 ]; then
    echo "✅ Container push successful!"
    echo "Image available at: ${FULL_IMAGE_NAME}"
else
    echo "❌ Container push failed!"
    exit 1
fi

# Optional: Clean up local build cache
echo "Cleaning up..."
podman system prune -f

echo "✅ pPXF container deployment complete!"
echo "Use this image in your Kubernetes deployment: ${FULL_IMAGE_NAME}" 
#!/bin/bash
set -e

IMAGE_NAME="quay.io/bcapper30/astroapp-voronoi:latest"

echo "🔨 Building Voronoi container..."
podman build --platform linux/amd64 -t "$IMAGE_NAME" .

echo "📤 Pushing to registry..."
podman push "$IMAGE_NAME"

echo "✅ Build and push complete!"
echo "Image: $IMAGE_NAME"


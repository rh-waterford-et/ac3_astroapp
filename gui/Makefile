# UC3 GUI Build and Deploy Makefile
IMAGE_NAME = quay.io/bcapper30/uc3-apps:latest
DOCKERFILE = Dockerfile
DEPLOYMENT_NAME = uc3-frontend
NAMESPACE = uc3-applications

.PHONY: build push deploy rebuild status clean help

# Default target
all: rebuild

# Build the container image
build:
	@echo "🔨 Building UI container..."
	podman build --platform linux/amd64 -t $(IMAGE_NAME) -f $(DOCKERFILE) .
	@echo "✅ Build completed!"

# Push the container image to registry
push:
	@echo "📤 Pushing UI container to registry..."
	podman push $(IMAGE_NAME)
	@echo "✅ Push completed!"

# Restart the deployment in OpenShift
deploy:
	@echo "🚀 Restarting deployment..."
	oc rollout restart deployment/$(DEPLOYMENT_NAME) -n $(NAMESPACE)
	@echo "✅ Deployment restarted!"

# Full rebuild process: build + push + deploy
rebuild: build push deploy
	@echo "🎉 Full rebuild process completed!"

# Check deployment status
status:
	@echo "📊 Checking deployment status..."
	oc get pods -n $(NAMESPACE) | grep $(DEPLOYMENT_NAME)
	@echo "🌐 Access URL: https://uc3-frontend-route-uc3-applications.apps.ac3.rh-horizon.eu"

# Clean local images
clean:
	@echo "🧹 Cleaning local images..."
	podman rmi $(IMAGE_NAME) || true
	@echo "✅ Cleanup completed!"

# Show help
help:
	@echo "UC3 GUI Build and Deploy Commands:"
	@echo ""
	@echo "  make build    - Build the container image"
	@echo "  make push     - Push image to registry"
	@echo "  make deploy   - Restart OpenShift deployment"
	@echo "  make rebuild  - Full rebuild (build + push + deploy)"
	@echo "  make status   - Check deployment status"
	@echo "  make clean    - Remove local images"
	@echo "  make help     - Show this help message"
	@echo ""
	@echo "Default: make rebuild" 
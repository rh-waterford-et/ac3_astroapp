# UC3 pPXF Build and Deploy Makefile
IMAGE_NAME = quay.io/rh_et_wd/uc3-ppxf:latest
DOCKERFILE = Dockerfile
PROCESSOR_DEPLOYMENT = ucm-processor-deployment
NAMESPACE = uc3-applications

.PHONY: build push deploy rebuild status clean help

# Default target
all: rebuild

# Build the container image
build:
	@echo "🔨 Building pPXF container..."
	podman build --no-cache --platform linux/amd64 -t $(IMAGE_NAME) -f $(DOCKERFILE) .
	@echo "✅ pPXF build completed!"

# Push the container image to registry
push:
	@echo "📤 Pushing pPXF container to registry..."
	podman push $(IMAGE_NAME)
	@echo "✅ pPXF push completed!"

# Restart the processor deployment in OpenShift
deploy:
	@echo "🚀 Restarting pPXF deployment..."
	oc rollout restart deployment/$(PROCESSOR_DEPLOYMENT) -n $(NAMESPACE)
	@echo "✅ pPXF deployment restarted!"

# Full rebuild process: build + push + deploy
rebuild: build push deploy
	@echo "🎉 Full pPXF rebuild process completed!"

# Check deployment status
status:
	@echo "📊 Checking pPXF deployment status..."
	oc get pods -n $(NAMESPACE) | grep $(PROCESSOR_DEPLOYMENT)
	@echo "🔬 pPXF container ready for processing!"

# Clean local images
clean:
	@echo "🧹 Cleaning local pPXF images..."
	podman rmi $(IMAGE_NAME) || true
	@echo "✅ Cleanup completed!"

# Show help
help:
	@echo "UC3 pPXF Build and Deploy Commands:"
	@echo ""
	@echo "  make build    - Build the pPXF container image"
	@echo "  make push     - Push pPXF image to registry"
	@echo "  make deploy   - Restart OpenShift deployment"
	@echo "  make rebuild  - Full rebuild (build + push + deploy)"
	@echo "  make status   - Check deployment status"
	@echo "  make clean    - Remove local images"
	@echo "  make help     - Show this help message"
	@echo ""
	@echo "Default: make rebuild" 
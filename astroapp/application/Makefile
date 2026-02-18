# UC3 Backend Build and Deploy Makefile
IMAGE_NAME = quay.io/rh_et_wd/ac3/uc3-backend:watcher-fix
DOCKERFILE = containerfile
PRODUCER_DEPLOYMENT = ucm-producer-deployment
PROCESSOR_DEPLOYMENT = ucm-processor-deployment
NAMESPACE = uc3-applications

.PHONY: build push deploy rebuild status clean help

# Default target
all: rebuild

# Build the container image
build:
	@echo "🔨 Building backend container..."
	podman build --platform linux/amd64 -t $(IMAGE_NAME) -f $(DOCKERFILE) .
	@echo "✅ Build completed!"


# Push the container image to registry
push:
	@echo "📤 Pushing backend container to registry..."
	podman push $(IMAGE_NAME)
	@echo "✅ Push completed!"



# Restart the deployments in OpenShift
deploy:
	@echo "🚀 Restarting backend deployments..."
	oc rollout restart deployment/$(PRODUCER_DEPLOYMENT) -n $(NAMESPACE)
	oc rollout restart deployment/$(PROCESSOR_DEPLOYMENT) -n $(NAMESPACE)
	@echo "✅ Backend deployments restarted!"

# Full rebuild process: build + push + deploy
rebuild: build push deploy
	@echo "🎉 Full backend rebuild process completed!"

# Check deployment status
status:
	@echo "📊 Checking backend deployment status..."
	@echo "Producer components:"
	oc get pods -n $(NAMESPACE) | grep $(PRODUCER_DEPLOYMENT)
	@echo "Processor components:"
	oc get pods -n $(NAMESPACE) | grep $(PROCESSOR_DEPLOYMENT)
	@echo "🌐 Backend API URL: http://uc3-backend-service:8080"

# Show help
help:
	@echo "UC3 Backend Build and Deploy Commands:"
	@echo ""
	@echo "  make build    - Build the backend container image"
	@echo "  make push     - Push backend image to registry"
	@echo "  make deploy   - Restart OpenShift deployments"
	@echo "  make rebuild  - Full rebuild (build + push + deploy)"
	@echo "  make status   - Check deployment status"
	@echo "  make clean    - Remove local images"
	@echo "  make help     - Show this help message"
	@echo ""
	@echo "Default: make rebuild"

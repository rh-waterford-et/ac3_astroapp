# UC3 Experiment Tool Build and Deploy Makefile
IMAGE_NAME = quay.io/bcapper30/uc3-experiment-tool:latest
DOCKERFILE = Dockerfile
DEPLOYMENT = experiment-tool
NAMESPACE = uc3-applications

.PHONY: build push deploy rebuild status clean help

# Default target
all: rebuild

# Build the container image (context is ucm_app root, dockerfile is in experiment-tool/)
build:
	@echo "🔨 Building experiment tool container..."
	podman build --platform linux/amd64 -t $(IMAGE_NAME) -f $(DOCKERFILE) ../..
	@echo "✅ Build completed!"

# Push the container image to registry
push:
	@echo "📤 Pushing experiment tool container to registry..."
	podman push $(IMAGE_NAME)
	@echo "✅ Push completed!"

# Restart the deployment in OpenShift
deploy:
	@echo "🚀 Restarting experiment tool deployment..."
	oc rollout restart deployment/$(DEPLOYMENT) -n $(NAMESPACE)
	@echo "✅ Experiment tool deployment restarted!"

# Full rebuild process: build + push + deploy
rebuild: build push deploy
	@echo "🎉 Full experiment tool rebuild process completed!"

# Check deployment status
status:
	@echo "📊 Checking experiment tool deployment status..."
	oc get pods -n $(NAMESPACE) | grep $(DEPLOYMENT)

# Clean local images
clean:
	@echo "🧹 Cleaning local experiment tool images..."
	podman rmi $(IMAGE_NAME) || true
	@echo "✅ Cleanup completed!"

# Show help
help:
	@echo "UC3 Experiment Tool Build and Deploy Commands:"
	@echo ""
	@echo "  make build    - Build the experiment tool container image"
	@echo "  make push     - Push image to registry"
	@echo "  make deploy   - Restart OpenShift deployment"
	@echo "  make rebuild  - Full rebuild (build + push + deploy)"
	@echo "  make status   - Check deployment status"
	@echo "  make clean    - Remove local images"
	@echo "  make help     - Show this help message"
	@echo ""
	@echo "Note: Build must run from experiment-tool directory (changes to ../../ to access full context)"
	@echo "Default: make rebuild"


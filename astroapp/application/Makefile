# UC3 Backend Build and Deploy Makefile
IMAGE_NAME = quay.io/ryjenkin/uc3-backend:connector-mode
DOCKERFILE = containerfile
PRODUCER_DEPLOYMENT = ucm-producer-deployment
PROCESSOR_DEPLOYMENT = ucm-processor-deployment
NAMESPACE = uc3-applications

# HScaler Configuration
HSCALER_IMAGE = quay.io/rcarroll/ac3_uc3/hautoscaler:latest
HSCALER_MOCKAPI_IMAGE = quay.io/rcarroll/ac3_uc3/hscaler-mockapi:latest
HSCALER_DEPLOYMENT = predictor-client
HSCALER_MOCKAPI_DEPLOYMENT = model-api

.PHONY: build push deploy rebuild status clean help hscaler-build hscaler-push hscaler-deploy hscaler-rebuild hscaler-mockapi-build hscaler-mockapi-push hscaler-mockapi-deploy hscaler-mockapi-rebuild

# Default target
all: rebuild

# Build the container image
build:
	@echo "🔨 Building backend container..."
	sudo docker build --platform linux/amd64 -t $(IMAGE_NAME) -f $(DOCKERFILE) .
	@echo "✅ Build completed!"


# Push the container image to registry
push:
	@echo "📤 Pushing backend container to registry..."
	sudo docker push $(IMAGE_NAME)
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

# Clean local images
clean:
	@echo "🧹 Cleaning local backend images..."
	sudo docker rmi $(IMAGE_NAME) || true
	@echo "✅ Cleanup completed!"

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
	@echo "HScaler Commands:"
	@echo "  make hscaler-build        - Build hscaler predictor-client image"
	@echo "  make hscaler-push         - Push hscaler image to registry"
	@echo "  make hscaler-deploy       - Deploy hscaler to OpenShift"
	@echo "  make hscaler-rebuild      - Full hscaler rebuild"
	@echo "  make hscaler-mockapi-build - Build mock API image"
	@echo "  make hscaler-mockapi-push  - Push mock API image to registry"
	@echo ""
	@echo "Default: make rebuild"

# HScaler Build Targets
hscaler-build:
	@echo "🔨 Building hscaler predictor-client container..."
	cd pkg/hscaler && sudo docker build --platform linux/amd64 -t $(HSCALER_IMAGE) -f Dockerfile .
	@echo "✅ HScaler build completed!"

hscaler-push:
	@echo "📤 Pushing hscaler image to registry..."
	sudo docker push $(HSCALER_IMAGE)
	@echo "✅ HScaler push completed!"

hscaler-deploy:
	@echo "🚀 Deploying hscaler to OpenShift..."
	cd deployments && bash deploy_hscaler.sh
	@echo "✅ HScaler deployment completed!"

hscaler-rebuild: hscaler-build hscaler-push hscaler-deploy
	@echo "🎉 Full hscaler rebuild process completed!"

hscaler-mockapi-build:
	@echo "🔨 Building hscaler mock API container..."
	cd pkg/hscaler/mock_api && sudo docker build --platform linux/amd64 -t $(HSCALER_MOCKAPI_IMAGE) -f Dockerfile.mock_api .
	@echo "✅ HScaler Mock API build completed!"

hscaler-mockapi-push:
	@echo "📤 Pushing hscaler mock API image to registry..."
	sudo docker push $(HSCALER_MOCKAPI_IMAGE)
	@echo "✅ HScaler Mock API push completed!"

hscaler-mockapi-deploy:
	@echo "🚀 Deploying hscaler mock API to OpenShift..."
	cd deployments && bash deploy_hscaler_mockapi.sh
	@echo "✅ HScaler Mock API deployment completed!"

hscaler-mockapi-rebuild: hscaler-mockapi-build hscaler-mockapi-push hscaler-mockapi-deploy
	@echo "🎉 Full hscaler mock API rebuild process completed!"

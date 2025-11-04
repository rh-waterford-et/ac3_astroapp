#!/bin/bash

# ==========================
# UC3 Application Unified Deployment Script
# ==========================

set -e  # Exit on error

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default options
DEPLOY_MONITORING=false
DEPLOY_HSCALER=false
DEPLOY_HSCALER_MOCK=false
DRY_RUN=false

# Function to print colored output
print_header() {
    echo -e "${BLUE}===========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}===========================================${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_info() {
    echo -e "${YELLOW}📋 $1${NC}"
}

print_step() {
    echo -e "${GREEN}🚀 $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Function to apply or dry-run
apply_resource() {
    local file=$1
    local description=$2
    
    if [ ! -f "$file" ]; then
        print_error "File not found: $file"
        return 1
    fi
    
    print_info "Deploying $description ($file)"
    
    if [ "$DRY_RUN" = true ]; then
        echo "  [DRY RUN] Would apply: $file"
    else
        oc apply -f "$file"
    fi
}

# Usage function
usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Deploy UC3 Application to OpenShift/Kubernetes

OPTIONS:
    -m, --monitoring        Deploy with Prometheus & Grafana monitoring
    -h, --hscaler          Deploy HScaler predictor client
    -k, --mock             Deploy HScaler mock API (for testing)
    -a, --all              Deploy everything (monitoring + hscaler)
    -d, --dry-run          Show what would be deployed without applying
    --help                 Show this help message

EXAMPLES:
    $0                     # Basic deployment (core services only)
    $0 -m                  # Deploy with monitoring
    $0 -h -k               # Deploy with hscaler and mock API
    $0 --all               # Deploy everything
    $0 -d --all            # Dry run of full deployment

COMPONENTS:
    Core:
      - Namespace, RBAC, Storage
      - RabbitMQ, Redis
      - Producer, Processor
      - Frontend

    Monitoring (-m):
      - Prometheus
      - Grafana
      - Pod Metrics Exporter

    HScaler (-h):
      - Predictor Client
      - ServiceMonitor

    Mock API (-k):
      - Mock Prediction API
EOF
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -m|--monitoring)
            DEPLOY_MONITORING=true
            shift
            ;;
        -h|--hscaler)
            DEPLOY_HSCALER=true
            shift
            ;;
        -k|--mock)
            DEPLOY_HSCALER_MOCK=true
            shift
            ;;
        -a|--all)
            DEPLOY_MONITORING=true
            DEPLOY_HSCALER=true
            DEPLOY_HSCALER_MOCK=true
            shift
            ;;
        -d|--dry-run)
            DRY_RUN=true
            shift
            ;;
        --help)
            usage
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Main deployment
main() {
    print_header "UC3 Application Deployment"
    
    if [ "$DRY_RUN" = true ]; then
        print_info "DRY RUN MODE - No changes will be applied"
    fi
    
    echo ""
    print_info "Deployment Configuration:"
    echo "  Monitoring:       $DEPLOY_MONITORING"
    echo "  HScaler:          $DEPLOY_HSCALER"
    echo "  Mock API:         $DEPLOY_HSCALER_MOCK"
    echo ""
    
    # Step 1: Namespace
    print_step "Step 1/8: Creating namespace..."
    apply_resource "namespace.yaml" "Namespace"
    
    # Step 2: Secrets & ConfigMaps
    print_step "Step 2/8: Deploying configuration..."
    apply_resource "secrets.yaml" "Shared Secrets"
    apply_resource "configmap.yaml" "Shared ConfigMap"
    
    # Step 3: RBAC
    print_step "Step 3/8: Deploying RBAC resources..."
    apply_resource "rbac.yaml" "Service Accounts & Role Bindings"
    
    # Step 4: Storage
    print_step "Step 4/8: Deploying storage..."
    apply_resource "storage.yaml" "Persistent Volumes & Claims"
    
    # Step 5: Infrastructure Services
    print_step "Step 5/8: Deploying infrastructure services..."
    apply_resource "deployment_rabbitmq.yaml" "RabbitMQ"
    apply_resource "deployment_redis.yaml" "Redis"
    
    # Step 6: Backend Services
    print_step "Step 6/8: Deploying backend services..."
    apply_resource "deployment_producer.yaml" "Producer"
    apply_resource "deployment_processor.yaml" "Processor"
    apply_resource "service_backend.yaml" "Backend API Service"
    apply_resource "service_monitor_metrics.yaml" "Backend Metrics Service"
    
    # Step 7: Frontend
    print_step "Step 7/8: Deploying frontend..."
    apply_resource "deployment_frontend.yaml" "Frontend"
    
    # Step 8: Optional Components
    print_step "Step 8/8: Deploying optional components..."
    
    # Monitoring Stack
    if [ "$DEPLOY_MONITORING" = true ]; then
        print_info "Deploying monitoring stack..."
        apply_resource "cluster-monitoring-config.yaml" "Cluster Monitoring Config"
        apply_resource "prometheus_rules.yaml" "Prometheus Rules"
        apply_resource "deployment_prometheus.yaml" "Prometheus"
        apply_resource "deployment_grafana.yaml" "Grafana"
        apply_resource "deployment_pod_metrics_exporter.yaml" "Pod Metrics Exporter"
        print_success "Monitoring stack deployed"
    else
        print_info "Skipping monitoring stack (use -m to enable)"
    fi
    
    # HScaler
    if [ "$DEPLOY_HSCALER" = true ]; then
        print_info "Deploying HScaler..."
        apply_resource "hscaler.yaml" "HScaler Predictor Client"
        print_success "HScaler deployed"
    else
        print_info "Skipping HScaler (use -h to enable)"
    fi
    
    # Mock API
    if [ "$DEPLOY_HSCALER_MOCK" = true ]; then
        print_info "Deploying Mock API..."
        apply_resource "hscaler-mock.yaml" "Mock Prediction API"
        print_success "Mock API deployed"
    else
        print_info "Skipping Mock API (use -k to enable)"
    fi
    
    echo ""
    print_header "Deployment Complete!"
    
    if [ "$DRY_RUN" = false ]; then
        echo ""
        print_success "All resources deployed successfully!"
        echo ""
        print_info "Useful Commands:"
        echo "  📊 Check status:     oc get pods -n uc3-applications"
        echo "  🌐 Get routes:       oc get routes -n uc3-applications"
        echo "  📋 View logs:        oc logs -f <pod-name> -n uc3-applications"
        echo ""
        
        if [ "$DEPLOY_MONITORING" = true ]; then
            print_info "Monitoring Endpoints:"
            echo "  Prometheus:          oc get route prometheus-server -n uc3-applications"
            echo "  Grafana:             oc get route grafana -n uc3-applications"
            echo "  Pod Metrics:         oc get route pod-metrics-exporter -n uc3-applications"
            echo ""
        fi
        
        print_info "Frontend & API:"
        echo "  Frontend:            oc get route uc3-frontend-route -n uc3-applications"
        echo "  Backend API:         oc get route uc3-backend-api -n uc3-applications"
    fi
}

# Run main function
main


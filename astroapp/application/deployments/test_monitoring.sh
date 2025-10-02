#!/bin/bash

echo "🧪 Testing Prometheus Monitoring System"
echo "======================================"

NAMESPACE="uc3-applications"

# Function to check if a pod is running
check_pod() {
    local app_name=$1
    local pod_count=$(oc get pods -l app=$app_name -n $NAMESPACE --no-headers | wc -l)
    if [ $pod_count -gt 0 ]; then
        echo "✅ $app_name: $pod_count pod(s) running"
        return 0
    else
        echo "❌ $app_name: No pods running"
        return 1
    fi
}

# Function to check if a service is available
check_service() {
    local service_name=$1
    local port=$2
    if oc get svc $service_name -n $NAMESPACE >/dev/null 2>&1; then
        echo "✅ Service $service_name is available"
        return 0
    else
        echo "❌ Service $service_name is not available"
        return 1
    fi
}

# Function to test metrics endpoint
test_metrics_endpoint() {
    local service_name=$1
    local port=$2
    local path=${3:-/metrics}
    
    echo "Testing metrics endpoint for $service_name..."
    
    # Start port forward in background
    oc port-forward svc/$service_name $port:$port -n $NAMESPACE >/dev/null 2>&1 &
    local pf_pid=$!
    
    # Wait for port forward to be ready
    sleep 3
    
    # Test the endpoint
    if curl -s http://localhost:$port$path | grep -q "prometheus"; then
        echo "✅ $service_name metrics endpoint is working"
        result=0
    else
        echo "❌ $service_name metrics endpoint is not working"
        result=1
    fi
    
    # Clean up port forward
    kill $pf_pid 2>/dev/null
    return $result
}

echo "🔍 Checking pods..."
check_pod "prometheus-server"
check_pod "pod-metrics-exporter"
check_pod "grafana"
check_pod "ucm-processor"

echo ""
echo "🔍 Checking services..."
check_service "prometheus-server" "9090"
check_service "pod-metrics-exporter" "8080"
check_service "grafana" "3000"

echo ""
echo "🔍 Testing metrics endpoints..."
test_metrics_endpoint "prometheus-server" "9090" "/metrics"
test_metrics_endpoint "pod-metrics-exporter" "8080" "/metrics"

echo ""
echo "📊 Pod count metrics:"
echo "Total pods in namespace:"
oc get pods -n $NAMESPACE --no-headers | wc -l

echo ""
echo "Pods by status:"
oc get pods -n $NAMESPACE -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.phase}{"\n"}{end}' | sort | uniq -c

echo ""
echo "🎯 Quick access commands:"
echo "Prometheus UI: oc port-forward svc/prometheus-server 9090:9090 -n $NAMESPACE"
echo "Grafana UI: oc port-forward svc/grafana 3000:3000 -n $NAMESPACE"
echo "Pod Metrics: oc port-forward svc/pod-metrics-exporter 8080:8080 -n $NAMESPACE"

echo ""
echo "✅ Monitoring system test complete!"

#!/bin/bash

echo "Restarting EDC connector deployments in connector-test namespace..."

# Verify we're logged into OpenShift
if ! oc whoami &> /dev/null; then
    echo "Error: Not logged into OpenShift cluster."
    echo "Please run: oc login <your-cluster-url>"
    exit 1
fi

# Verify namespace exists
if ! oc get namespace connector-test &> /dev/null; then
    echo "Error: Namespace 'connector-test' not found."
    echo "Current cluster: $(oc cluster-info | head -1)"
    exit 1
fi

echo "Connected to cluster: $(oc cluster-info | head -1 | cut -d' ' -f6)"
echo "Current user: $(oc whoami)"

# Restart consumer deployment
echo "Restarting consumer..."
oc rollout restart deployment/consumer -n connector-test
if [ $? -ne 0 ]; then
  echo "Error restarting consumer deployment."
  exit 1
fi

# Restart provider deployment  
echo "Restarting provider..."
oc rollout restart deployment/provider -n connector-test
if [ $? -ne 0 ]; then
  echo "Error restarting provider deployment."
  exit 1
fi

# Restart transfer deployment
echo "Restarting transfer..."
oc rollout restart deployment/transfer -n connector-test
if [ $? -ne 0 ]; then
  echo "Error restarting transfer deployment."
  exit 1
fi

echo "All deployments restarted successfully!"
echo "Waiting for pods to be ready..."
oc wait --for=condition=ready pod -l app=consumer -n connector-test --timeout=60s
oc wait --for=condition=ready pod -l app=provider -n connector-test --timeout=60s  
oc wait --for=condition=ready pod -l app=transfer -n connector-test --timeout=60s
echo "All pods are ready!"
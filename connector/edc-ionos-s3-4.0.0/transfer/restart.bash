#!/bin/bash

# Cluster 1 login
oc login --insecure-skip-tls-verify --username=kubeadmin --password=ni4zc-Sw2m6-jnswB-2goot --server=https://api.ac3-cluster-1.rh-horizon.eu:6443
if [ $? -ne 0 ]; then
  echo "Error: Failed to log in to cluster-1. Exiting."
  exit 1
fi
oc rollout restart deployment/consumer -n connectors
if [ $? -ne 0 ]; then
  echo "Error restarting consumer on cluster-1."
fi

# Cluster 2 login
oc login --insecure-skip-tls-verify --username=kubeadmin --password=FLoY7-DuQi7-HRJJQ-WbKJP --server=https://api.ac3-cluster-2.rh-horizon.eu:6443
if [ $? -ne 0 ]; then
  echo "Error: Failed to log in to cluster-2. Exiting."
  exit 1
fi
oc rollout restart deployment/provider -n connectors
if [ $? -ne 0 ]; then
  echo "Error restarting provider on cluster-2."
fi
#!/bin/bash
# Deploy predictor client resources
kubectl apply -f k8s/deployment-mockapi.yaml
kubectl apply -f k8s/service-mockapi.yaml

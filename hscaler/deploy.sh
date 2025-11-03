#!/bin/bash
# Deploy predictor client resources
kubectl apply -f k8s/serviceaccount.yaml
kubectl apply -f k8s/rolebinding.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/service.yaml
kubectl apply -f k8s/servicemonitor.yaml
kubectl apply -f k8s/deployment.yaml
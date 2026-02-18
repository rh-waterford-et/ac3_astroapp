# AC3 Astronomical Data Processing System

A distributed system for processing astronomical spectral data using containerised scientific applications on Kubernetes/OpenShift.

## Project Structure

```
ac3_astroapp/
├── astroapp/           # Backend processing system and scientific applications
├── connector/          # Eclipse Dataspace Connector for federated data transfer
├── experiment-tool/    # Experiment orchestration and metrics collection
└── gui/                # React web interface
```

## Components

### astroapp

The core processing backend. Contains:
- Go-based microservices (producer, consumer, processor)
- RabbitMQ-based job distribution
- S3 storage integration
- Scientific processing applications (Starlight, pPXF, Voronoi)

### connector

Eclipse Dataspace Connector (EDC) implementation for IONOS S3. Enables federated data transfer between provider and consumer buckets with bidirectional synchronisation.

### experiment-tool

Go CLI tool for orchestrating processing experiments. Manages dataset submission, monitors S3 for completion, and collects timing metrics for analysis.

### gui

React-based web interface for:
- Dataset management and file uploads
- Processing pipeline monitoring
- Aladin sky viewer integration
- Result visualisation

## Deployment

Each component has its own deployment configuration for OpenShift/Kubernetes. See individual component READMEs for specific instructions.

### Prerequisites

- OpenShift/Kubernetes cluster
- Container registry access (quay.io)
- S3-compatible storage (IONOS Cloud)
- RabbitMQ

### Quick Start

1. Deploy infrastructure (namespace, volumes, RabbitMQ):
   ```bash
   cd astroapp/application/deployments
   oc apply -f namespace.yaml
   oc apply -f volume.yaml -f volumeclaim.yaml
   oc apply -f rabbitmq.yaml
   ```

2. Deploy backend services:
   ```bash
   oc apply -f producer.yaml
   oc apply -f processor.yaml
   oc apply -f consumer.yaml
   ```

3. Deploy frontend:
   ```bash
   cd ../../gui
   make rebuild
   ```

## Configuration

Environment variables are defined in deployment YAML files.

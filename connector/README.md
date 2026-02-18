# EDC Connector for IONOS S3

Eclipse Dataspace Connector (EDC) implementation for bidirectional file transfer between IONOS S3 buckets.

## Overview

Provides federated data sharing between provider and consumer endpoints using the EDC protocol. Monitors S3 buckets and triggers transfers for new files automatically.

## Directory Structure

```
connector/
└── edc-ionos-s3-4.0.0/
    ├── extensions/           # EDC extension modules (Java)
    │   ├── core-ionos-s3/    # Core S3 integration
    │   ├── data-plane-ionos-s3/  # Data plane for transfers
    │   └── provision-ionos-s3/   # S3 provisioning
    ├── launchers/            # Application launchers
    │   ├── dev/              # Development configs
    │   └── prod/             # Production configs
    ├── deployment/           # Deployment configurations
    │   ├── k8s/              # Kubernetes manifests
    │   ├── helm/             # Helm charts
    │   └── terraform/        # Infrastructure as code
    ├── transfer/             # Transfer automation scripts
    └── jsons/                # EDC API request templates
```

## Components

### Transfer Script

`transfer/trigger.py` - Python script that:
- Monitors provider and consumer S3 buckets
- Detects new files
- Triggers EDC transfer workflow
- Handles bidirectional synchronisation

### EDC Extensions

Java modules extending base EDC functionality for IONOS S3:
- Asset creation and management
- Contract negotiation
- Data plane transfers


## Deployment

### Kubernetes

```bash
cd deployment/k8s
oc apply -f vault.yaml
oc apply -f provider.yaml
oc apply -f consumer.yaml
oc apply -f transfer.yaml
```

### Helm

```bash
cd deployment/helm
helm install edc-ionos-s3 ./edc-ionos-s3
```

## Configuration

### S3 Credentials

Set in `transfer/trigger.py` or via environment variables:
- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `S3_ENDPOINT` (default: `https://s3.eu-central-1.ionoscloud.com`)

### Bucket Names

- `PROVIDER_BUCKET_NAME` - Source bucket
- `CONSUMER_BUCKET_NAME` - Destination bucket

## EDC API Usage

Request templates in `jsons/`:

| File | Purpose |
|------|---------|
| `asset.json` | Create data asset |
| `create-policy.json` | Define access policy |
| `create-contract.json` | Create contract definition |
| `fetch-catalog.json` | Query provider catalog |
| `contract-negotiation.json` | Initiate negotiation |
| `transfer.json` | Start data transfer |

## Transfer Workflow

1. Provider creates asset pointing to S3 object
2. Consumer fetches catalog from provider
3. Contract negotiation establishes agreement
4. Transfer request moves data between buckets
5. `trigger.py` automates this for new files

## Vault Integration

HashiCorp Vault stores secrets. See `hashicorp/` for setup.

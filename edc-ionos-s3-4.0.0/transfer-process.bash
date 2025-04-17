#!/bin/bash

# Configuration
API_KEY="password"
MANAGEMENT_URL="http://localhost:8182/management/v3"
CONSUMER_URL="http://localhost:9192/management/v3"
PROVIDER_PROTOCOL="http://provider:8282/protocol"

# Step 1: Create Asset
echo "Creating asset..."
ASSET_PAYLOAD='{
    "@context": {
        "@vocab": "https://w3id.org/edc/v0.0.1/ns/"
    },
    "@id": "asset-1",
    "properties": {
        "name": "Asset 1"
    },
    "dataAddress": {
        "type": "IonosS3",
        "bucketName": "test-provider",
        "blobName": "asset-1.txt"
    }
}'
echo "$ASSET_PAYLOAD" | curl -d @- -H "X-API-Key: $API_KEY" \
     -H "content-type: application/json" "$MANAGEMENT_URL/assets" -s

# Step 2: Create Policy
echo "Creating policy..."
POLICY_PAYLOAD='{
    "@context": {
        "edc": "https://w3id.org/edc/v0.0.1/ns/",
        "odrl": "http://www.w3.org/ns/odrl/2/"
    },
    "@id": "policy-1",
    "policy": {
        "@type": "odrl:Set",
        "odrl:assigner": {
            "@id": "provider"
        },
        "odrl:target": {
            "@id": "asset-1"
        },
        "odrl:permission": [],
        "odrl:prohibition": [],
        "odrl:obligation": []
    }
}'
echo "$POLICY_PAYLOAD" | curl -d @- -H "X-API-Key: $API_KEY" \
     -H "content-type: application/json" "$MANAGEMENT_URL/policydefinitions" -s

# Step 3: Create Contract Definition
echo "Creating contract definition..."
CONTRACT_PAYLOAD='{
    "@context": {
        "edc": "https://w3id.org/edc/v0.0.1/ns/"
    },
    "@id": "contract-1",
    "accessPolicyId": "policy-1",
    "contractPolicyId": "policy-1"
}'
echo "$CONTRACT_PAYLOAD" | curl -d @- -H "X-API-Key: $API_KEY" \
     -H "content-type: application/json" "$MANAGEMENT_URL/contractdefinitions" -s

# Step 4: Fetch Catalogue and Extract Offer ID
echo "Fetching catalogue..."
CATALOG_PAYLOAD='{
    "@context": {
        "edc": "https://w3id.org/edc/v0.0.1/ns/"
    },
    "counterPartyAddress": "'"$PROVIDER_PROTOCOL"'",
    "protocol": "dataspace-protocol-http"
}'
CATALOG_RESPONSE=$(echo "$CATALOG_PAYLOAD" | curl -X POST "$CONSUMER_URL/catalog/request" \
     --header "X-API-Key: $API_KEY" \
     --header "Content-Type: application/json" \
     -d @- -s)

OFFER_ID=$(echo "$CATALOG_RESPONSE" | jq -r '.["dcat:dataset"]["odrl:hasPolicy"]["@id"]')
if [ -z "$OFFER_ID" ]; then
    echo "Failed to extract offer ID from catalogue response. Full response:"
    echo "$CATALOG_RESPONSE" | jq .
    exit 1
fi
echo "Extracted offer ID: $OFFER_ID"

# Step 5: Initiate Contract Negotiation
echo "Initiating contract negotiation..."
NEGOTIATION_PAYLOAD=$(jq -n \
    --arg offerId "$OFFER_ID" \
    --arg provider "$PROVIDER_PROTOCOL" \
    '{
        "@context": {
            "@vocab": "https://w3id.org/edc/v0.0.1/ns/",
            "odrl": "http://www.w3.org/ns/odrl/2/"
        },
        "@type": "NegotiationInitiateRequestDto",
        "counterPartyAddress": $provider,
        "protocol": "dataspace-protocol-http",
        "offer": {
            "offerId": $offerId
        },
        "policy": {
            "@id": $offerId,
            "@type": "odrl:Offer",
            "odrl:assigner": {"@id": "provider"},
            "odrl:target": {"@id": "asset-1"},
            "odrl:permission": [],
            "odrl:prohibition": [],
            "odrl:obligation": []
        }
    }')

NEGOTIATION_RESPONSE=$(echo "$NEGOTIATION_PAYLOAD" | curl -X POST "$CONSUMER_URL/contractnegotiations" \
     --header "X-API-Key: $API_KEY" \
     --header "Content-Type: application/json" \
     -d @- -s)

NEGOTIATION_ID=$(echo "$NEGOTIATION_RESPONSE" | jq -r '.["@id"]')
if [ -z "$NEGOTIATION_ID" ]; then
    echo "Failed to extract negotiation ID. Response:"
    echo "$NEGOTIATION_RESPONSE" | jq .
    exit 1
fi
echo "Extracted negotiation ID: $NEGOTIATION_ID"

# Step 6: Poll Negotiation Status Until Finalized
echo "Polling negotiation status..."
while true; do
    STATUS_RESPONSE=$(curl -X GET "$CONSUMER_URL/contractnegotiations/$NEGOTIATION_ID" \
        --header "X-API-Key: $API_KEY" \
        --header "Content-Type: application/json" \
        -s)
    STATE=$(echo "$STATUS_RESPONSE" | jq -r '.state')
    if [ "$STATE" = "FINALIZED" ]; then
        CONTRACT_AGREEMENT_ID=$(echo "$STATUS_RESPONSE" | jq -r '.contractAgreementId')
        echo "Contract finalized. Agreement ID: $CONTRACT_AGREEMENT_ID"
        break
    elif [ "$STATE" = "TERMINATED" ]; then
        echo "Negotiation terminated. Response:"
        echo "$STATUS_RESPONSE" | jq .
        exit 1
    fi
    echo "Current state: $STATE. Waiting..."
    sleep 2
done

# Step 7: Initiate Transfer
echo "Initiating transfer..."
KEY_NAME=$(uuidgen)
TRANSFER_PAYLOAD=$(jq -n \
    --arg contractId "$CONTRACT_AGREEMENT_ID" \
    --arg provider "$PROVIDER_PROTOCOL" \
    --arg keyName "$KEY_NAME" \
    '{
        "@context": {
            "edc": "https://w3id.org/edc/v0.0.1/ns/"
        },
        "@type": "TransferRequestDto",
        "connectorId": "provider",
        "counterPartyAddress": $provider,
        "protocol": "dataspace-protocol-http",
        "contractId": $contractId,
        "assetId": "asset-1",
        "transferType": "IonosS3-PUSH",
        "dataDestination": {
            "type": "IonosS3",
            "bucketName": "test-consumer",
            "keyName": $keyName
        }
    }')

TRANSFER_RESPONSE=$(echo "$TRANSFER_PAYLOAD" | curl -X POST "$CONSUMER_URL/transferprocesses" \
     --header "Content-Type: application/json" \
     --header "X-API-Key: $API_KEY" \
     -d @- -s)

TRANSFER_ID=$(echo "$TRANSFER_RESPONSE" | jq -r '.["@id"]')
if [ -z "$TRANSFER_ID" ]; then
    echo "Failed to initiate transfer. Response:"
    echo "$TRANSFER_RESPONSE" | jq .
    exit 1
fi
echo "Transfer process initiated successfully with ID: $TRANSFER_ID"
echo "Transfer response:"
echo "$TRANSFER_RESPONSE" | jq .
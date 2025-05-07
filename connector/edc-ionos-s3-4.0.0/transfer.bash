#!/bin/bash

# Local transfer
# API_KEY="password"
# PROVIDER_MGMT_URL="http://localhost:8182/management/v3"
# CONSUMER_MGMT_URL="http://localhost:9192/management/v3"
# PROVIDER_PROTOCOL="http://provider:8282/protocol"

# Multi-cluster 
# API_KEY="password"
# PROVIDER_MGMT_URL="https://provider-management-connectors.apps.ac3-cluster-2.rh-horizon.eu/management/v3"
# CONSUMER_MGMT_URL="https://consumer-management-connectors.apps.ac3-cluster-1.rh-horizon.eu/management/v3"
# PROVIDER_PROTOCOL="http://provider-protocol-connectors.apps.ac3-cluster-2.rh-horizon.eu/protocol"

# Check if asset name is provided as an argument
if [ -z "$1" ]; then
    echo "Error: No asset name provided. Usage: $0 <asset_name>" >&2
    exit 1
fi

# Configuration
ASSET_NAME="$1"
POLICY_NAME="${ASSET_NAME}"
CONTRACT_NAME="${ASSET_NAME}"
API_KEY="password"
PROVIDER_MGMT_URL="https://provider-management-connectors.apps.ac3-cluster-2.rh-horizon.eu/management/v3"
CONSUMER_MGMT_URL="https://consumer-management-connectors.apps.ac3-cluster-1.rh-horizon.eu/management/v3"
PROVIDER_PROTOCOL="http://provider-protocol-connectors.apps.ac3-cluster-2.rh-horizon.eu/protocol"

# Common curl options
CURL_OPTS=(--insecure -s -H "X-API-Key: $API_KEY" -H "Content-Type: application/json")

# Helper function for API calls
make_api_call() {
    local method=$1 url=$2 payload=$3
    if [ "$method" = "POST" ]; then
        echo "$payload" | curl "${CURL_OPTS[@]}" -X POST "$url" -d @-
    else
        curl "${CURL_OPTS[@]}" -X GET "$url"
    fi
}

# Helper function to extract ID
extract_id() {
    local response=$1 fallback_id=$2
    local id
    id=$(echo "$response" | jq -r '.["@id"]' 2>/dev/null)
    echo "${id:-$fallback_id}"
}

# Helper function to poll status
poll_status() {
    local type=$1 id=$2 url=$3 success_state=$4
    while true; do
        local response state
        response=$(make_api_call GET "$url/$id")
        state=$(echo "$response" | jq -r '.state')
        if [ "$state" = "$success_state" ]; then
            echo "$response"
            return 0
        elif [ "$state" = "TERMINATED" ]; then
            echo "$type terminated. Response:" >&2
            echo "$response" | jq . >&2
            exit 1
        fi
        echo "Current $type state: $state. Waiting..." >&2
        sleep 2
    done
}

# Step 1: Create Asset
echo "Creating asset..."
ASSET_PAYLOAD=$(jq -n \
    --arg asset_name "$ASSET_NAME" \
    '{
        "@context": {"@vocab": "https://w3id.org/edc/v0.0.1/ns/"},
        "@id": $asset_name,
        "properties": {"name": $asset_name},
        "dataAddress": {"type": "IonosS3", "bucketName": "test-provider", "blobName": $asset_name}
}')
ASSET_RESPONSE=$(make_api_call POST "$PROVIDER_MGMT_URL/assets" "$ASSET_PAYLOAD")
echo "$ASSET_RESPONSE"
ASSET_ID=$(extract_id "$ASSET_RESPONSE" "$ASSET_NAME")
# Check if response is valid JSON and contains @id
if echo "$ASSET_RESPONSE" | jq -e '.["@id"]' >/dev/null 2>&1; then
    echo "Asset created successfully with ID: $ASSET_ID" >&2
else
    echo "Asset creation failed, using asset: $ASSET_ID" >&2
fi

# Step 2: Create Policy
echo "Creating policy..."
POLICY_PAYLOAD=$(jq -n \
    --arg asset_name "$ASSET_NAME" \
    --arg policy_name "$POLICY_NAME" \
'{
    "@context": {"edc": "https://w3id.org/edc/v0.0.1/ns/", "odrl": "http://www.w3.org/ns/odrl/2/"},
    "@id": $policy_name,
    "policy": {
        "@type": "odrl:Set",
        "odrl:assigner": {"@id": "provider"},
        "odrl:target": {"@id": $asset_name},
        "odrl:permission": [],
        "odrl:prohibition": [],
        "odrl:obligation": []
    }
}')
POLICY_RESPONSE=$(make_api_call POST "$PROVIDER_MGMT_URL/policydefinitions" "$POLICY_PAYLOAD")
echo "$POLICY_RESPONSE"
POLICY_ID=$(extract_id "$POLICY_RESPONSE" "$POLICY_NAME")
# Check if response is valid JSON and contains @id
if echo "$POLICY_RESPONSE" | jq -e '.["@id"]' >/dev/null 2>&1; then
    echo "Policy created successfully with ID: $POLICY_ID" >&2
else
    echo "Policy creation failed, using policy: $POLICY_ID" >&2
fi

# Step 3: Create Contract Definition
echo "Creating contract definition..."
CONTRACT_PAYLOAD=$(jq -n \
    --arg contract_name "$CONTRACT_NAME" \
    --arg policy_name "$POLICY_NAME" \
'{
    "@context": {"edc": "https://w3id.org/edc/v0.0.1/ns/"},
    "@id": $contract_name,
    "accessPolicyId": $policy_name,
    "contractPolicyId": $policy_name
}')
CONTRACT_RESPONSE=$(make_api_call POST "$PROVIDER_MGMT_URL/contractdefinitions" "$CONTRACT_PAYLOAD")
echo "$CONTRACT_RESPONSE"
CONTRACT_ID=$(extract_id "$CONTRACT_RESPONSE" "$CONTRACT_NAME")
# Check if response is valid JSON and contains @id
if echo "$CONTRACT_RESPONSE" | jq -e '.["@id"]' >/dev/null 2>&1; then
    echo "Contract created successfully with ID: $CONTRACT_ID" >&2
else
    echo "Contract creation failed, using contract: $CONTRACT_ID" >&2
fi

# Step 4: Fetch Catalogue
echo "Fetching catalogue..."
CATALOG_PAYLOAD='{
    "@context": {"edc": "https://w3id.org/edc/v0.0.1/ns/"},
    "counterPartyAddress": "'"$PROVIDER_PROTOCOL"'",
    "protocol": "dataspace-protocol-http"
}'
CATALOG_RESPONSE=$(make_api_call POST "$CONSUMER_MGMT_URL/catalog/request" "$CATALOG_PAYLOAD")
OFFER_ID=$(echo "$CATALOG_RESPONSE" | jq -r --arg asset_name "$ASSET_NAME" '
    (.["dcat:dataset"] | if type == "array" then .[] else . end) |
    select(.["@id"] == $asset_name) |
    (.["odrl:hasPolicy"] | if type == "array" then .[] else . end) |
    select(.["odrl:target"]["@id"] == $asset_name) |
    .["@id"]' 2>/dev/null)
if [ -z "$OFFER_ID" ]; then
    echo "Failed to extract offer ID for asset: $ASSET_NAME. Full response:" >&2
    echo "$CATALOG_RESPONSE" | jq . 2>/dev/null || echo "Invalid JSON: $CATALOG_RESPONSE" >&2
    exit 1
fi
echo "Extracted offer ID: $OFFER_ID"

# Step 5: Initiate Contract Negotiation
echo "Initiating contract negotiation..."
NEGOTIATION_PAYLOAD=$(jq -n \
    --arg asset_name "$ASSET_NAME" \
    --arg offerId "$OFFER_ID" \
    --arg provider "$PROVIDER_PROTOCOL" \
    '{
        "@context": {"@vocab": "https://w3id.org/edc/v0.0.1/ns/", "odrl": "http://www.w3.org/ns/odrl/2/"},
        "@type": "NegotiationInitiateRequestDto",
        "counterPartyAddress": $provider,
        "protocol": "dataspace-protocol-http",
        "offer": {"offerId": $offerId},
        "policy": {
            "@id": $offerId,
            "@type": "odrl:Offer",
            "odrl:assigner": {"@id": "provider"},
            "odrl:target": {"@id": $asset_name},
            "odrl:permission": [],
            "odrl:prohibition": [],
            "odrl:obligation": []
        }
    }')
NEGOTIATION_RESPONSE=$(make_api_call POST "$CONSUMER_MGMT_URL/contractnegotiations" "$NEGOTIATION_PAYLOAD")
echo "$NEGOTIATION_RESPONSE"
NEGOTIATION_ID=$(extract_id "$NEGOTIATION_RESPONSE" "")
if [ -z "$NEGOTIATION_ID" ]; then
    echo "Failed to extract negotiation ID. Response:" >&2
    echo "$NEGOTIATION_RESPONSE" | jq . 2>/dev/null || echo "Invalid JSON: $NEGOTIATION_RESPONSE" >&2
    exit 1
fi
echo "Extracted negotiation ID: $NEGOTIATION_ID"

# Step 6: Poll Negotiation Status
echo "Polling negotiation status..."
NEGOTIATION_STATUS=$(poll_status "negotiation" "$NEGOTIATION_ID" "$CONSUMER_MGMT_URL/contractnegotiations" "FINALIZED")
CONTRACT_AGREEMENT_ID=$(echo "$NEGOTIATION_STATUS" | jq -r '.contractAgreementId')
echo "Contract finalized. Agreement ID: $CONTRACT_AGREEMENT_ID"

# Step 7: Initiate Transfer
echo "Initiating transfer..."
KEY_NAME=$(uuidgen)
TRANSFER_PAYLOAD=$(jq -n \
    --arg asset_name "$ASSET_NAME" \
    --arg contractId "$CONTRACT_AGREEMENT_ID" \
    --arg provider "$PROVIDER_PROTOCOL" \
    --arg keyName "$KEY_NAME" \
    '{
        "@context": {"edc": "https://w3id.org/edc/v0.0.1/ns/"},
        "@type": "TransferRequestDto",
        "connectorId": "provider",
        "counterPartyAddress": $provider,
        "protocol": "dataspace-protocol-http",
        "contractId": $contractId,
        "assetId": $asset_name,
        "transferType": "IonosS3-PUSH",
        "dataDestination": {
            "type": "IonosS3",
            "bucketName": "test-consumer",
            "path": "batch-01/",
            "keyName": $keyName
        }
    }')
TRANSFER_RESPONSE=$(make_api_call POST "$CONSUMER_MGMT_URL/transferprocesses" "$TRANSFER_PAYLOAD")
echo "$TRANSFER_RESPONSE"
TRANSFER_ID=$(extract_id "$TRANSFER_RESPONSE" "")
if [ -z "$TRANSFER_ID" ]; then
    echo "Failed to initiate transfer. Response:" >&2
    echo "$TRANSFER_RESPONSE" | jq . 2>/dev/null || echo "Invalid JSON: $TRANSFER_RESPONSE" >&2
    exit 1
fi
echo "Transfer process initiated successfully with ID: $TRANSFER_ID"

# Step 8: Poll Transfer Status
echo "Polling transfer status..."
poll_status "transfer" "$TRANSFER_ID" "$CONSUMER_MGMT_URL/transferprocesses" "COMPLETED" >/dev/null
echo "Transfer completed successfully."

# Step 9: Deprovision Transfer
echo "Deprovisioning transfer..."
DEPROVISION_RESPONSE=$(curl "${CURL_OPTS[@]}" -X POST "$CONSUMER_MGMT_URL/transferprocesses/$TRANSFER_ID/deprovision")
echo -e "$DEPROVISION_RESPONSE\nDone."
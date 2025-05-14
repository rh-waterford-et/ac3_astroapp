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
if [ $# -lt 3 ]; then
    echo "[$2 -> $3] Error: Missing arguments. Usage: $0 <asset_name> <provider_bucket> <consumer_bucket>" >&2
    exit 1
fi

# Configuration
ASSET_NAME="$1"
POLICY_NAME="$1-policy"
CONTRACT_NAME="$1-contract"
API_KEY="password"
PROVIDER_MGMT_URL="https://provider-management-connectors.apps.ac3-cluster-2.rh-horizon.eu/management/v3"
CONSUMER_MGMT_URL="https://consumer-management-connectors.apps.ac3-cluster-1.rh-horizon.eu/management/v3"
CONSUMER_PROTOCOL="http://consumer-protocol-connectors.apps.ac3-cluster-1.rh-horizon.eu/protocol"
PROVIDER_PROTOCOL="http://provider-protocol-connectors.apps.ac3-cluster-2.rh-horizon.eu/protocol"
BEARER_TOKEN="eyJ0eXAiOiJKV1QiLCJraWQiOiJiNzlkODE3OS02MmFhLTRkMGYtODU0Zi1lMzQyMmNmYzE1MTciLCJhbGciOiJSUzI1NiJ9.eyJpc3MiOiJpb25vc2Nsb3VkIiwiaWF0IjoxNzQ0ODE0NzE0LCJjbGllbnQiOiJVU0VSIiwiaWRlbnRpdHkiOnsicm9sZSI6ImFkbWluIiwiY29udHJhY3ROdW1iZXIiOjMyNDM5MjQ1LCJpc1BhcmVudCI6ZmFsc2UsInByaXZpbGVnZXMiOlsiREFUQV9DRU5URVJfQ1JFQVRFIiwiU05BUFNIT1RfQ1JFQVRFIiwiSVBfQkxPQ0tfUkVTRVJWRSIsIk1BTkFHRV9EQVRBUExBVEZPUk0iLCJBQ0NFU1NfQUNUSVZJVFlfTE9HIiwiUENDX0NSRUFURSIsIkFDQ0VTU19TM19PQkpFQ1RfU1RPUkFHRSIsIkJBQ0tVUF9VTklUX0NSRUFURSIsIkNSRUFURV9JTlRFUk5FVF9BQ0NFU1MiLCJLOFNfQ0xVU1RFUl9DUkVBVEUiLCJGTE9XX0xPR19DUkVBVEUiLCJBQ0NFU1NfQU5EX01BTkFHRV9NT05JVE9SSU5HIiwiQUNDRVNTX0FORF9NQU5BR0VfQ0VSVElGSUNBVEVTIiwiQUNDRVNTX0FORF9NQU5BR0VfTE9HR0lORyIsIk1BTkFHRV9EQkFBUyIsIkFDQ0VTU19BTkRfTUFOQUdFX0ROUyIsIk1BTkFHRV9SRUdJU1RSWSIsIkFDQ0VTU19BTkRfTUFOQUdFX0NETiIsIkFDQ0VTU19BTkRfTUFOQUdFX1ZQTiIsIkFDQ0VTU19BTkRfTUFOQUdFX0FQSV9HQVRFV0FZIiwiQUNDRVNTX0FORF9NQU5BR0VfTkdTIiwiQUNDRVNTX0FORF9NQU5BR0VfS0FBUyIsIkFDQ0VTU19BTkRfTUFOQUdFX05FVFdPUktfRklMRV9TVE9SQUdFIiwiQUNDRVNTX0FORF9NQU5BR0VfQUlfTU9ERUxfSFVCIiwiQ1JFQVRFX05FVFdPUktfU0VDVVJJVFlfR1JPVVBTIiwiQUNDRVNTX0FORF9NQU5BR0VfSUFNX1JFU09VUkNFUyJdLCJ1dWlkIjoiZWUxMjdlOGItOTMwNi00OTFhLTk1MDAtOGE0MWE4MjQ3YzhlIiwicmVzZWxsZXJJZCI6MSwicmVnRG9tYWluIjoiaW9ub3MuZGUifSwiZXhwIjoxNzc2MzUwNzE0fQ.K8axaTD53Mr1ymn5X9dJhI8DlAu2cO_IgknpBtFjs9tOPkSGPwKc8ImTfyiJbiA3UemXAoLQ4W67KUf7SR7E8dOJWRbnCdv4OAMk1MGDcQ58gRPlwQ-ztS6sHN0PWQsB7Os4IILkgDsUKMvDNj_FWWBFy3910ztnkqvrgmexQJcgOsm0VumxmBWntRnNPpaudl8GK2wJip-fn8iNOwss9qz_IqlN7PilmdYLzVMIk9KNIRUWnjLj2wdbYmJaS4tDkUFcm5y17pICx4hyaBNo19pN2B7FWEvDsWGK_avCsxkUiI5mpiYnEW5dIwPRMr_LsiQleFxUZnY46xa5ekttbA"
KEEP_ID="f72ce80a-f4f8-43e2-b4e1-224258895a09"
ACCESS_KEYS_URL="https://s3.ionos.com/accesskeys"

# Common curl options
CURL_OPTS=(--insecure -s -H "X-API-Key: $API_KEY" -H "Content-Type: application/json")

# Helper function for API calls
make_api_call() {
    local method=$1 url=$2 payload=$3
    if [ "$method" = "POST" ]; then
        echo "$payload" | curl "${CURL_OPTS[@]}" -X POST "$url" -d @-
    elif [ "$method" = "DELETE" ]; then
        curl "${CURL_OPTS[@]}" -X DELETE "$url"
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

# Helper function to poll deprovision status
poll_deprovision_status() {
    local id=$1 url=$2 success_state=$3
    while true; do
        local response state
        response=$(curl "${CURL_OPTS[@]}" -X GET "$url/$id/state")
        state=$(echo "$response" | jq -r '.state')
        if [ "$state" = "$success_state" ]; then
            echo "$response"
            return 0
        elif [ "$state" = "FAILED" ]; then
            echo "Deprovisioning failed. Response:" >&2
            echo "$response" | jq . >&2
            exit 1
        fi
        echo "Current deprovisioning state: $state. Waiting..." >&2
        sleep 2
    done
}

# Function to delete a resource (asset, policy, or contract)
function delete_resource {
  local resource_type="$1"
  local resource_id="$2"
  local endpoint=""
  local resource_name_for_output="" #used in the echo

  case "$resource_type" in
    "asset")
      endpoint="/assets/$resource_id"
      resource_name_for_output="asset with ID: $resource_id"
      ;;
    "policy")
      endpoint="/policydefinitions/$resource_id"
      resource_name_for_output="policy: $resource_id"
      ;;
    "contract")
      endpoint="/contractdefinitions/$resource_id"
      resource_name_for_output="contract definition: $resource_id"
      ;;
    *)
      echo "Error: Invalid resource type: $resource_type" >&2
      return 1
      ;;
  esac

  echo "Deleting $resource_name_for_output..." >&2
  local delete_response=$(make_api_call DELETE "$CONSUMER_MGMT_URL$endpoint" "")
  if [ -z "$delete_response" ]; then
    echo "$resource_name_for_output deleted successfully." >&2
  else
    echo "[$2 -> $3] Failed to delete $resource_name_for_output. Response:" >&2
    echo "$delete_response" | jq . 2>/dev/null || echo "Invalid JSON: $delete_response" >&2
    return 1
  fi
  return 0
}

# Get all access keys
response=$(curl -s -H "Authorization: Bearer $BEARER_TOKEN" "$ACCESS_KEYS_URL")
ids_to_delete=$(echo "$response" | jq -r --arg keep_id "$KEEP_ID" '.items[] | select(.id != $keep_id) | .id')


keys=($ids_to_delete) # Ensure $ids_to_delete is treated as an array


if [[ ${#keys[@]} -gt 4 ]]; then
    second_id="${keys[1]}" # Access the element at index 1 (the second element)
    echo "[$2 -> $3] Deleting access key: $second_id"
    curl -s -X DELETE -H "Authorization: Bearer $BEARER_TOKEN" "$ACCESS_KEYS_URL/$second_id"
else
    echo "[$2 -> $3] The list of IDs to delete has 4 or fewer elements."
fi

# if [[ ${#keys[@]} -ge 2 ]]; then
#   second_id="${keys[1]}" # Access the element at index 1 (the second element)
#   echo "Deleting access key: $second_id"
#   curl -s -X DELETE -H "Authorization: Bearer $BEARER_TOKEN" "$ACCESS_KEYS_URL/$second_id"
# else
#   echo "The list of IDs to delete has fewer than two elements."
# fi

# Step 1: Create Asset
echo "Creating asset..."
ASSET_PAYLOAD=$(jq -n \
    --arg asset_name "$ASSET_NAME" \
    --arg bucket "$2" \
    '{
        "@context": {"@vocab": "https://w3id.org/edc/v0.0.1/ns/"},
        "@id": $asset_name,
        "properties": {"name": $asset_name},
        "dataAddress": {"type": "IonosS3", "bucketName": $bucket, "blobName": $asset_name}
}')
ASSET_RESPONSE=$(make_api_call POST "$PROVIDER_MGMT_URL/assets" "$ASSET_PAYLOAD")
echo "$ASSET_RESPONSE"
ASSET_ID=$(extract_id "$ASSET_RESPONSE" "$ASSET_NAME")
# Check if response is valid JSON and contains @id
if echo "$ASSET_RESPONSE" | jq -e '.["@id"]' >/dev/null 2>&1; then
    echo "[$2 -> $3] Asset created successfully with ID: $ASSET_ID" >&2
else
    echo "[$2 -> $3] Asset creation failed, using asset: $ASSET_ID" >&2
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
    echo "[$2 -> $3] Policy created successfully with ID: $POLICY_ID" >&2
else
    echo "[$2 -> $3] Policy creation failed, using policy: $POLICY_ID" >&2
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
    echo "[$2 -> $3] Contract created successfully with ID: $CONTRACT_ID" >&2
else
    echo "[$2 -> $3] Contract creation failed, using contract: $CONTRACT_ID" >&2
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
        delete_resource "asset" "$ASSET_ID" 2>&1 | tee /dev/stderr
        echo "[$2 -> $3] Failed to extract offer ID for asset: $ASSET_NAME." >&2
        logger -t transfer_script "Fetch Catalogue failed for asset $ASSET_NAME"
        #echo "$CATALOG_RESPONSE" | jq . 2>/dev/null || echo "Invalid JSON: $CATALOG_RESPONSE" >&2
        exit 100
    fi
    echo "[$2 -> $3] Extracted offer ID: $OFFER_ID"

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
    echo "[$2 -> $3] Failed to extract negotiation ID. Response:" >&2
    echo "$NEGOTIATION_RESPONSE" | jq . 2>/dev/null || echo "Invalid JSON: $NEGOTIATION_RESPONSE" >&2
    exit 1
fi
echo "Extracted negotiation ID: $NEGOTIATION_ID"

# Step 6: Poll Negotiation Status
echo "Polling negotiation status..."
NEGOTIATION_STATUS=$(poll_status "negotiation" "$NEGOTIATION_ID" "$CONSUMER_MGMT_URL/contractnegotiations" "FINALIZED")
CONTRACT_AGREEMENT_ID=$(echo "$NEGOTIATION_STATUS" | jq -r '.contractAgreementId')
echo "[$2 -> $3] Contract finalized. Agreement ID: $CONTRACT_AGREEMENT_ID"

# Step 7: Initiate Transfer
echo "[$2 -> $3] Initiating transfer..."
KEY_NAME=$(uuidgen)
TRANSFER_PAYLOAD=$(jq -n \
    --arg asset_name "$ASSET_NAME" \
    --arg contractId "$CONTRACT_AGREEMENT_ID" \
    --arg provider "$PROVIDER_PROTOCOL" \
    --arg keyName "$KEY_NAME" \
    --arg bucket "$3" \
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
            "bucketName": $bucket,
            "keyName": $keyName
        }
    }')
TRANSFER_RESPONSE=$(make_api_call POST "$CONSUMER_MGMT_URL/transferprocesses" "$TRANSFER_PAYLOAD")
echo "$TRANSFER_RESPONSE"
TRANSFER_ID=$(extract_id "$TRANSFER_RESPONSE" "")
if [ -z "$TRANSFER_ID" ]; then
    echo "[$2 -> $3] Failed to initiate transfer. Response:" >&2
    echo "$TRANSFER_RESPONSE" | jq . 2>/dev/null || echo "Invalid JSON: $TRANSFER_RESPONSE" >&2
    exit 1
fi
echo "[$2 -> $3] Transfer process initiated successfully with ID: $TRANSFER_ID"

# Step 8: Poll Transfer Status
echo "[$2 -> $3] Polling transfer status..."
poll_status "transfer" "$TRANSFER_ID" "$CONSUMER_MGMT_URL/transferprocesses" "COMPLETED" >/dev/null
echo "Transfer completed successfully."

# Step 9: Deprovision Transfer
echo "[$2 -> $3] Deprovisioning transfer..."
DEPROVISION_RESPONSE=$(curl "${CURL_OPTS[@]}" -X POST "$CONSUMER_MGMT_URL/transferprocesses/$TRANSFER_ID/deprovision")
echo -e "$DEPROVISION_RESPONSE"

# Step 10: Poll Deprovision Status
echo "[$2 -> $3] Polling deprovision status for transfer ID: $TRANSFER_ID..."
poll_deprovision_status "$TRANSFER_ID" "$CONSUMER_MGMT_URL/transferprocesses" "DEPROVISIONED" >/dev/null
echo "[$2 -> $3] Deprovisioning completed successfully."

delete_resource "asset" "$ASSET_NAME" 2>&1
delete_resource "policy" "$POLICY_NAME" 2>&1
delete_resource "contract" "$CONTRACT_NAME" 2>&1

# for id in $ids_to_delete; do
#   echo "Deleting access key: $id"
#   curl -s -X DELETE -H "Authorization: Bearer $BEARER_TOKEN" "$ACCESS_KEYS_URL/$id"
# done

echo "[$2 -> $3] Done"
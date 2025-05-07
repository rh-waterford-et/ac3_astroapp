#!/bin/bash

# Ensure the resources directory exists
mkdir -p /app/resources

# Write environment variables to config.properties
cat <<EOF > /app/resources/config.properties
edc.participant.id=$EDC_PARTICIPANT_ID
web.http.port=$WEB_HTTP_PORT
web.http.path=$WEB_HTTP_PATH
web.http.management.port=$WEB_HTTP_MANAGEMENT_PORT
web.http.management.path=$WEB_HTTP_MANAGEMENT_PATH
web.http.protocol.port=$WEB_HTTP_PROTOCOL_PORT
web.http.protocol.path=$WEB_HTTP_PROTOCOL_PATH
web.http.control.port=$WEB_HTTP_CONTROL_PORT
web.http.control.path=$WEB_HTTP_CONTROL_PATH
web.http.public.port=$WEB_HTTP_PUBLIC_PORT
web.http.public.path=$WEB_HTTP_PUBLIC_PATH
edc.dsp.callback.address=$EDC_DSP_CALLBACK_ADDRESS
edc.dataplane.token.validation.endpoint=$EDC_DATAPLANE_TOKEN_VALIDATION_ENDPOINT
edc.dataplane.api.public.baseurl=$EDC_DATAPLANE_API_PUBLIC_BASEURL
edc.dsp.http.enabled=$EDC_DSP_HTTP_ENABLED
edc.api.auth.key=$EDC_API_AUTH_KEY
edc.transfer.proxy.token.signer.privatekey.alias=$EDC_TRANSFER_PROXY_TOKEN_SIGNER_PRIVATEKEY_ALIAS
edc.transfer.proxy.token.verifier.publickey.alias=$EDC_TRANSFER_PROXY_TOKEN_VERIFIER_PUBLICKEY_ALIAS
edc.vault.hashicorp.url=$EDC_VAULT_HASHICORP_URL
edc.vault.hashicorp.token=$EDC_VAULT_HASHICORP_TOKEN
edc.vault.hashicorp.timeout.seconds=$EDC_VAULT_HASHICORP_TIMEOUT_SECONDS
edc.ionos.access.key=$EDC_IONOS_ACCESS_KEY
edc.ionos.secret.key=$EDC_IONOS_SECRET_KEY
edc.ionos.endpoint.region=$EDC_IONOS_ENDPOINT_REGION
edc.ionos.token=$EDC_IONOS_TOKEN
EOF

# Start the Java application, respecting JAVA_TOOL_OPTIONS from the deployment
exec java $JAVA_TOOL_OPTIONS -jar dataspace-connector.jar
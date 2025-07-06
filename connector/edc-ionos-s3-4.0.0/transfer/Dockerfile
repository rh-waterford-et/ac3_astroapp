# Use a lightweight Python base image
FROM python:3.9-slim

# Install dependencies for Python and transfer.bash, and tools for oc
RUN apt-get update && apt-get install -y \
    curl \
    jq \
    uuid-runtime \
    wget \
    tar \
    gzip \
    && rm -rf /var/lib/apt/lists/* \
    && pip install --no-cache-dir boto3 requests uuid urllib3

# Download and install the OpenShift client (oc)
ARG OC_VERSION=4.14.0
RUN wget https://mirror.openshift.com/pub/openshift-v4/x86_64/clients/ocp/$OC_VERSION/openshift-client-linux-$OC_VERSION.tar.gz && \
    tar -zxvf openshift-client-linux-$OC_VERSION.tar.gz -C /usr/local/bin oc && \
    chmod +x /usr/local/bin/oc && \
    rm openshift-client-linux-$OC_VERSION.tar.gz

# Create a directory for scripts
WORKDIR /app

# Copy the scripts into the image
COPY trigger.py /app/trigger.py
COPY transfer.bash /app/transfer.bash
COPY restart.bash /app/restart.bash

# Ensure the bash script is executable
RUN chmod +x /app/transfer.bash
RUN chmod +x /app/trigger.py
RUN chmod +x /app/restart.bash

# Run the trigger script on container startup
CMD ["python", "/app/trigger.py"]
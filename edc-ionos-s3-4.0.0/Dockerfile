# Use a lightweight Python base image
FROM python:3.9-slim

# Install dependencies for Python and transfer.bash
RUN apt-get update && apt-get install -y \
    curl \
    jq \
    uuid-runtime \
    && rm -rf /var/lib/apt/lists/* \
    && pip install --no-cache-dir boto3

# Create a directory for scripts
WORKDIR /app

# Copy the scripts into the image
COPY trigger.py /app/trigger.py
COPY transfer.bash /app/transfer.bash

# Ensure the bash script is executable
RUN chmod +x /app/transfer.bash

# Run the Python script on container startup
CMD ["python", "/app/trigger.py"]
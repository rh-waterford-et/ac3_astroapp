# Build stage
FROM golang:1.24-alpine AS builder

# Install git for go modules
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy the UC3 application module to handle the replace directive
COPY ac3_astroapp/astroapp/application /astroapp/application

# Copy experiment tool files
COPY ac3_astroapp/experiment-tool/go.mod ac3_astroapp/experiment-tool/go.sum ./
COPY ac3_astroapp/experiment-tool/ .

# Download dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o uc3-experiment ./cmd/uc3-experiment

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests, bash for cleanup script, and curl
RUN apk --no-cache add ca-certificates bash curl

# Install kubectl (OpenShift CLI compatible)
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
    chmod +x kubectl && \
    mv kubectl /usr/local/bin/kubectl && \
    ln -s /usr/local/bin/kubectl /usr/local/bin/oc

# Create app directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/uc3-experiment .

# Copy configuration files to the correct location
COPY --from=builder /app/configs ./configs

# Copy cleanup scripts
COPY --from=builder /app/scripts ./scripts

# Copy dataset files from host system
COPY datasets /app/datasets/

# Create directories for data and results
RUN mkdir -p /app/experiment-data /app/test-results

# Run the application
ENTRYPOINT ["./uc3-experiment"]

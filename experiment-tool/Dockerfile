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

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create app directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/uc3-experiment .

# Copy configuration files to the correct location
COPY --from=builder /app/configs ./configs

# Copy dataset files from host system
COPY NGC7025_short /app/datasets/NGC7025_short
COPY NGC7025_full /app/datasets/NGC7025_full
COPY NGC7000 /app/datasets/NGC7000
COPY NGC7021 /app/datasets/NGC7021
COPY NGC7022 /app/datasets/NGC7022
COPY NGC7023 /app/datasets/NGC7023
COPY NGC7024 /app/datasets/NGC7024

# Create directories for data and results
RUN mkdir -p /app/experiment-data /app/test-results

# Run the application
ENTRYPOINT ["./uc3-experiment"]

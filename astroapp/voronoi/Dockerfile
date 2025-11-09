FROM python:3.11-slim

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    g++ \
    curl \
    jq \
    && rm -rf /var/lib/apt/lists/*

# Install Python dependencies
RUN pip install --no-cache-dir \
    numpy \
    scipy \
    astropy \
    matplotlib \
    vorbin

# Create app directory
WORKDIR /app

# Copy voronoi script and monitoring script
COPY voronoi_binning.py /app/
COPY run_voronoi.sh /app/
RUN chmod +x /app/run_voronoi.sh

# Create data directories
RUN mkdir -p /processing_data/voronoi/data/input /processing_data/voronoi/data/output /processing_data/voronoi/runtime

# Run Voronoi monitoring script
CMD ["/app/run_voronoi.sh"]


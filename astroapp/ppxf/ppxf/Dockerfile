FROM ubuntu:focal
MAINTAINER Mario Chamorro <mchamorro@ucm.com>

# Install Python and pip
RUN apt-get update && \
    apt-get install -y python3 python3-pip curl && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /home/ppxf/

RUN echo "deb [trusted=yes] http://old-releases.ubuntu.com/ubuntu/ hardy universe" >> /etc/apt/sources.list
RUN echo "deb-src [trusted=yes] http://old-releases.ubuntu.com/ubuntu/ hardy universe" >> /etc/apt/sources.list
RUN echo "deb [trusted=yes] http://old-releases.ubuntu.com/ubuntu/ hardy-updates universe" >> /etc/apt/sources.list
RUN echo "deb-src [trusted=yes] http://old-releases.ubuntu.com/ubuntu/ hardy-updates universe" >> /etc/apt/sources.list

# Install ppxf and other dependencies using pip
RUN pip install --no-cache-dir ppxf vorbin plotbin specutils

RUN apt update -y

# Copy the pPXF setup
COPY run_ppxf/ /home/ppxf/run_ppxf/

# Apply patches and setup SSP models
COPY run_ppxf/ssp_models/* /usr/local/lib/python3.8/dist-packages/ppxf/sps_models/
RUN patch /usr/local/lib/python3.8/dist-packages/ppxf/ppxf.py run_ppxf/modified_scripts/patch_ppxf.diff
RUN patch /usr/local/lib/python3.8/dist-packages/ppxf/sps_util.py run_ppxf/modified_scripts/patch_sps_util.diff
RUN patch /usr/local/lib/python3.8/dist-packages/vorbin/voronoi_2d_binning.py run_ppxf/modified_scripts/patch_voronoi_2d_binning.diff
RUN cp /usr/local/lib/python3.8/dist-packages/capfit/capfit.py /usr/local/lib/python3.8/dist-packages/ppxf/

# Create necessary directories for our processing pipeline
RUN mkdir -p /processing_data/ppxf/data/input && \
    mkdir -p /processing_data/ppxf/data/output && \
    mkdir -p /processing_data/ppxf/data/processed && \
    mkdir -p /processing_data/ppxf/runtime && \
    touch /processing_data/ppxf/runtime/processlist.txt

# Copy our processing script
COPY ppxf_script.sh /home/ppxf/ppxf_script.sh

# Make the script executable
RUN chmod +x /home/ppxf/ppxf_script.sh

# Set proper permissions for processing directories
RUN chmod -R 755 /processing_data/ppxf/

# Set the default command to run our processing script
CMD ["/home/ppxf/ppxf_script.sh"]
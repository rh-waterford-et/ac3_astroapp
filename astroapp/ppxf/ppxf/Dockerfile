FROM ubuntu:focal
MAINTAINER Mario Chamorro <mchamorro@ucm.com>

# Install Python, pip, and curl (for progress updates)
RUN apt-get update && \
    apt-get install -y python3 python3-pip curl

WORKDIR /home/ppxf/

RUN echo "deb [trusted=yes] http://old-releases.ubuntu.com/ubuntu/ hardy universe" >> /etc/apt/sources.list
RUN echo "deb-src [trusted=yes] http://old-releases.ubuntu.com/ubuntu/ hardy universe" >> /etc/apt/sources.list
RUN echo "deb [trusted=yes] http://old-releases.ubuntu.com/ubuntu/ hardy-updates universe" >> /etc/apt/sources.list
RUN echo "deb-src [trusted=yes] http://old-releases.ubuntu.com/ubuntu/ hardy-updates universe" >> /etc/apt/sources.list


# Install ppxf and other dependencies using pip with specific versions
RUN pip install --no-cache-dir ppxf vorbin==3.1.5 plotbin specutils

RUN apt update -y

COPY ./* /home/ppxf/run_ppxf/
COPY run_ppxf/ssp_models/* /usr/local/lib/python3.8/dist-packages/ppxf/sps_models/
# Copy the main script to the working directory
COPY ppxf_script.sh /home/ppxf/ppxf_script.sh
RUN chmod +x /home/ppxf/ppxf_script.sh
RUN patch /usr/local/lib/python3.8/dist-packages/ppxf/ppxf.py run_ppxf/modified_scripts/patch_ppxf.diff
RUN patch /usr/local/lib/python3.8/dist-packages/ppxf/sps_util.py run_ppxf/modified_scripts/patch_sps_util.diff
# Try to apply voronoi patch with tolerance for fuzz
RUN patch --fuzz=3 /usr/local/lib/python3.8/dist-packages/vorbin/voronoi_2d_binning.py run_ppxf/modified_scripts/patch_voronoi_2d_binning.diff || echo "Voronoi patch failed, continuing without it"
RUN cp /usr/local/lib/python3.8/dist-packages/capfit/capfit.py /usr/local/lib/python3.8/dist-packages/ppxf/
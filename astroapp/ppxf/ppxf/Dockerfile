FROM ubuntu:focal
MAINTAINER Mario Chamorro <mchamorro@ucm.com>

# Install Python, pip, curl, and patch utility  
RUN apt-get update && \
    apt-get install -y python3 python3-pip patch curl

WORKDIR /home/ppxf/

RUN echo "deb [trusted=yes] http://old-releases.ubuntu.com/ubuntu/ hardy universe" >> /etc/apt/sources.list
RUN echo "deb-src [trusted=yes] http://old-releases.ubuntu.com/ubuntu/ hardy universe" >> /etc/apt/sources.list
RUN echo "deb [trusted=yes] http://old-releases.ubuntu.com/ubuntu/ hardy-updates universe" >> /etc/apt/sources.list
RUN echo "deb-src [trusted=yes] http://old-releases.ubuntu.com/ubuntu/ hardy-updates universe" >> /etc/apt/sources.list

# Install ppxf and other dependencies using pip
RUN pip install --no-cache-dir ppxf==9.4.1 vorbin==3.1.5 plotbin==3.1.7 specutils

RUN apt update -y

# Copy scripts and patches but exclude the LFS pointer files
COPY run_ppxf/ /home/ppxf/run_ppxf/
COPY download_sps_models.py /home/ppxf/
COPY ppxf_script.sh /home/ppxf/ppxf_script.sh

# Download real SPS model files to the staging area (overwriting LFS pointers)
RUN python3 /home/ppxf/download_sps_models.py

# Copy the downloaded models to the ppxf package directory
RUN cp /home/ppxf/run_ppxf/ssp_models/* /usr/local/lib/python3.8/dist-packages/ppxf/sps_models/

# Double-check: ensure models are in BOTH locations by copying back
RUN cp /usr/local/lib/python3.8/dist-packages/ppxf/sps_models/* /home/ppxf/run_ppxf/ssp_models/

# Apply patches to the installed ppxf package
RUN patch /usr/local/lib/python3.8/dist-packages/ppxf/ppxf.py /home/ppxf/run_ppxf/modified_scripts/patch_ppxf.diff
RUN patch /usr/local/lib/python3.8/dist-packages/ppxf/sps_util.py /home/ppxf/run_ppxf/modified_scripts/patch_sps_util.diff
RUN patch /usr/local/lib/python3.8/dist-packages/vorbin/voronoi_2d_binning.py /home/ppxf/run_ppxf/modified_scripts/patch_voronoi_2d_binning.diff
RUN cp /usr/local/lib/python3.8/dist-packages/capfit/capfit.py /usr/local/lib/python3.8/dist-packages/ppxf/

# Fix pickle issue
RUN sed -i 's/a = np\.load(filename)/a = np.load(filename, allow_pickle=True)/g' /usr/local/lib/python3.8/dist-packages/ppxf/sps_util.py

# Make script executable
RUN chmod +x /home/ppxf/ppxf_script.sh

WORKDIR /processing_data/ppxf/data/input

CMD ["bash", "/home/ppxf/ppxf_script.sh"]

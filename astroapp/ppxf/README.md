# pPXF individual Docker Container

This project provides a preconfigured Docker container for running the pPXF (Penalized Pixel-Fitting) astronomy code on an individual spectrum (or a set of individual spectra). pPXF is a powerful tool used in the astronomical community for stellar kinematics and spectrum decomposition. By encapsulating pPXF in a Docker container, we facilitate its execution without worrying about environment dependencies.

## Sources

For any reference and docs go to https://github.com/micappe/ppxf_examples

## How-to build from the repository
```
# Clone the repository from GitHub:
git clone https://github.com/MarioChC/astro-containers.git
cd astro-containers/ppxf_individual_docker

# Enter the "ppxf" folder:
cd ppxf

# Build the Docker container:
docker build -t ppxf_image .

# Check the image ID:
docker image ls

# Create the shared directories:
mkdir shared_directory

# Create the output directory in the shared volume:
mkdir shared_directory/output

# Copy the directory with yout input files:
cp -r ../input shared_directory/

# Run pPXF container in detached mode (-d) from the same folder, mounting a shared volume between the local machine and the container, and leave it running:

docker run -d --name ppxf_container -v <LOCAL_PATH>/astro-containers/ppxf_docker/ppxf/shared_directory/:/home/ppxf/shared_directory/ <IMAGE_ID> sleep infinity

# Check the container ID:
docker ps

# Run the analysis with Starlight and the configuration files:
docker exec ppxf_container /home/ppxf/shared_directory/input/bash_script.sh

# The output files will be stored in your computer in the "<LOCAL_PATH>/astro-containers/ppxf_docker/ppxf/shared_directory/output" directory.

# Adjust your data files and execute it as you need.
```

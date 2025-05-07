#!/bin/bash

export RABBITMQ_USER="guest"
export RABBITMQ_PASSWORD="guest"
export RABBITMQ_HOST="localhost"
export RABBITMQ_PORT="5672"
export BATCH_SIZE="5"
export AWS_ACCESS_KEY_ID = "EEAAAAHuEn6LkwZJGpUAikGoJHyOrAra4yXAV1WVXpvA8XU0HAAAAAEB7vvNAAAAAAHu-80YrfBIAbZ4Ef4idjOF9BTQ"
export AWS_SECRET_ACCESS_KEY = "D3c2ApVsGGa7Wm1+pmfDwJddoPxhY3qSwU1U2EUUyKJRE07H3MjR3nwwaxim7mhY"
export S3_ENDPOINT = "https://s3.eu-central-1.ionoscloud.com"
export S3_REGION = "de"
export EXPLORED_DIR_STARLIGHT = "starlight_input"
export OUTPUT_DIR_STARLIGHT = "starlight_output"
export EXPLORED_DIR_PPFX = "ppfx_input"
export OUTPUT_DIR_PPFX = "ppfx_output"
export EXPLORED_DIR_STECKMAP ="steckmap_input"
export OUTPUT_DIR_STECKMAP = "steckmap_output"
export TEMPLATE_IN_FILE_PATH="data/grid_example.in"
export IN_FILE_OUTPUT_PATH="starlight/runtime/infiles/"


./build/ucm "${1}"


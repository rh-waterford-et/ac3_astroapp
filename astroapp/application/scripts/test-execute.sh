#!/bin/bash

export RABBITMQ_USER="guest"
export RABBITMQ_PASSWORD="guest"
export RABBITMQ_HOST="localhost"
export RABBITMQ_PORT="5672"
export BATCH_SIZE="5"
export EXPLORED_DIR_Starlight="starlight/data/input"
export OUTPUT_DIR_Starlight="starlight/data/output"
export EXPLORED_DIR_PPFX="ppfx/data/input"
export OUTPUT_DIR_PPFX="ppfx/data/output"
export EXPLORED_DIR_Steckmap="steckmap/data/input"
export OUTPUT_DIR_Steckmap="steckmap/data/output"
export TEMPLATE_IN_FILE_PATH="data/grid_example.in"
export IN_FILE_OUTPUT_PATH="starlight/runtime/infiles/"


./build/ucm "${1}"


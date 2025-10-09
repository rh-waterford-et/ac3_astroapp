#!/bin/bash

set -x

# Get pod-specific processlist path
POD_NAME="${POD_NAME:-default}"
PROCESS_FILE="/processing_data/starlight/runtime/processlist-${POD_NAME}.txt"

# Ensure processlist file exists
touch "$PROCESS_FILE"

# Configuration
INFILES_DIR="/processing_data/starlight/runtime/infiles"
EXECUTABLE="/docker/starlight/STARLIGHTv04/StarlightChains_v04.amd64_g77-3.4.6-r1_static.exe"

echo "Using pod-specific processlist: $PROCESS_FILE"

removeInFileFromList(){
    echo "before"
    cat $PROCESS_FILE
    sed -i '1d' $PROCESS_FILE
    echo "after"
    cat $PROCESS_FILE
}

# Verificar si el ejecutable existe
#if [ ! -f "$EXECUTABLE" ]; then
#    echo "Error: No se encontró el ejecutable $EXECUTABLE"
#    exit 1
#fi


while :
do
    echo "Reading Next Line from $PROCESS_FILE"
    read -r firstline<"$PROCESS_FILE"
    echo "NEXT FILE = $firstline"

    if [[ "$firstline" = "" ]]; then # TODO fix this to check for empty values properly
    ##TODO CROSS CHECK FILE IS PRESENT
        echo "Waiting for data file to start"
    else
        echo "Starting Application with input " /processing_data/starlight/runtime/infiles/$firstline
        
        #./StarlightChains_v04.amd64_g77-3.4.6-r1_static.exe < /processing_data/starlight/grid_example.in
        ./StarlightChains_v04.amd64_g77-3.4.6-r1_static.exe < /processing_data/starlight/runtime/infiles/$firstline
        exit_code=$?

        if [ $exit_code -ne 0 ]; then
            echo "Error"
        fi
        
        echo "Removing Start Flag"
        removeInFileFromList
        echo "Complete"

    fi
    #if flag set
#    if [ ! -f "$DATA_FILE_FLAG" ]; then
#        echo "Waiting for data file to start"
#    else
#        echo "Starting Application"
#        ./StarlightChains_v04.amd64_g77-3.4.6-r1_static.exe < /starlight/grid_example.in
#        exit_code=$?

#        if [ $exit_code -ne 0 ]; then
#            echo "Error"
#        fi
#        
#        echo "Removing Start Flag"
#        rm "$DATA_FILE_FLAG"
#        echo "Complete"
#        ls -al "$DATA_FILE_FLAG"
#    fi
    sleep 10
done


## need to handle errors, remove flag or flag error
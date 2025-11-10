#!/bin/bash
set -e

echo "Starting combined UC3 container..."


/usr/bin/ucm server &


if [ -f /docker/starlight/STARLIGHTv04/run_starlight.sh ]; then
    bash /docker/starlight/STARLIGHTv04/run_starlight.sh &
fi


if [ -f /home/ppxf/ppxf_script.sh ]; then
    bash /home/ppxf/ppxf_script.sh &
fi

wait

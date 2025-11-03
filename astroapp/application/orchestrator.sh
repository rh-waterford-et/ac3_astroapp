#!/bin/bash

# Start the first process
/usr/bin/ucm watcher producer &

# Start the second process
/usr/bin/ucm server &

# Start the second process
/usr/bin/ucm consumer producer &

/usr/bin/ucm aggregator &

# Wait for any process to exit
wait -n

# Exit with status of process that exited first
exit $?
#!/bin/bash

set -x

# Configuration
PPXF_INPUT_DIR="/processing_data/ppxf/data/input"
PPXF_OUTPUT_DIR="/processing_data/ppxf/data/output"
POD_NAME="${POD_NAME:-default}"
PROCESS_FILE="/processing_data/ppxf/runtime/processlist-${POD_NAME}.txt"
PPXF_SCRIPT="/home/ppxf/run_ppxf/ppxf_individual.py"
MASK_FILE="/processing_data/ppxf/data/input/mask.txt"

# API server URL for progress updates
API_URL="http://uc3-backend-service:8080/api/progress/update"

# Function to load configuration from ppxf_config.json
load_ppxf_config() {
    local config_file="$PPXF_INPUT_DIR/ppxf_config.json"
    
    # Default parameters
    REDSHIFT=${PPXF_REDSHIFT:-0.0}
    VELOCITY_DISPERSION=${PPXF_VEL_DISP:-100.0}
    WAVE_RANGE_START=${PPXF_WAVE_START:-4000}
    WAVE_RANGE_END=${PPXF_WAVE_END:-7000}
    SPS_NAME=${PPXF_SPS_NAME:-emiles}
    
    # Check if user config file exists
    if [ -f "$config_file" ]; then
        echo "Found user pPXF configuration: $config_file"
        echo "Loading user-defined parameters..."
        
        # Parse JSON and extract values using jq if available, otherwise use grep/sed
        if command -v jq >/dev/null 2>&1; then
            echo "Using jq for JSON parsing"
            REDSHIFT=$(jq -r '.redshift // empty' "$config_file" 2>/dev/null)
            VELOCITY_DISPERSION=$(jq -r '.velocityDisp // empty' "$config_file" 2>/dev/null)
            WAVE_RANGE_START=$(jq -r '.waveRangeStart // empty' "$config_file" 2>/dev/null)
            WAVE_RANGE_END=$(jq -r '.waveRangeEnd // empty' "$config_file" 2>/dev/null)
            SPS_NAME=$(jq -r '.spsName // empty' "$config_file" 2>/dev/null)
        else
            echo "jq not available, using grep/sed for JSON parsing"
            # Fallback parsing without jq
            REDSHIFT=$(grep -o '"redshift"[^,]*' "$config_file" | sed 's/.*: *\([0-9.]*\).*/\1/' 2>/dev/null)
            VELOCITY_DISPERSION=$(grep -o '"velocityDisp"[^,]*' "$config_file" | sed 's/.*: *\([0-9.]*\).*/\1/' 2>/dev/null)
            WAVE_RANGE_START=$(grep -o '"waveRangeStart"[^,]*' "$config_file" | sed 's/.*: *\([0-9]*\).*/\1/' 2>/dev/null)
            WAVE_RANGE_END=$(grep -o '"waveRangeEnd"[^,]*' "$config_file" | sed 's/.*: *\([0-9]*\).*/\1/' 2>/dev/null)
            SPS_NAME=$(grep -o '"spsName"[^,]*' "$config_file" | sed 's/.*: *"\([^"]*\)".*/\1/' 2>/dev/null)
        fi
        
        # Use defaults if parsing failed
        REDSHIFT=${REDSHIFT:-${PPXF_REDSHIFT:-0.0}}
        VELOCITY_DISPERSION=${VELOCITY_DISPERSION:-${PPXF_VEL_DISP:-100.0}}
        WAVE_RANGE_START=${WAVE_RANGE_START:-${PPXF_WAVE_START:-4000}}
        WAVE_RANGE_END=${WAVE_RANGE_END:-${PPXF_WAVE_END:-7000}}
        SPS_NAME=${SPS_NAME:-${PPXF_SPS_NAME:-emiles}}
        
        echo "Loaded configuration from $config_file:"
        echo "  Redshift: $REDSHIFT"
        echo "  Velocity Dispersion: $VELOCITY_DISPERSION km/s"
        echo "  Wave Range: $WAVE_RANGE_START - $WAVE_RANGE_END Å"
        echo "  SPS Model: $SPS_NAME"
    else
        echo "No user config found at $config_file"
        echo "Using environment variables or defaults:"
        echo "  Redshift: $REDSHIFT"
        echo "  Velocity Dispersion: $VELOCITY_DISPERSION km/s"
        echo "  Wave Range: $WAVE_RANGE_START - $WAVE_RANGE_END Å"
        echo "  SPS Model: $SPS_NAME"
    fi
}

# Function to remove first line from process list
removeFileFromList(){
    echo "before"
    cat $PROCESS_FILE
    sed -i '1d' $PROCESS_FILE
    echo "after"
    cat $PROCESS_FILE
}

# Function to send progress update
send_progress_update() {
    local dataset_id="$1"
    local stage="$2"
    local progress="$3"
    
    curl -s -X POST "$API_URL" \
        -H "Content-Type: application/json" \
        -d "{\"dataset_id\":\"$dataset_id\",\"stage\":\"$stage\",\"progress\":$progress}" \
        || echo "Failed to send progress update"
}

# Create default mask file if it doesn't exist
create_default_mask() {
    echo "Checking mask file: $MASK_FILE"
    if [ ! -f "$MASK_FILE" ]; then
        echo "Mask file not found, creating directories and file..."
        mkdir -p "$(dirname "$MASK_FILE")"
        echo "Created directory: $(dirname "$MASK_FILE")"
        
        cat > "$MASK_FILE" << EOF
5245 5280 False
5572 5582 False
5650 5665 False
5885 5900 False
5875 5910 True
EOF
        echo "Created default mask file: $MASK_FILE"
        
        # Verify the file was created
        if [ -f "$MASK_FILE" ]; then
            echo "Mask file successfully created and verified"
            ls -la "$MASK_FILE"
            echo "Mask file contents:"
            cat "$MASK_FILE"
        else
            echo "ERROR: Mask file creation failed!"
        fi
    else
        echo "Mask file already exists: $MASK_FILE"
        ls -la "$MASK_FILE"
    fi
}

# Main processing loop
main() {
    echo "Starting pPXF processing script..."
    echo "Pod name: $POD_NAME"
    echo "Process file: $PROCESS_FILE"
    echo "Input directory: $PPXF_INPUT_DIR"
    echo "Output directory: $PPXF_OUTPUT_DIR"
    
    # Load configuration (either from user config file or environment variables)
    load_ppxf_config
    
    echo "Final parameters: redshift=$REDSHIFT, vel_disp=$VELOCITY_DISPERSION, wave_range=$WAVE_RANGE_START-$WAVE_RANGE_END"
    
    # Create default mask file
    create_default_mask
    
    while true; do
        # Check if process file exists and has content
        if [ ! -f "$PROCESS_FILE" ] || [ ! -s "$PROCESS_FILE" ]; then
            echo "No process file or empty process file. Waiting..."
            sleep 5
            continue
        fi
        
        # Read the first line (next file to process)
        filename=$(head -n 1 "$PROCESS_FILE" 2>/dev/null)
        
        if [ -z "$filename" ]; then
            echo "Empty filename in process list. Waiting..."
            sleep 5
            continue
        fi
        
        # Check if input file exists
        input_file="$PPXF_INPUT_DIR/$filename"
        if [ ! -f "$input_file" ]; then
            echo "Input file not found: $input_file"
            removeFileFromList
            continue
        fi
        
        echo "Processing file: $filename"
        
        # Extract dataset ID from filename for progress tracking
        dataset_id=$(echo "$filename" | sed 's/\.[^.]*$//')
        
        # Ensure mask file exists before processing
        echo "Ensuring mask file exists before processing..."
        create_default_mask
        
        # Run pPXF analysis directly on the input file
        echo "Running pPXF analysis for: $filename"
        
        # Debug: Check mask file right before Python script
        echo "=== PRE-PYTHON DEBUG ==="
        echo "Current working directory: $(pwd)"
        echo "Checking mask file existence right before Python script:"
        ls -la "$MASK_FILE" || echo "Mask file not found!"
        echo "Directory contents of $(dirname "$MASK_FILE"):"
        ls -la "$(dirname "$MASK_FILE")" || echo "Directory not found!"
        echo "File system check:"
        file "$MASK_FILE" || echo "File command failed"
        echo "========================"
        
        # Send progress update - processing started
        send_progress_update "$dataset_id" "processing" 30.0
        
        # Run pPXF analysis
        python3 "$PPXF_SCRIPT" \
            --filenames "$input_file" \
            --mask-file "$MASK_FILE" \
            --redshift "$REDSHIFT" \
            --velocity-dispersion "$VELOCITY_DISPERSION" \
            --wave-range "$WAVE_RANGE_START" "$WAVE_RANGE_END" \
            --sps-name "$SPS_NAME" \
            --output-dir "$PPXF_OUTPUT_DIR" || echo "Python script failed with exit code $?"
        
        ppxf_exit_code=$?
        
        if [ $ppxf_exit_code -eq 0 ]; then
            echo "pPXF analysis completed successfully for: $filename"
            
            # Verify output files were created
            base_name=$(basename "$filename" .fits)
            
            # Expected output files
            expected_files=(
                "${base_name}_kinematics_and_stellar_pops_info.txt"
                "${base_name}_pPXF_fitting.pdf"
                "${base_name}_residuals.fits"
                "${base_name}_bestfit.fits"
                "${base_name}_galaxy.fits"
            )
            
            # Check each output file
            for output_file in "${expected_files[@]}"; do
                if [ -f "$PPXF_OUTPUT_DIR/$output_file" ]; then
                    echo "Created output file: $output_file"
                else
                    echo "Warning: Expected output file not found: $output_file"
                fi
            done
            
            # Send progress update - processing completed
            send_progress_update "$dataset_id" "complete" 100.0
            
            echo "Successfully processed: $filename"
        else
            echo "pPXF analysis failed for: $filename (exit code: $ppxf_exit_code)"
            
            # Send progress update - error
            send_progress_update "$dataset_id" "error" 0.0
            
            # For failed files, we could implement retry logic here
            echo "File processing failed: $filename"
        fi
        
        # No cleanup needed since we work directly with processing_data
        
        # Remove processed file from the list
        removeFileFromList
        
        echo "Completed processing: $filename"
        echo "----------------------------------------"
        
        # Small delay to prevent overwhelming the system
        sleep 2
    done
}

# Run the main function
main 
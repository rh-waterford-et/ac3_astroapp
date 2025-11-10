#!/bin/bash

set -x

# Configuration
VORONOI_INPUT_DIR="/processing_data/voronoi/data/input"
VORONOI_OUTPUT_DIR="/processing_data/voronoi/data/output"
POD_NAME="${POD_NAME:-default}"
PROCESS_FILE="/processing_data/voronoi/runtime/processlist-${POD_NAME}.txt"
VORONOI_SCRIPT="/app/voronoi_binning.py"

# API server URL for progress updates
API_URL="http://uc3-backend-service:8080/api/progress/update"

# Function to load configuration from voronoi_config.json
load_voronoi_config() {
    local config_file="$VORONOI_INPUT_DIR/voronoi_config.json"
    
    # Default parameters
    TARGET_SN=${VORONOI_TARGET_SN:-30}
    REDSHIFT=${VORONOI_REDSHIFT:-0.0}
    WAVE_START=${VORONOI_WAVE_START:-5000}
    WAVE_END=${VORONOI_WAVE_END:-7000}
    SN_METHOD=${VORONOI_SN_METHOD:-spline}
    KNOTS_NUMBER=${VORONOI_KNOTS:-40}
    MIN_SN=${VORONOI_MIN_SN:-1}
    GEN_INDIVIDUAL=${VORONOI_GEN_INDIVIDUAL:-false}
    INSTRUMENT=${VORONOI_INSTRUMENT:-megara}
    
    # Check if user config file exists
    if [ -f "$config_file" ]; then
        echo "Found Voronoi configuration: $config_file"
        echo "Loading user-defined parameters..."
        
        # Parse JSON and extract values using jq if available, otherwise use grep/sed
        if command -v jq >/dev/null 2>&1; then
            echo "Using jq for JSON parsing"
            TARGET_SN=$(jq -r '.targetSN // empty' "$config_file" 2>/dev/null)
            REDSHIFT=$(jq -r '.redshift // empty' "$config_file" 2>/dev/null)
            WAVE_START=$(jq -r '.wavelengthStart // empty' "$config_file" 2>/dev/null)
            WAVE_END=$(jq -r '.wavelengthEnd // empty' "$config_file" 2>/dev/null)
            SN_METHOD=$(jq -r '.snMethod // empty' "$config_file" 2>/dev/null)
            KNOTS_NUMBER=$(jq -r '.knotsNumber // empty' "$config_file" 2>/dev/null)
            MIN_SN=$(jq -r '.minSN // empty' "$config_file" 2>/dev/null)
            GEN_INDIVIDUAL=$(jq -r '.generateIndividualSpectra // empty' "$config_file" 2>/dev/null)
            INSTRUMENT=$(jq -r '.instrument // empty' "$config_file" 2>/dev/null)
        else
            echo "jq not available, using grep/sed for JSON parsing"
            # Fallback parsing without jq
            TARGET_SN=$(grep -o '"targetSN"[^,]*' "$config_file" | sed 's/.*: *\([0-9.]*\).*/\1/' 2>/dev/null)
            REDSHIFT=$(grep -o '"redshift"[^,]*' "$config_file" | sed 's/.*: *\([0-9.]*\).*/\1/' 2>/dev/null)
            WAVE_START=$(grep -o '"wavelengthStart"[^,]*' "$config_file" | sed 's/.*: *\([0-9]*\).*/\1/' 2>/dev/null)
            WAVE_END=$(grep -o '"wavelengthEnd"[^,]*' "$config_file" | sed 's/.*: *\([0-9]*\).*/\1/' 2>/dev/null)
            SN_METHOD=$(grep -o '"snMethod"[^,]*' "$config_file" | sed 's/.*: *"\([^"]*\)".*/\1/' 2>/dev/null)
            KNOTS_NUMBER=$(grep -o '"knotsNumber"[^,]*' "$config_file" | sed 's/.*: *\([0-9]*\).*/\1/' 2>/dev/null)
            MIN_SN=$(grep -o '"minSN"[^,]*' "$config_file" | sed 's/.*: *\([0-9.]*\).*/\1/' 2>/dev/null)
            GEN_INDIVIDUAL=$(grep -o '"generateIndividualSpectra"[^,]*' "$config_file" | sed 's/.*: *\([a-z]*\).*/\1/' 2>/dev/null)
            INSTRUMENT=$(grep -o '"instrument"[^,]*' "$config_file" | sed 's/.*: *"\([^"]*\)".*/\1/' 2>/dev/null)
        fi
        
        # Use defaults if parsing failed
        TARGET_SN=${TARGET_SN:-${VORONOI_TARGET_SN:-30}}
        REDSHIFT=${REDSHIFT:-${VORONOI_REDSHIFT:-0.0}}
        WAVE_START=${WAVE_START:-${VORONOI_WAVE_START:-5000}}
        WAVE_END=${WAVE_END:-${VORONOI_WAVE_END:-7000}}
        SN_METHOD=${SN_METHOD:-${VORONOI_SN_METHOD:-spline}}
        KNOTS_NUMBER=${KNOTS_NUMBER:-${VORONOI_KNOTS:-40}}
        MIN_SN=${MIN_SN:-${VORONOI_MIN_SN:-1}}
        GEN_INDIVIDUAL=${GEN_INDIVIDUAL:-${VORONOI_GEN_INDIVIDUAL:-false}}
        INSTRUMENT=${INSTRUMENT:-${VORONOI_INSTRUMENT:-megara}}
        
        echo "Loaded configuration from $config_file:"
        echo "  Target S/N: $TARGET_SN"
        echo "  Redshift: $REDSHIFT"
        echo "  Wavelength Range: $WAVE_START - $WAVE_END Å"
        echo "  S/N Method: $SN_METHOD"
        echo "  Knots Number: $KNOTS_NUMBER"
        echo "  Min S/N: $MIN_SN"
        echo "  Generate Individual: $GEN_INDIVIDUAL"
        echo "  Instrument: $INSTRUMENT"
    else
        echo "No user config found at $config_file"
        echo "Using environment variables or defaults:"
        echo "  Target S/N: $TARGET_SN"
        echo "  Redshift: $REDSHIFT"
        echo "  Wavelength Range: $WAVE_START - $WAVE_END Å"
        echo "  S/N Method: $SN_METHOD"
        echo "  Instrument: $INSTRUMENT"
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

# Main processing loop
main() {
    echo "Starting Voronoi binning processing script..."
    echo "Pod name: $POD_NAME"
    echo "Process file: $PROCESS_FILE"
    echo "Input directory: $VORONOI_INPUT_DIR"
    echo "Output directory: $VORONOI_OUTPUT_DIR"
    
    # Load configuration (either from user config file or environment variables)
    load_voronoi_config
    
    echo "Final parameters: targetSN=$TARGET_SN, redshift=$REDSHIFT, wavelength=$WAVE_START-$WAVE_END, snMethod=$SN_METHOD, instrument=$INSTRUMENT"
    
    while true; do
        # Check if process file exists and has content
        if [ ! -f "$PROCESS_FILE" ] || [ ! -s "$PROCESS_FILE" ]; then
            echo "No process file or empty process file. Waiting..."
            sleep 5
            continue
        fi
        
        # Read the first line (next datacube to process)
        filename=$(head -n 1 "$PROCESS_FILE" 2>/dev/null)
        
        if [ -z "$filename" ]; then
            echo "Empty filename in process list. Waiting..."
            sleep 5
            continue
        fi
        
        # Check if input file exists
        input_file="$VORONOI_INPUT_DIR/$filename"
        if [ ! -f "$input_file" ]; then
            echo "Input file not found: $input_file"
            removeFileFromList
            continue
        fi
        
        echo "Processing Voronoi datacube: $filename"
        
        # Extract dataset ID from filename for progress tracking
        dataset_id=$(echo "$filename" | sed 's/\.[^.]*$//')
        
        # Send progress update - processing started
        send_progress_update "$dataset_id" "processing" 30.0
        
        # Build Voronoi command based on configuration
        echo "Running Voronoi binning analysis for: $filename"
        
        cmd="python3 $VORONOI_SCRIPT $input_file"
        cmd="$cmd --sn $TARGET_SN"
        cmd="$cmd --redshift $REDSHIFT"
        cmd="$cmd --wavelength-start $WAVE_START"
        cmd="$cmd --wavelength-end $WAVE_END"
        cmd="$cmd --sn-method $SN_METHOD"
        cmd="$cmd --min-sn $MIN_SN"
        cmd="$cmd --instrument $INSTRUMENT"
        cmd="$cmd --output-dir $VORONOI_OUTPUT_DIR"
        
        # Add optional flags based on configuration
        if [ "$SN_METHOD" = "spline" ]; then
            cmd="$cmd --knots-number $KNOTS_NUMBER"
        fi
        
        if [ "$GEN_INDIVIDUAL" = "true" ]; then
            cmd="$cmd --generate-individual-spectra"
        fi
        
        echo "Executing: $cmd"
        
        # Run Voronoi binning analysis
        eval $cmd || echo "Python script failed with exit code $?"
        
        voronoi_exit_code=$?
        
        if [ $voronoi_exit_code -eq 0 ]; then
            echo "Voronoi binning completed successfully for: $filename"
            
            # List generated output files
            echo "Generated output files:"
            ls -lh "$VORONOI_OUTPUT_DIR/" || echo "No output directory found"
            
            # Send progress update - processing completed
            send_progress_update "$dataset_id" "complete" 100.0
            
            echo "Successfully processed: $filename"
        else
            echo "Voronoi binning failed for: $filename (exit code: $voronoi_exit_code)"
            
            # Send progress update - error
            send_progress_update "$dataset_id" "error" 0.0
            
            # For failed files, we could implement retry logic here
            echo "File processing failed: $filename"
        fi
        
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


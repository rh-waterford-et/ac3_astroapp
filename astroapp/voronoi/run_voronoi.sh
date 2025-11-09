#!/bin/bash

set -e

# Configuration
VORONOI_INPUT_DIR="${INPUT_DIR_VORONOI:-/processing_data/voronoi/data/input}"
VORONOI_OUTPUT_DIR="${OUTPUT_DIR_VORONOI:-/processing_data/voronoi/data/output}"
POD_NAME="${POD_NAME:-default}"
PROCESS_FILE="${PROCESS_LIST_VORONOI:-/processing_data/voronoi/runtime/processlist-${POD_NAME}.txt}"
VORONOI_SCRIPT="/app/voronoi_binning.py"

# Function to remove first line from process list
removeFileFromList() {
    echo "Removing processed file from list..."
    echo "Before:"
    cat "$PROCESS_FILE" 2>/dev/null || echo "(empty or missing)"
    sed -i '1d' "$PROCESS_FILE" 2>/dev/null || true
    echo "After:"
    cat "$PROCESS_FILE" 2>/dev/null || echo "(empty)"
}

# Function to load configuration from voronoi_config.json
# Accepts optional config file path as parameter (defaults to voronoi_config.json)
load_voronoi_config() {
    local config_file="${1:-$VORONOI_INPUT_DIR/voronoi_config.json}"
    
    # Default parameters
    INSTRUMENT="${VORONOI_INSTRUMENT:-megara}"
    TARGET_SN="${VORONOI_TARGET_SN:-30}"
    REDSHIFT="${VORONOI_REDSHIFT:-0.01657}"
    WAVE_START="${VORONOI_WAVE_START:-5600}"
    WAVE_END="${VORONOI_WAVE_END:-5800}"
    SN_METHOD="${VORONOI_SN_METHOD:-spline}"
    KNOTS_NUMBER="${VORONOI_KNOTS_NUMBER:-40}"
    MIN_SN="${VORONOI_MIN_SN:-1}"
    GEN_SPECTRA="${VORONOI_GEN_SPECTRA:-true}"
    
    # Check if user config file exists
    if [ -f "$config_file" ]; then
        echo "Loading configuration from: $config_file"
        # Use a here-document to avoid quoting issues with file paths
        read -r INSTRUMENT TARGET_SN REDSHIFT WAVE_START WAVE_END SN_METHOD KNOTS_NUMBER MIN_SN GEN_SPECTRA <<EOF
$(python3 <<PYTHON_EOF
import json
import sys
try:
    with open('$config_file', 'r') as f:
        config = json.load(f)
    
    # Extract and convert all values to strings for bash compatibility
    instrument = str(config.get('instrument', '$INSTRUMENT'))
    target_sn = str(config.get('targetSN', $TARGET_SN))
    redshift = str(config.get('redshift', $REDSHIFT))
    wave_start = str(config.get('wavelengthStart', $WAVE_START))
    wave_end = str(config.get('wavelengthEnd', $WAVE_END))
    sn_method = str(config.get('snMethod', '$SN_METHOD'))
    knots_number = str(config.get('knotsNumber', $KNOTS_NUMBER))
    min_sn = str(config.get('minSN', $MIN_SN))
    
    # Convert boolean to string for bash compatibility
    gen_spectra_val = config.get('generateIndividualSpectra')
    if gen_spectra_val is None:
        gen_spectra_str = '$GEN_SPECTRA'
    else:
        # Convert boolean to lowercase string
        gen_spectra_str = 'true' if gen_spectra_val else 'false'
    
    # Print all values space-separated on a single line
    print(f"{instrument} {target_sn} {redshift} {wave_start} {wave_end} {sn_method} {knots_number} {min_sn} {gen_spectra_str}")
except Exception as e:
    print('ERROR loading config:', e, file=sys.stderr)
    print('$INSTRUMENT $TARGET_SN $REDSHIFT $WAVE_START $WAVE_END $SN_METHOD $KNOTS_NUMBER $MIN_SN $GEN_SPECTRA')
    sys.exit(0)
PYTHON_EOF
)
EOF
        echo "Configuration loaded from file:"
        echo "  Instrument: $INSTRUMENT"
        echo "  Target S/N: $TARGET_SN"
        echo "  Redshift: $REDSHIFT"
        echo "  Wavelength: $WAVE_START - $WAVE_END Å"
        echo "  S/N Method: $SN_METHOD"
        echo "  Knots Number: $KNOTS_NUMBER"
        echo "  Min S/N: $MIN_SN"
        echo "  Generate Individual Spectra: $GEN_SPECTRA"
    else
        echo "No config file found at $config_file"
        echo "Using environment variables or defaults:"
        echo "  Instrument: $INSTRUMENT"
        echo "  Target S/N: $TARGET_SN"
        echo "  Redshift: $REDSHIFT"
        echo "  Wavelength: $WAVE_START - $WAVE_END Å"
        echo "  S/N Method: $SN_METHOD"
        echo "  Knots Number: $KNOTS_NUMBER"
        echo "  Min S/N: $MIN_SN"
        echo "  Generate Individual Spectra: $GEN_SPECTRA"
    fi
}

# Main processing loop
main() {
    echo "=== Voronoi Binning Processor ==="
    echo "Starting Voronoi processing script..."
    echo "Pod name: $POD_NAME"
    echo "Process file: $PROCESS_FILE"
    echo "Input directory: $VORONOI_INPUT_DIR"
    echo "Output directory: $VORONOI_OUTPUT_DIR"
    
    # Configuration will be loaded per-job (per dataset) when processing each file
    # This ensures each dataset uses its own config file
    
    while true; do
        # Check if process file exists and has content
        if [ ! -f "$PROCESS_FILE" ] || [ ! -s "$PROCESS_FILE" ]; then
            echo "No process file or empty process file. Waiting..."
            sleep 5
            continue
        fi
        
        # Read the first line (next file to process)
        # Format may be: "dataset:filename" or just "filename"
        process_entry=$(head -n 1 "$PROCESS_FILE" 2>/dev/null | tr -d '\r\n' | xargs)
        
        if [ -z "$process_entry" ]; then
            echo "Empty entry in process list. Waiting..."
            sleep 5
            continue
        fi
        
        # Parse dataset and filename from processlist entry
        # Format: "dataset:filename" or just "filename"
        if [[ "$process_entry" == *":"* ]]; then
            # Entry contains dataset:filename format
            dataset_name="${process_entry%%:*}"
            filename="${process_entry#*:}"
        else
            # Entry is just filename (backward compatibility)
            dataset_name=""
            filename="$process_entry"
        fi
        
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
        
        # Determine config file path based on dataset
        if [ -n "$dataset_name" ]; then
            # Use dataset-specific config file
            config_file="$VORONOI_INPUT_DIR/voronoi_config_${dataset_name}.json"
        else
            # Fallback to generic config file (backward compatibility)
            config_file="$VORONOI_INPUT_DIR/voronoi_config.json"
        fi
        
        if [ ! -f "$config_file" ]; then
            echo "Config file not found: $config_file"
            if [ -n "$dataset_name" ]; then
                echo "Waiting for dataset-specific config file (dataset: $dataset_name)..."
            else
                echo "Waiting for config file to be available..."
            fi
            sleep 5
            continue
        fi
        
        if [ -n "$dataset_name" ]; then
            echo "Processing file: $filename (dataset: $dataset_name)"
        else
            echo "Processing file: $filename"
        fi
        
        # Reload config from the specific config file for this dataset
        load_voronoi_config "$config_file"
        
        # Build command
        CMD="python3 $VORONOI_SCRIPT \"$input_file\""
        CMD="$CMD --sn $TARGET_SN"
        CMD="$CMD --redshift $REDSHIFT"
        CMD="$CMD --wavelength-start $WAVE_START"
        CMD="$CMD --wavelength-end $WAVE_END"
        CMD="$CMD --sn-method $SN_METHOD"
        CMD="$CMD --output-dir \"$VORONOI_OUTPUT_DIR\""
        CMD="$CMD --min-sn $MIN_SN"
        CMD="$CMD --instrument $INSTRUMENT"
        
        # Add knots-number only if snMethod is spline
        if [ "$SN_METHOD" = "spline" ]; then
            CMD="$CMD --knots-number $KNOTS_NUMBER"
        fi
        
        # Add generate-individual-spectra flag if true
        if [ "$GEN_SPECTRA" = "true" ] || [ "$GEN_SPECTRA" = "True" ]; then
            CMD="$CMD --generate-individual-spectra"
        fi
        
        # Add progress flag
        CMD="$CMD -p"
        
        echo ""
        echo "Executing: $CMD"
        echo ""
        
        # Execute
        if eval $CMD; then
            echo ""
            echo "=== Voronoi Binning Complete for: $filename ==="
            echo "Successfully processed: $filename"
        else
            echo ""
            echo "=== Voronoi Binning Failed for: $filename ==="
            echo "File processing failed: $filename (exit code: $?)"
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

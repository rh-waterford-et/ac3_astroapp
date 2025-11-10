#!/bin/bash
set -e

echo "=== Voronoi Binning Processor ==="

# Directories
INPUT_DIR="${INPUT_DIR:-/voronoi/data/input}"
OUTPUT_DIR="${OUTPUT_DIR:-/voronoi/data/output}"

# Find the config file
CONFIG_FILE=$(find "$INPUT_DIR" -name "voronoi_config.json" -type f | head -n 1)
if [ -z "$CONFIG_FILE" ]; then
    echo "ERROR: No voronoi_config.json found in $INPUT_DIR"
    exit 1
fi

echo "Found config: $CONFIG_FILE"

# Find the datacube FITS file
DATACUBE=$(find "$INPUT_DIR" -name "*.fits" -o -name "*.fit" | head -n 1)
if [ -z "$DATACUBE" ]; then
    echo "ERROR: No FITS datacube found in $INPUT_DIR"
    exit 1
fi

echo "Found datacube: $DATACUBE"

# Parse JSON config using Python
read -r INSTRUMENT TARGET_SN REDSHIFT WAVE_START WAVE_END SN_METHOD KNOTS_NUMBER MIN_SN GEN_SPECTRA <<< $(python3 -c "
import json
import sys
with open('$CONFIG_FILE', 'r') as f:
    config = json.load(f)
print(
    config.get('instrument', 'megara'),
    config.get('targetSN', 30),
    config.get('redshift', 0.01657),
    config.get('wavelengthStart', 5600),
    config.get('wavelengthEnd', 5800),
    config.get('snMethod', 'spline'),
    config.get('knotsNumber', 40),
    config.get('minSN', 1),
    config.get('generateIndividualSpectra', 'true')
)
")

echo "Configuration:"
echo "  Instrument: $INSTRUMENT"
echo "  Target S/N: $TARGET_SN"
echo "  Redshift: $REDSHIFT"
echo "  Wavelength: $WAVE_START - $WAVE_END Å"
echo "  S/N Method: $SN_METHOD"
echo "  Knots Number: $KNOTS_NUMBER"
echo "  Min S/N: $MIN_SN"
echo "  Generate Individual Spectra: $GEN_SPECTRA"

# Build command
CMD="python3 /app/voronoi_binning.py \"$DATACUBE\""
CMD="$CMD --sn $TARGET_SN"
CMD="$CMD --redshift $REDSHIFT"
CMD="$CMD --wavelength-start $WAVE_START"
CMD="$CMD --wavelength-end $WAVE_END"
CMD="$CMD --sn-method $SN_METHOD"
CMD="$CMD --output-dir \"$OUTPUT_DIR\""
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
eval $CMD

echo ""
echo "=== Voronoi Binning Complete ==="


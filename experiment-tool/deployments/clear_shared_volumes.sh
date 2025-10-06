#!/bin/bash

# Script to clear UC3 shared volume directories for fresh experiment runs
# Assumes you're already logged into OpenShift

set -e  # Exit on any error

NAMESPACE="uc3-applications"
APP_LABEL="app=ucm-processor"

echo "🧹 UC3 Shared Volume Cleanup Script"
echo "=================================="

# Find the processor pod
echo "📍 Finding processor pod..."
POD_NAME=$(oc get pods -n $NAMESPACE -l $APP_LABEL --field-selector=status.phase=Running -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

if [ -z "$POD_NAME" ]; then
    echo "❌ Error: No running processor pod found in namespace $NAMESPACE"
    echo "   Make sure the ucm-processor-deployment is running and you're logged into OpenShift"
    exit 1
fi

echo "✅ Found running processor pod: $POD_NAME"

# Function to clear directory and report results
clear_directory() {
    local dir_path="$1"
    local description="$2"
    local pattern="$3"
    
    echo "🗑️  Clearing $description..."
    
    # Count files before deletion
    local file_count=$(oc exec $POD_NAME -n $NAMESPACE -c starlight -- find "$dir_path" -name "$pattern" 2>/dev/null | wc -l)
    
    if [ "$file_count" -eq 0 ]; then
        echo "   ✅ $description already empty"
    else
        echo "   📁 Found $file_count files to delete"
        oc exec $POD_NAME -n $NAMESPACE -c starlight -- find "$dir_path" -name "$pattern" -delete
        echo "   ✅ Cleared $file_count files from $description"
    fi
}

# Clear all directories
echo ""
echo "🚀 Starting cleanup..."

clear_directory "/processing_data/batch_info/" "batch_info files" "*.txt"
clear_directory "/processing_data/starlight/data/input/" "input files" "*.txt"
clear_directory "/processing_data/starlight/data/output/" "output files" "*.txt"

# For processed directory, clear files in subdirectories
echo "🗑️  Clearing processed files..."
processed_count=$(oc exec $POD_NAME -n $NAMESPACE -c starlight -- find "/processing_data/starlight/data/processed/" -mindepth 2 -name "*.txt" 2>/dev/null | wc -l)

if [ "$processed_count" -eq 0 ]; then
    echo "   ✅ processed files already empty"
else
    echo "   📁 Found $processed_count files to delete"
    oc exec $POD_NAME -n $NAMESPACE -c starlight -- find "/processing_data/starlight/data/processed/" -mindepth 2 -name "*.txt" -delete
    echo "   ✅ Cleared $processed_count files from processed directories"
fi

# Verify cleanup
echo ""
echo "🔍 Verifying cleanup..."
total_remaining=$(oc exec $POD_NAME -n $NAMESPACE -c starlight -- find /processing_data/ -name "*.txt" 2>/dev/null | wc -l)

if [ "$total_remaining" -eq 0 ]; then
    echo "✅ SUCCESS: All directories cleared! (0 files remaining)"
else
    echo "⚠️  WARNING: $total_remaining files still remain"
    echo "   Listing remaining files:"
    oc exec $POD_NAME -n $NAMESPACE -c starlight -- find /processing_data/ -name "*.txt" 2>/dev/null
fi

echo ""
echo "🎉 Shared volume cleanup completed!"
echo "   Pod used: $POD_NAME"
echo "   Deployment: ucm-processor-deployment"
echo "   Ready for fresh experiment run" 
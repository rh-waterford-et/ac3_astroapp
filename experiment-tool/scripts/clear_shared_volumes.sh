#!/bin/bash

# Script to clear UC3 shared volume directories for fresh experiment runs
# Assumes you're already logged into OpenShift
# Preserves processlist.txt and mask.txt files

set -e  # Exit on any error

NAMESPACE="uc3-applications"
APP_LABEL="app=ucm-processor"

echo "🧹 UC3 Shared Volume Cleanup Script"
echo "=================================="

# Find a processor pod
echo "📍 Finding processor pod..."
POD_NAME=$(oc get pods -n $NAMESPACE -l $APP_LABEL --field-selector=status.phase=Running -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

if [ -z "$POD_NAME" ]; then
    echo "❌ Error: No running processor pod found in namespace $NAMESPACE"
    echo "   Make sure the processor deployment is running and you're logged into OpenShift"
    exit 1
fi

echo "✅ Found running processor pod: $POD_NAME"

# Function to clear directory with exclusions
clear_directory_with_exclusions() {
    local dir_path="$1"
    local description="$2"
    local pattern="$3"
    local exclude_pattern="$4"
    
    echo "🗑️  Clearing $description..."
    
    if [ -n "$exclude_pattern" ]; then
        # Count files before deletion (excluding protected files)
        local file_count=$(oc exec $POD_NAME -n $NAMESPACE -c starlight -- find "$dir_path" -name "$pattern" ! -name "$exclude_pattern" 2>/dev/null | wc -l)
        
        if [ "$file_count" -eq 0 ]; then
            echo "   ✅ $description already empty (excluding protected files)"
        else
            echo "   📁 Found $file_count files to delete (excluding protected files)"
            oc exec $POD_NAME -n $NAMESPACE -c starlight -- find "$dir_path" -name "$pattern" ! -name "$exclude_pattern" -delete
            echo "   ✅ Cleared $file_count files from $description"
        fi
    else
        # Count files before deletion
        local file_count=$(oc exec $POD_NAME -n $NAMESPACE -c starlight -- find "$dir_path" -name "$pattern" 2>/dev/null | wc -l)
        
        if [ "$file_count" -eq 0 ]; then
            echo "   ✅ $description already empty"
        else
            echo "   📁 Found $file_count files to delete"
            oc exec $POD_NAME -n $NAMESPACE -c starlight -- find "$dir_path" -name "$pattern" -delete
            echo "   ✅ Cleared $file_count files from $description"
        fi
    fi
}

# Clear all directories
echo ""
echo "🚀 Starting cleanup..."

# Clear batch_info files (including per-pod directories)
echo "🗑️  Clearing batch_info files (including per-pod directories)..."
batch_info_count=$(oc exec $POD_NAME -n $NAMESPACE -c starlight -- find "/processing_data/" -type d -name "batch_info*" -exec find {} -name "*.txt" \; 2>/dev/null | wc -l)

if [ "$batch_info_count" -eq 0 ]; then
    echo "   ✅ batch_info files already empty"
else
    echo "   📁 Found $batch_info_count files to delete"
    oc exec $POD_NAME -n $NAMESPACE -c starlight -- find "/processing_data/" -type d -name "batch_info*" -exec find {} -name "*.txt" -delete \;
    echo "   ✅ Cleared $batch_info_count files from batch_info directories"
fi

# Clear starlight directories
clear_directory_with_exclusions "/processing_data/starlight/data/input/" "starlight input files" "*.txt" ""
clear_directory_with_exclusions "/processing_data/starlight/data/output/" "starlight output files" "*.txt" ""

# Clear starlight processed files in subdirectories
echo "🗑️  Clearing starlight processed files..."
starlight_processed_count=$(oc exec $POD_NAME -n $NAMESPACE -c starlight -- find "/processing_data/starlight/data/processed/" -mindepth 2 -name "*.txt" 2>/dev/null | wc -l)

if [ "$starlight_processed_count" -eq 0 ]; then
    echo "   ✅ starlight processed files already empty"
else
    echo "   📁 Found $starlight_processed_count files to delete"
    oc exec $POD_NAME -n $NAMESPACE -c starlight -- find "/processing_data/starlight/data/processed/" -mindepth 2 -name "*.txt" -delete
    echo "   ✅ Cleared $starlight_processed_count files from starlight processed directories"
fi

# Clear ppxf directories (excluding mask.txt)
clear_directory_with_exclusions "/processing_data/ppxf/data/input/" "ppxf input files" "*.txt" "mask.txt"
clear_directory_with_exclusions "/processing_data/ppxf/data/output/" "ppxf output files" "*.txt" ""
clear_directory_with_exclusions "/processing_data/ppxf/data/processed/" "ppxf processed files" "*.txt" "mask.txt"

# Verify cleanup
echo ""
echo "🔍 Verifying cleanup..."

# Count remaining files (excluding protected files: processlist.txt, processlist-*.txt, mask.txt)
total_remaining=$(oc exec $POD_NAME -n $NAMESPACE -c starlight -- sh -c 'find /processing_data/ -name "*.txt" | grep -v -E "(processlist.*\.txt|mask\.txt)"' 2>/dev/null | wc -l)
protected_files=$(oc exec $POD_NAME -n $NAMESPACE -c starlight -- sh -c 'find /processing_data/ -name "*.txt" | grep -E "(processlist.*\.txt|mask\.txt)"' 2>/dev/null | wc -l)

if [ "$total_remaining" -eq 0 ]; then
    echo "✅ SUCCESS: All experiment files cleared! (0 files remaining)"
    if [ "$protected_files" -gt 0 ]; then
        echo "   📋 Protected files preserved: $protected_files (processlist*.txt, mask.txt)"
    fi
else
    echo "⚠️  WARNING: $total_remaining experiment files still remain"
    echo "   Listing remaining files (excluding protected files):"
    oc exec $POD_NAME -n $NAMESPACE -c starlight -- sh -c 'find /processing_data/ -name "*.txt" | grep -v -E "(processlist.*\.txt|mask\.txt)"' 2>/dev/null | head -20
    if [ "$total_remaining" -gt 20 ]; then
        echo "   ... and $((total_remaining - 20)) more files"
    fi
fi

echo ""
echo "🎉 Shared volume cleanup completed!"
echo "   Pod used: $POD_NAME"
echo "   Protected files: processlist*.txt, mask.txt (preserved)"
echo "   Ready for fresh experiment run" 
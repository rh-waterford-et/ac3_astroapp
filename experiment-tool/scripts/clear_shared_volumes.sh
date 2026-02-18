#!/bin/bash

# Script to clear UC3 shared volume directories for fresh experiment runs
# Assumes you're already logged into OpenShift
# Deletes: batch_info dirs, processlist files, .in files, all data files
# Preserves: mask.txt only (processlist files are recreated by active pods)

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
batch_info_count=$(oc exec $POD_NAME -n $NAMESPACE -c starlight -- sh -c 'find /processing_data -type d -name "batch_info*" 2>/dev/null | wc -l')

if [ "$batch_info_count" -eq 0 ]; then
    echo "   ✅ batch_info directories already empty"
else
    echo "   📁 Found $batch_info_count batch_info directories"
    oc exec $POD_NAME -n $NAMESPACE -c starlight -- sh -c 'rm -rf /processing_data/batch_info* 2>/dev/null'
    echo "   ✅ Cleared all batch_info directories"
fi

# Clear failed_files.log (from previous runs)
echo "🗑️  Clearing failed_files.log..."
oc exec $POD_NAME -n $NAMESPACE -c starlight -- sh -c 'rm -f /processing_data/failed_files.log 2>/dev/null' && \
    echo "   ✅ Cleared failed_files.log" || echo "   ✅ No failed_files.log to clear"

# Clear processlist files (from old pods)
echo "🗑️  Clearing old processlist files..."
processlist_count=$(oc exec $POD_NAME -n $NAMESPACE -c starlight -- sh -c 'find /processing_data -name "processlist*.txt" 2>/dev/null | wc -l')

if [ "$processlist_count" -eq 0 ]; then
    echo "   ✅ No processlist files to clear"
else
    echo "   📁 Found $processlist_count processlist files to delete"
    oc exec $POD_NAME -n $NAMESPACE -c starlight -- sh -c 'find /processing_data -name "processlist*.txt" -delete'
    echo "   ✅ Cleared $processlist_count processlist files (will be recreated by active pods)"
fi

# Clear .in files (job input files)
echo "🗑️  Clearing old .in files..."
infile_count=$(oc exec $POD_NAME -n $NAMESPACE -c starlight -- sh -c 'find /processing_data/starlight/runtime/infiles -name "*.in" 2>/dev/null | wc -l')

if [ "$infile_count" -eq 0 ]; then
    echo "   ✅ No .in files to clear"
else
    echo "   📁 Found $infile_count .in files to delete"
    oc exec $POD_NAME -n $NAMESPACE -c starlight -- sh -c 'rm -f /processing_data/starlight/runtime/infiles/*.in'
    echo "   ✅ Cleared $infile_count .in files"
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

# Count remaining files (excluding only mask.txt)
total_remaining=$(oc exec $POD_NAME -n $NAMESPACE -c starlight -- sh -c 'find /processing_data/ -name "*.txt" ! -name "mask.txt" 2>/dev/null | wc -l')

if [ "$total_remaining" -eq 0 ]; then
    echo "✅ SUCCESS: All experiment files cleared! (0 files remaining)"
    echo "   📋 Protected: mask.txt only"
else
    echo "⚠️  WARNING: $total_remaining files still remain"
    echo "   Listing remaining files:"
    oc exec $POD_NAME -n $NAMESPACE -c starlight -- sh -c 'find /processing_data/ -name "*.txt" ! -name "mask.txt"' 2>/dev/null | head -20
    if [ "$total_remaining" -gt 20 ]; then
        echo "   ... and $((total_remaining - 20)) more files"
    fi
fi

echo ""
echo "🎉 Shared volume cleanup completed!"
echo "   Pod used: $POD_NAME"
echo "   Protected: mask.txt only"
echo "   Note: Active pods will recreate their processlist files automatically" 
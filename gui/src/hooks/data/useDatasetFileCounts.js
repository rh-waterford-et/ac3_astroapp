import { useState, useEffect, useCallback, useRef } from 'react';
import { getDatasetFilesUnified } from '../../services/api';

/**
 * Hook to fetch file counts for all datasets (except the selected one)
 * This enables progress calculation for all datasets without loading full file lists
 */
export const useDatasetFileCounts = (datasets, selectedDataset, processorType) => {
  const [counts, setCounts] = useState({}); // { datasetId: { input: number, output: number } }
  const [loading, setLoading] = useState(false);
  const abortControllerRef = useRef(null);

  const fetchFileCount = useCallback(async (datasetId, fileType, signal) => {
    try {
      // Use limit=1 to only fetch metadata (pagination.total)
      const response = await getDatasetFilesUnified(
        datasetId, 
        processorType, 
        fileType, 
        0, 
        1,
        signal
      );
      
      const count = response.pagination.total || 0;
      return count;
    } catch (error) {
      if (error.name === 'AbortError') {
        throw error; // Re-throw to handle in loadCounts
      }
      return 0;
    }
  }, [processorType]);

  const loadCounts = useCallback(async () => {
    if (!datasets || datasets.length === 0) {
      return;
    }
    
    // Abort any previous requests
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
    }
    
    // Create new abort controller
    abortControllerRef.current = new AbortController();
    const currentAbortController = abortControllerRef.current;
    
    setLoading(true);
    const newCounts = {};

    try {
      // Fetch counts for ALL datasets (including selected one)
      // This keeps progress bars independent from dataset management pane
      const fetchPromises = datasets.map(async (dataset) => {
        if (currentAbortController.signal.aborted) {
          return; // Skip if already aborted
        }
        
        try {
          const [inputCount, outputCount] = await Promise.all([
            fetchFileCount(dataset.id, 'input', currentAbortController.signal),
            fetchFileCount(dataset.id, 'output', currentAbortController.signal)
          ]);
          
          newCounts[dataset.id] = {
            input: inputCount,
            output: outputCount
          };
        } catch (error) {
          if (error.name === 'AbortError') {
            throw error; // Propagate abort
          }
          // Otherwise, skip this dataset
        }
      });

      await Promise.all(fetchPromises);
      
      // Only update state if not aborted
      if (!currentAbortController.signal.aborted) {
        setCounts(newCounts);
      }
    } catch (error) {
      // Silently handle aborts and errors
    } finally {
      setLoading(false);
    }
  }, [datasets, fetchFileCount, processorType]);

  // Load counts when datasets or selection changes
  useEffect(() => {
    loadCounts();
    
    // Cleanup: abort on unmount or when dependencies change
    return () => {
      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }
    };
  }, [loadCounts, processorType]);

  return {
    counts,
    loading,
    refresh: loadCounts
  };
};


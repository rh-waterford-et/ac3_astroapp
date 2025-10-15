import { useState, useEffect, useCallback } from 'react';
import { getDatasetFilesUnified } from '../../services/api';

/**
 * Hook to fetch file counts for all datasets (except the selected one)
 * This enables progress calculation for all datasets without loading full file lists
 */
export const useDatasetFileCounts = (datasets, selectedDataset, processorType) => {
  const [counts, setCounts] = useState({}); // { datasetId: { input: number, output: number } }
  const [loading, setLoading] = useState(false);

  const fetchFileCount = useCallback(async (datasetId, fileType) => {
    try {
      // Use limit=1 to only fetch metadata (pagination.total)
      const response = await getDatasetFilesUnified(
        datasetId, 
        processorType, 
        fileType, 
        0, 
        1
      );
      return response.pagination.total || 0;
    } catch (error) {
      console.error(`Failed to fetch ${fileType} count for ${datasetId}:`, error);
      return 0;
    }
  }, [processorType]);

  const loadCounts = useCallback(async () => {
    if (!datasets || datasets.length === 0) return;
    
    setLoading(true);
    const newCounts = {};

    // Fetch counts for ALL datasets (including selected one)
    // This keeps progress bars independent from dataset management pane
    const fetchPromises = datasets.map(async (dataset) => {
      const [inputCount, outputCount] = await Promise.all([
        fetchFileCount(dataset.id, 'input'),
        fetchFileCount(dataset.id, 'output')
      ]);
      
      newCounts[dataset.id] = {
        input: inputCount,
        output: outputCount
      };
    });

    await Promise.all(fetchPromises);
    setCounts(newCounts);
    setLoading(false);
  }, [datasets, fetchFileCount]);

  // Load counts when datasets or selection changes
  useEffect(() => {
    loadCounts();
  }, [loadCounts]);

  return {
    counts,
    loading,
    refresh: loadCounts
  };
};


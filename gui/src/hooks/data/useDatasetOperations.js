import { useState, useEffect, useCallback, useRef } from 'react';
import { 
  getDatasets, 
  deleteDataset as apiDeleteDataset, 
  startProcessing as apiStartProcessing, 
  startSingleFileProcessing as apiStartSingleFileProcessing 
} from '../../services/api';

export const useDatasetOperations = (processorType) => {
  const [datasets, setDatasets] = useState([]);
  const [selectedDataset, setSelectedDataset] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const isRefreshing = useRef(false);
  const abortController = useRef(null);

  // Cancel pending requests
  const cancelRequests = useCallback(() => {
    if (abortController.current) {
      abortController.current.abort();
      abortController.current = null;
    }
  }, []);

  // Load datasets with auto-selection logic
  const loadDatasets = useCallback(async (silent = false, forceAutoSelect = false) => {
    if (isRefreshing.current) return;
    
    isRefreshing.current = true;
    
    // Only show loading spinner for user actions
    if (!silent) {
      setLoading(true);
    }
    
    try {
      const datasetNames = await getDatasets(processorType);
      const datasetObjects = datasetNames
        .sort((a, b) => a.toLowerCase().localeCompare(b.toLowerCase()))
        .map(name => ({
          id: name,
          name: name,
          status: 'ready',
          progress: 0,
          stage: 'Ready for processing'
        }));

      setDatasets(datasetObjects);

      // Auto-select first dataset if none selected, on processor switch, or forced
      setSelectedDataset(prevSelected => {
        if ((forceAutoSelect || !prevSelected || prevSelected === '') && datasetObjects.length > 0) {
          return datasetObjects[0].id;
        } else {
          return prevSelected;
        }
      });
    } catch (err) {
      // Only show error for user actions
      if (!silent) {
        setError(err.message || 'Failed to load datasets');
      }
    } finally {
      // Only hide loading spinner if we showed it
      if (!silent) {
        setLoading(false);
      }
      isRefreshing.current = false;
    }
  }, [processorType]);

  // Delete dataset with selection management
  const deleteDataset = useCallback(async (datasetId, datasetName) => {
    const confirmed = window.confirm(`Are you sure you want to delete the dataset "${datasetName}"?`);
    
    if (!confirmed) {
      return { success: false, cancelled: true };
    }

    try {
      setLoading(true);
      
      const result = await apiDeleteDataset(datasetId, processorType);
      
      if (result.success) {
        
        // Handle selection logic
        if (selectedDataset === datasetId) {
          // Find remaining datasets (excluding the deleted one)
          const remainingDatasets = datasets.filter(d => d.id !== datasetId);
          
          if (remainingDatasets.length > 0) {
            // Select the first remaining dataset (alphabetically sorted)
            const nextDataset = remainingDatasets[0];
            setSelectedDataset(nextDataset.id);
          } else {
            // No datasets left, clear everything
            setSelectedDataset('');
            setError(null);
          }
        }
        
        // Refresh datasets list to get updated list
        const shouldForceAutoSelect = !selectedDataset || selectedDataset === '';
        await loadDatasets(false, shouldForceAutoSelect);
        
        return { success: true };
      } else {
        setError(result.message || 'Failed to delete dataset');
        return { success: false, error: result.message };
      }
    } catch (error) {
      setError(error.message || 'Failed to delete dataset');
      return { success: false, error: error.message };
    } finally {
      setLoading(false);
    }
  }, [datasets, selectedDataset, processorType, loadDatasets]);

  // Start processing for entire dataset
  const startProcessing = useCallback(async (datasetName) => {
    const confirmed = window.confirm(`Start processing dataset "${datasetName}" with ${processorType}?`);
    
    if (!confirmed) {
      return { success: false, cancelled: true };
    }

    try {
      
      const result = await apiStartProcessing(datasetName, processorType);
      
      if (result.success) {
        return { success: true };
      } else {
        return { success: false, error: result.message };
      }
    } catch (error) {
      return { success: false, error: error.message };
    }
  }, [processorType]);

  // Start processing for single file
  const startSingleFileProcessing = useCallback(async (fileName) => {
    if (!selectedDataset) {
      return { success: false, error: 'No dataset selected' };
    }

    const confirmed = window.confirm(`Are you sure you want to process the file "${fileName}" with ${processorType}?`);
    
    if (!confirmed) {
      return { success: false, cancelled: true };
    }

    try {
      
      const result = await apiStartSingleFileProcessing(selectedDataset, fileName, processorType);
      
      if (result.success) {
        return { success: true, message: result.message };
      } else {
        return { success: false, error: result.message };
      }
    } catch (error) {
      return { success: false, error: error.message };
    }
  }, [selectedDataset, processorType]);

  // Handle dataset creation callback
  const handleDatasetCreated = useCallback((datasetName) => {
    loadDatasets(true, false); // Silent reload, no force auto-select
  }, [loadDatasets]);

  // Manual refresh
  const refresh = useCallback(async () => {
    setError(null); // Clear any previous errors
    await loadDatasets(false, false);
  }, [loadDatasets]);

  // Clear all data when processor type changes
  useEffect(() => {
    
    // Cancel any pending requests first
    cancelRequests();
    
    // Clear all state immediately
    setDatasets([]);
    setSelectedDataset('');
    setError(null);
    setLoading(false);
    isRefreshing.current = false;
    
    
    // Start fresh - force auto-select first dataset on processor switch
    loadDatasets(false, true); // forceAutoSelect = true
  }, [processorType, cancelRequests, loadDatasets]);

  // Cleanup: cancel requests when component unmounts
  useEffect(() => {
    return () => {
      cancelRequests();
    };
  }, [cancelRequests]);

  return {
    // State
    datasets,
    selectedDataset,
    loading,
    error,
    
    // Actions
    setSelectedDataset,
    loadDatasets,
    deleteDataset,
    startProcessing,
    startSingleFileProcessing,
    handleDatasetCreated,
    refresh,
    
    // Utils
    selectedDatasetInfo: datasets.find(dataset => dataset.id === selectedDataset),
    datasetName: datasets.find(dataset => dataset.id === selectedDataset)?.name || 'Unknown'
  };
}; 
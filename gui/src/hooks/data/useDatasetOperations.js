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
    console.log('🚫 Cancelling dataset requests');
    if (abortController.current) {
      abortController.current.abort();
      abortController.current = null;
    }
  }, []);

  // Load datasets with auto-selection logic
  const loadDatasets = useCallback(async (silent = false, forceAutoSelect = false) => {
    if (isRefreshing.current) return;
    
    console.log(silent ? '🔄 Background refresh datasets for' : '🔄 Loading datasets for', processorType);
    console.log('🔧 loadDatasets called with forceAutoSelect:', forceAutoSelect, 'selectedDataset:', selectedDataset);
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
          console.log('🎯 Auto-selecting first dataset:', datasetObjects[0].id, 'Reason: forceAutoSelect =', forceAutoSelect, 'prevSelected =', prevSelected);
          return datasetObjects[0].id;
        } else {
          console.log('❌ Not auto-selecting. forceAutoSelect:', forceAutoSelect, 'prevSelected:', prevSelected, 'datasetObjects.length:', datasetObjects.length);
          return prevSelected;
        }
      });
    } catch (err) {
      console.error('❌ Failed to load datasets:', err);
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
      console.log('🗑️ Starting dataset deletion:', datasetId);
      setLoading(true);
      
      const result = await apiDeleteDataset(datasetId, processorType);
      
      if (result.success) {
        console.log('Dataset deleted successfully:', datasetId);
        
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
        console.log('🔧 Dataset deletion - shouldForceAutoSelect:', shouldForceAutoSelect, 'selectedDataset:', selectedDataset);
        await loadDatasets(false, shouldForceAutoSelect);
        
        console.log(`✅ Dataset "${datasetName}" deleted successfully`);
        return { success: true };
      } else {
        console.error('❌ Failed to delete dataset:', result.message);
        setError(result.message || 'Failed to delete dataset');
        return { success: false, error: result.message };
      }
    } catch (error) {
      console.error('❌ Error deleting dataset:', error.message);
      setError(error.message || 'Failed to delete dataset');
      return { success: false, error: error.message };
    } finally {
      setLoading(false);
      console.log('🏁 Dataset deletion process completed');
    }
  }, [datasets, selectedDataset, processorType, loadDatasets]);

  // Start processing for entire dataset
  const startProcessing = useCallback(async (datasetName) => {
    const confirmed = window.confirm(`Start processing dataset "${datasetName}" with ${processorType}?`);
    
    if (!confirmed) {
      return { success: false, cancelled: true };
    }

    try {
      console.log('Starting processing for dataset:', datasetName);
      
      const result = await apiStartProcessing(datasetName, processorType);
      
      if (result.success) {
        console.log('Processing started successfully for:', datasetName);
        return { success: true };
      } else {
        console.error('Failed to start processing:', result.message);
        return { success: false, error: result.message };
      }
    } catch (error) {
      console.error('Error starting processing:', error.message);
      return { success: false, error: error.message };
    }
  }, [processorType]);

  // Start processing for single file
  const startSingleFileProcessing = useCallback(async (fileName) => {
    if (!selectedDataset) {
      console.error('No dataset selected for single file processing');
      return { success: false, error: 'No dataset selected' };
    }

    const confirmed = window.confirm(`Are you sure you want to process the file "${fileName}" with ${processorType}?`);
    
    if (!confirmed) {
      return { success: false, cancelled: true };
    }

    try {
      console.log('Processing single file:', fileName, 'in dataset:', selectedDataset, 'with processor:', processorType);
      
      const result = await apiStartSingleFileProcessing(selectedDataset, fileName, processorType);
      
      if (result.success) {
        console.log('Single file processing started successfully:', result.message);
        return { success: true, message: result.message };
      } else {
        console.error('Failed to start single file processing:', result.message);
        return { success: false, error: result.message };
      }
    } catch (error) {
      console.error('Error starting single file processing:', error);
      return { success: false, error: error.message };
    }
  }, [selectedDataset, processorType]);

  // Handle dataset creation callback
  const handleDatasetCreated = useCallback((datasetName) => {
    console.log('✅ Dataset created:', datasetName, '- refreshing...');
    loadDatasets(true, false); // Silent reload, no force auto-select
  }, [loadDatasets]);

  // Manual refresh
  const refresh = useCallback(async () => {
    console.log('🔄 Manual refresh: Datasets');
    setError(null); // Clear any previous errors
    await loadDatasets(false, false);
  }, [loadDatasets]);

  // Clear all data when processor type changes
  useEffect(() => {
    console.log('🔄 ProcessorType changed to:', processorType, '- clearing dataset state');
    console.log('🔍 Current selectedDataset before clearing:', selectedDataset);
    
    // Cancel any pending requests first
    cancelRequests();
    
    // Clear all state immediately
    setDatasets([]);
    setSelectedDataset('');
    setError(null);
    setLoading(false);
    isRefreshing.current = false;
    
    console.log('🧹 Dataset state cleared, selectedDataset set to empty string');
    
    // Start fresh - force auto-select first dataset on processor switch
    console.log('🔄 Loading datasets for processor:', processorType);
    loadDatasets(false, true); // forceAutoSelect = true
  }, [processorType, cancelRequests, loadDatasets]);

  // Cleanup: cancel requests when component unmounts
  useEffect(() => {
    return () => {
      console.log('🧹 useDatasetOperations unmounting - cancelling requests');
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
import React, { useState, useEffect, useCallback, useRef } from 'react';
import PropTypes from 'prop-types';
// Re-enabling FileUpload component
import FileUpload from './FileUpload';
import PipelineProgress from './PipelineProgress';
import VirtualizedFileList from './VirtualizedFileList';
import { getDatasets, getDatasetFiles, getDatasetFilesUnified, getDatasetFilesListPaginated, getDatasetOutputFiles, getDatasetOutputFilesListPaginated, deleteDataset, deleteFile, startProcessing, startSingleFileProcessing } from '../services/api';
import { getProcessorConfig } from '../config/processorConfig';

function DatasetsList({ processorType }) {
  const [selectedDataset, setSelectedDataset] = useState('');
  
  // Helper functions for localStorage persistence
  const getStoredCollapseState = (key, defaultValue = false) => {
    try {
      const stored = localStorage.getItem(`pipeline-${key}-collapsed`);
      return stored !== null ? JSON.parse(stored) : defaultValue;
    } catch (error) {
      console.error(`Error reading ${key} collapse state:`, error);
      return defaultValue;
    }
  };

  const setStoredCollapseState = (key, value) => {
    try {
      localStorage.setItem(`pipeline-${key}-collapsed`, JSON.stringify(value));
    } catch (error) {
      console.error(`Error storing ${key} collapse state:`, error);
    }
  };

  // Initialize collapsed states from localStorage, defaulting to false (not collapsed)
  const [isUploadCollapsed, setIsUploadCollapsed] = useState(() => 
    getStoredCollapseState('upload', false)
  );
  const [isDatasetsCollapsed, setIsDatasetsCollapsed] = useState(() => 
    getStoredCollapseState('datasets', false)
  );
  const [isPipelineProgressCollapsed, setIsPipelineProgressCollapsed] = useState(() => 
    getStoredCollapseState('progress', false)
  );

  // Enhanced setters that also save to localStorage
  const toggleUploadCollapsed = () => {
    const newState = !isUploadCollapsed;
    setIsUploadCollapsed(newState);
    setStoredCollapseState('upload', newState);
  };

  const toggleDatasetsCollapsed = () => {
    const newState = !isDatasetsCollapsed;
    setIsDatasetsCollapsed(newState);
    setStoredCollapseState('datasets', newState);
  };

  const togglePipelineProgressCollapsed = () => {
    const newState = !isPipelineProgressCollapsed;
    setIsPipelineProgressCollapsed(newState);
    setStoredCollapseState('progress', newState);
  };

  // Simple state management
  const [datasets, setDatasets] = useState([]);
  const [inputFiles, setInputFiles] = useState([]);
  const [outputFiles, setOutputFiles] = useState([]);
  const [loading, setLoading] = useState(false);
  const [inputFilesLoading, setInputFilesLoading] = useState(false);
  const [outputFilesLoading, setOutputFilesLoading] = useState(false);
  const [inputFilesLoaded, setInputFilesLoaded] = useState(false);
  const [error, setError] = useState(null);
  const [datasetsError, setDatasetsError] = useState(null);
  const [inputFilesError, setInputFilesError] = useState(null);
  const [outputFilesError, setOutputFilesError] = useState(null);

  // Clean offset-based pagination state
  const [outputFilesPagination, setOutputFilesPagination] = useState({
    offset: 0,
    hasMore: false,
    loading: false,
    total: 0
  });

  const [inputFilesPagination, setInputFilesPagination] = useState({
    offset: 0,
    hasMore: false,
    loading: false,
    total: 0
  });

  // Simple refresh tracking (no auto-refresh timer)
  const isRefreshing = useRef(false);
  const fileUploadRef = useRef(null);
  
  // AbortController refs for cancelling requests
  const inputFilesAbortController = useRef(null);
  const outputFilesAbortController = useRef(null);
  const datasetsAbortController = useRef(null);

  // Helper function to cancel all pending requests
  const cancelAllRequests = useCallback(() => {
    console.log('🚫 Cancelling all pending requests');
    
    if (inputFilesAbortController.current) {
      inputFilesAbortController.current.abort();
      inputFilesAbortController.current = null;
    }
    
    if (outputFilesAbortController.current) {
      outputFilesAbortController.current.abort();
      outputFilesAbortController.current = null;
    }
    
    if (datasetsAbortController.current) {
      datasetsAbortController.current.abort();
      datasetsAbortController.current = null;
    }
  }, []);

    // Clear all data when processor type changes
  useEffect(() => {
    console.log('🔄 ProcessorType changed to:', processorType, '- clearing all state');
    console.log('🔍 Current selectedDataset before clearing:', selectedDataset);
    
    // Cancel any pending requests first
    cancelAllRequests();
    
    // Clear all state immediately
    setDatasets([]);
    setInputFiles([]);
    setOutputFiles([]);
    setSelectedDataset('');
    setError(null);
    setDatasetsError(null);
    setInputFilesError(null);
    setOutputFilesError(null);
    setInputFilesLoading(false);
    setOutputFilesLoading(false);
    setInputFilesLoaded(false);
    setOutputFilesPagination({ page: 0, hasMore: false, loading: false, total: 0 });
    setInputFilesPagination({ page: 0, hasMore: false, loading: false, total: 0 });
    isRefreshing.current = false;
    
    console.log('🧹 State cleared, selectedDataset set to empty string');
    
    // Clear file upload queue
    if (fileUploadRef.current) {
      fileUploadRef.current.clearAll();
    }
    
    // Start fresh - force auto-select first dataset on processor switch
    console.log('🔄 Loading datasets for processor:', processorType);
    loadDatasets(false, true); // forceAutoSelect = true
  }, [processorType]);

  // Load datasets only
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
      if ((forceAutoSelect || !selectedDataset || selectedDataset === '') && datasetObjects.length > 0) {
        console.log('🎯 Auto-selecting first dataset:', datasetObjects[0].id, 'Reason: forceAutoSelect =', forceAutoSelect, 'selectedDataset =', selectedDataset);
        setSelectedDataset(datasetObjects[0].id);
      } else {
        console.log('❌ Not auto-selecting. forceAutoSelect:', forceAutoSelect, 'selectedDataset:', selectedDataset, 'datasetObjects.length:', datasetObjects.length);
      }
    } catch (err) {
      console.error('❌ Failed to load datasets:', err);
      // Only show error for user actions
      if (!silent) {
        setDatasetsError(err.message || 'Failed to load datasets');
      }
    } finally {
      // Only hide loading spinner if we showed it
      if (!silent) {
        setLoading(false);
      }
      isRefreshing.current = false;
    }
  }, [processorType]);

  // Load initial input files using unified API
  const loadInputFiles = useCallback(async (silent = false) => {
    if (!selectedDataset || isRefreshing.current) return;
    
    console.log('🔄 Loading input files for:', selectedDataset);
    isRefreshing.current = true;
    
    // Cancel any existing input files request
    if (inputFilesAbortController.current) {
      inputFilesAbortController.current.abort();
    }
    
    // Create new AbortController for this request
    inputFilesAbortController.current = new AbortController();
    
    if (!silent) {
      setInputFilesLoading(true);
    }
    
    try {
      const response = await getDatasetFilesUnified(selectedDataset, processorType, 'input', 0, 50, inputFilesAbortController.current.signal);

      // Process input files
      const inputFiles = response.files
        .filter(file => isValidFile(file.name))
        .map(file => ({
          name: file.name,
          size: formatFileSize(file.size),
          uploaded: file.timestamp,
          status: 'processed',
          key: file.key
        }));

      setInputFiles(inputFiles);
      setInputFilesPagination({
        offset: 50, // Next offset to load
        hasMore: response.pagination.hasMore,
        loading: false,
        total: response.pagination.total
      });
      setInputFilesLoaded(true);
    } catch (err) {
      if (err.name === 'AbortError') {
        console.log('🚫 Input files request cancelled');
        return;
      }
      
      console.error('❌ Failed to load input files:', err);
      if (!silent) {
        setInputFilesError(err.message || 'Failed to load input files');
      }
      setInputFilesPagination({ offset: 0, hasMore: false, loading: false, total: 0 });
    } finally {
      if (!silent) {
        setInputFilesLoading(false);
      }
      isRefreshing.current = false;
    }
  }, [selectedDataset, processorType]);

  // Load more input files using additive offset-based loading
  const loadMoreInputFiles = useCallback(async () => {
    if (!selectedDataset || inputFilesPagination.loading || !inputFilesPagination.hasMore) return;

    console.log('🔄 Loading more input files, offset:', inputFilesPagination.offset);
    
    setInputFilesPagination(prev => ({ ...prev, loading: true }));

    try {
      // Load 100 files per batch for subsequent loads (after initial 50)
      console.log(`📊 Loading 100 more input files (offset: ${inputFilesPagination.offset})`);
      
      const response = await getDatasetFilesUnified(
        selectedDataset, 
        processorType, 
        'input',
        inputFilesPagination.offset, 
        100,
        inputFilesAbortController.current?.signal
      );

      // Process new input files
      const newInputFiles = response.files
        .filter(file => isValidFile(file.name))
        .map(file => ({
          name: file.name,
          size: formatFileSize(file.size),
          uploaded: file.timestamp,
          status: 'processed',
          key: file.key
        }));

      // ADDITIVE: Append to existing files, never overwrite
      setInputFiles(prev => [...prev, ...newInputFiles]);
      setInputFilesPagination({
        offset: inputFilesPagination.offset + 100,
        hasMore: response.pagination.hasMore,
        loading: false,
        total: response.pagination.total
      });
    } catch (err) {
      console.error('❌ Failed to load more input files:', err);
      setInputFilesPagination(prev => ({ ...prev, loading: false }));
    }
  }, [selectedDataset, processorType, inputFilesPagination.offset, inputFilesPagination.loading, inputFilesPagination.hasMore]);

  // Smart auto-refresh: only update total count, never disrupt loaded files
  const refreshInputFilesCount = useCallback(async () => {
    if (!selectedDataset || isRefreshing.current || inputFilesPagination.loading) return;
    
    try {
      // Just check the first page to get updated total count
      const response = await getDatasetFilesUnified(selectedDataset, processorType, 'input', 0, 1);
      
      // Only update the total count if it has changed
      if (response.pagination.total !== inputFilesPagination.total) {
        console.log(`📊 Input files count updated: ${inputFilesPagination.total} → ${response.pagination.total}`);
        setInputFilesPagination(prev => ({
          ...prev,
          total: response.pagination.total,
          hasMore: prev.offset < response.pagination.total
        }));
      }
    } catch (err) {
      console.error('❌ Silent count refresh failed for input files:', err);
    }
  }, [selectedDataset, processorType, inputFilesPagination.total, inputFilesPagination.offset, inputFilesPagination.loading]);

  // Load initial output files with pagination
  const loadOutputFiles = useCallback(async (silent = false) => {
    if (!selectedDataset || isRefreshing.current) return;
    
    console.log('🔄 Loading output files for:', selectedDataset);
    isRefreshing.current = true;
    
    // Cancel any existing output files request
    if (outputFilesAbortController.current) {
      outputFilesAbortController.current.abort();
    }
    
    // Create new AbortController for this request
    outputFilesAbortController.current = new AbortController();
    
    if (!silent) {
      setOutputFilesLoading(true);
    }
    
    try {
      const response = await getDatasetFilesUnified(selectedDataset, processorType, 'output', 0, 50, outputFilesAbortController.current.signal);

      // Process output files
      const outputFiles = response.files
        .filter(file => isValidFile(file.name))
        .map(file => ({
          name: file.name,
          size: formatFileSize(file.size),
          uploaded: file.timestamp,
          status: 'completed',
          key: file.key
        }));

      setOutputFiles(outputFiles);
      setOutputFilesPagination({
        offset: 50, // Next offset to load
        hasMore: response.pagination.hasMore,
        loading: false,
        total: response.pagination.total
      });
    } catch (err) {
      if (err.name === 'AbortError') {
        console.log('🚫 Output files request cancelled');
        return;
      }
      
      console.error('❌ Failed to load output files:', err);
      if (!silent) {
        setOutputFilesError(err.message || 'Failed to load output files');
      }
      setOutputFilesPagination({ offset: 0, hasMore: false, loading: false, total: 0 });
    } finally {
      if (!silent) {
        setOutputFilesLoading(false);
      }
      isRefreshing.current = false;
    }
  }, [selectedDataset, processorType]);

  // Load more output files using additive offset-based loading
  const loadMoreOutputFiles = useCallback(async () => {
    if (!selectedDataset || outputFilesPagination.loading || !outputFilesPagination.hasMore) return;

    console.log('🔄 Loading more output files, offset:', outputFilesPagination.offset);
    
    setOutputFilesPagination(prev => ({ ...prev, loading: true }));

    try {
      // Load 100 files per batch for subsequent loads (after initial 50)
      console.log(`📊 Loading 100 more output files (offset: ${outputFilesPagination.offset})`);
      
      const response = await getDatasetFilesUnified(
        selectedDataset, 
        processorType, 
        'output',
        outputFilesPagination.offset, 
        100,
        outputFilesAbortController.current?.signal
      );

      // Process new output files
      const newOutputFiles = response.files
        .filter(file => isValidFile(file.name))
        .map(file => ({
          name: file.name,
          size: formatFileSize(file.size),
          uploaded: file.timestamp,
          status: 'completed',
          key: file.key
        }));

      // ADDITIVE: Append to existing files, never overwrite
      setOutputFiles(prev => [...prev, ...newOutputFiles]);
      setOutputFilesPagination({
        offset: outputFilesPagination.offset + 100,
        hasMore: response.pagination.hasMore,
        loading: false,
        total: response.pagination.total
      });
    } catch (err) {
      console.error('❌ Failed to load more output files:', err);
      setOutputFilesPagination(prev => ({ ...prev, loading: false }));
    }
  }, [selectedDataset, processorType, outputFilesPagination.offset, outputFilesPagination.loading, outputFilesPagination.hasMore]);

  // Smart auto-refresh: only update total count, never disrupt loaded files
  const refreshOutputFilesCount = useCallback(async () => {
    if (!selectedDataset || isRefreshing.current || outputFilesPagination.loading) return;
    
    try {
      // Just check the first page to get updated total count
      const response = await getDatasetFilesUnified(selectedDataset, processorType, 'output', 0, 1);
      
      // Only update the total count if it has changed
      if (response.pagination.total !== outputFilesPagination.total) {
        console.log(`📊 Output files count updated: ${outputFilesPagination.total} → ${response.pagination.total}`);
        setOutputFilesPagination(prev => ({
          ...prev,
          total: response.pagination.total,
          hasMore: prev.offset < response.pagination.total
        }));
      }
    } catch (err) {
      console.error('❌ Silent count refresh failed for output files:', err);
    }
  }, [selectedDataset, processorType, outputFilesPagination.total, outputFilesPagination.offset, outputFilesPagination.loading]);

  // Combined background refresh function
  const performBackgroundRefresh = useCallback(async () => {
    if (!selectedDataset || isRefreshing.current) return;
    
    console.log('🔄 Performing smart count refresh...');
    
    // Only refresh file counts, never disrupt loaded data
    await Promise.all([
      refreshInputFilesCount(),
      refreshOutputFilesCount()
    ]);
    
    console.log('✅ Smart count refresh completed');
  }, [selectedDataset, refreshInputFilesCount, refreshOutputFilesCount]);

  // Clean auto-refresh timer - count-only updates every 5 seconds

  // Auto-refresh timer - silent background updates every 5 seconds
  useEffect(() => {
    if (!selectedDataset) return;
    
    console.log('🔧 Setting up background refresh timer for dataset:', selectedDataset);
    
    const interval = setInterval(() => {
      performBackgroundRefresh();
    }, 5000); // Refresh every 5 seconds

    return () => {
      console.log('🧹 Clearing background refresh timer');
      clearInterval(interval);
    };
  }, [selectedDataset, performBackgroundRefresh]);

  // Cleanup: cancel all requests when component unmounts
  useEffect(() => {
    return () => {
      console.log('🧹 DatasetsList unmounting - cancelling all requests');
      cancelAllRequests();
    };
  }, [cancelAllRequests]);

  // Load input files when dataset selection changes
  useEffect(() => {
    if (selectedDataset) {
      setInputFilesLoaded(false);
      setInputFiles([]); // Clear input files when changing datasets
      setInputFilesPagination({ offset: 0, hasMore: false, loading: false, total: 0 }); // Reset input pagination
      setOutputFiles([]); // Clear output files when changing datasets
      setOutputFilesPagination({ offset: 0, hasMore: false, loading: false, total: 0 }); // Reset output pagination
      loadInputFiles();
    } else {
      setInputFiles([]);
      setOutputFiles([]);
      setInputFilesLoaded(false);
      setInputFilesPagination({ offset: 0, hasMore: false, loading: false, total: 0 });
      setOutputFilesPagination({ offset: 0, hasMore: false, loading: false, total: 0 });
    }
  }, [selectedDataset, loadInputFiles]);

  // Load output files after input files have loaded
  useEffect(() => {
    if (inputFilesLoaded && selectedDataset) {
      console.log('✅ Input files loaded, now loading output files');
      loadOutputFiles();
    }
  }, [inputFilesLoaded, selectedDataset, loadOutputFiles]);

   // Handle dataset creation callback
     const handleDatasetCreated = useCallback((datasetName) => {
    console.log('✅ Dataset created:', datasetName, '- refreshing...');
    loadDatasets(true, false); // Silent reload, no force auto-select
  }, [loadDatasets]);

  // Manual refresh functions
  const handleRefreshDatasets = useCallback(async () => {
    console.log('🔄 Manual refresh: Datasets');
    setDatasetsError(null); // Clear any previous errors for this pane
    await loadDatasets(false, false);
  }, [loadDatasets]);

  const handleRefreshInputFiles = useCallback(async () => {
    if (!selectedDataset) return;
    console.log('🔄 Manual refresh: Input files for', selectedDataset);
    
    // Reset pagination and reload from start, but preserve current total
    const currentTotal = inputFilesPagination.total;
    setInputFiles([]);
    setInputFilesPagination({ offset: 0, hasMore: false, loading: false, total: currentTotal });
    setInputFilesLoaded(false);
    setInputFilesError(null); // Clear any previous errors for this pane
    await loadInputFiles();
  }, [selectedDataset, loadInputFiles, inputFilesPagination.total]);

  const handleRefreshOutputFiles = useCallback(async () => {
    if (!selectedDataset) return;
    console.log('🔄 Manual refresh: Output files for', selectedDataset);
    
    // Reset pagination and reload from start, but preserve current total
    const currentTotal = outputFilesPagination.total;
    setOutputFiles([]);
    setOutputFilesPagination({ offset: 0, hasMore: false, loading: false, total: currentTotal });
    setOutputFilesError(null); // Clear any previous errors for this pane
    await loadOutputFiles();
  }, [selectedDataset, loadOutputFiles, outputFilesPagination.total]);

  const formatFileSize = (bytes) => {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  // Helper function to check if an item is a valid file (has extension, not a directory)
  const isValidFile = (fileName) => {
    // Filter out directory markers (ending with /)
    if (fileName.endsWith('/')) {
      return false;
    }
    
    // Filter out items without file extensions
    const lastDotIndex = fileName.lastIndexOf('.');
    if (lastDotIndex === -1 || lastDotIndex === fileName.length - 1) {
      return false;
    }
    
    // Additional check: file extension should be at least 1 character and at most 10 characters
    const extension = fileName.substring(lastDotIndex + 1);
    if (extension.length < 1 || extension.length > 10) {
      return false;
    }
    
    return true;
  };

  const truncateFileName = (fileName, maxLength = 50) => {
    if (fileName.length <= maxLength) return fileName;
    
    const lastDotIndex = fileName.lastIndexOf('.');
    if (lastDotIndex === -1) {
      // No extension, just truncate from the end
      return fileName.substring(0, maxLength - 3) + '...';
    }
    
    const extension = fileName.substring(lastDotIndex);
    const nameWithoutExt = fileName.substring(0, lastDotIndex);
    
    const availableLength = maxLength - extension.length - 3; // 3 for "..."
    
    if (availableLength <= 0) {
      // Extension is too long, just show extension
      return '...' + extension;
    }
    
    return nameWithoutExt.substring(0, availableLength) + '...' + extension;
  };

  const getDatasetStatusColor = (status) => {
    switch (status) {
      case 'completed': return '#68D391';
      case 'processing': return '#4FD1C5';
      case 'queued': return '#F6AD55';
      case 'ready': return '#9F7AEA';
      case 'error': return '#FC8181';
      default: return '#A0AEC0';
    }
  };

  const getFileStatusColor = (status) => {
    switch (status) {
      case 'ready': return '#4FD1C5';
      case 'processed': return '#4FD1C5';
      case 'completed': return '#4FD1C5'; // Match pipeline progress completed color
      case 'processing': return '#F6AD55';
      case 'queued': return '#9F7AEA';
      case 'error': return '#FC8181';
      default: return '#A0AEC0';
    }
  };

  const selectedDatasetInfo = datasets.find(dataset => dataset.id === selectedDataset);
  const datasetName = selectedDatasetInfo ? selectedDatasetInfo.name : 'Unknown';

  // Calculate expected output files based on processor type
  const getExpectedOutputCount = (inputCount, processorType) => {
    if (processorType === 'starlight') {
      return inputCount; // 1:1 ratio
    } else if (processorType === 'ppxf') {
      return inputCount * 5; // 1:5 ratio
    }
    return inputCount; // Default 1:1
  };

  // Check if processing is complete for a dataset
  const isProcessingComplete = (datasetName) => {
    // Only check if this is the currently selected dataset
    if (selectedDataset !== datasetName) {
      return false;
    }
    
    const datasetInputFiles = inputFiles.filter(file => file.name);
    const datasetOutputFiles = outputFiles.filter(file => file.name);
    
    const inputCount = datasetInputFiles.length;
    const outputCount = datasetOutputFiles.length;
    const expectedOutputCount = getExpectedOutputCount(inputCount, processorType);
    
    return inputCount > 0 && outputCount >= expectedOutputCount && expectedOutputCount > 0;
  };

  // Start processing function
  const handleStartProcessing = async (datasetName) => {
    const confirmed = window.confirm(`Start processing dataset "${datasetName}" with ${processorType}?`);
    
    if (!confirmed) {
      return;
    }

    try {
      console.log('Starting processing for dataset:', datasetName);
      
      const result = await startProcessing(datasetName, processorType);
      
      if (result.success) {
        console.log('Processing started successfully for:', datasetName);
      } else {
        console.error('Failed to start processing:', result.message);
      }
    } catch (error) {
      console.error('Error starting processing:', error.message);
    }
  };

  // Process individual file function
  const handleProcessFile = async (fileName) => {
    if (!selectedDataset) {
      console.error('No dataset selected for single file processing');
      return;
    }

    const confirmed = window.confirm(`Are you sure you want to process the file "${fileName}" with ${processorType}?`);
    
    if (!confirmed) {
      return;
    }

    try {
      console.log('Processing single file:', fileName, 'in dataset:', selectedDataset, 'with processor:', processorType);
      
      const result = await startSingleFileProcessing(selectedDataset, fileName, processorType);
      
      if (result.success) {
        console.log('Single file processing started successfully:', result.message);
        // Show success message to user
        alert(`✅ Processing started for file "${fileName}"`);
        
        // Optionally refresh data to show processing status
        // await loadFiles();
      } else {
        console.error('Failed to start single file processing:', result.message);
        alert(`❌ Failed to start processing: ${result.message}`);
      }
    } catch (error) {
      console.error('Error starting single file processing:', error);
      alert(`❌ Error starting processing: ${error.message}`);
    }
  };

  // Delete dataset function
  const handleDeleteDataset = async (datasetId, datasetName) => {
    const confirmed = window.confirm(`Are you sure you want to delete the dataset "${datasetName}"?`);
    
    if (!confirmed) {
      return;
    }

    try {
      console.log('🗑️ Starting dataset deletion:', datasetId);
      
      // Show loading states in all panes during deletion
      setLoading(true);
      setInputFilesLoading(true);
      setOutputFilesLoading(true);
      
      const result = await deleteDataset(datasetId, processorType);
      
      if (result.success) {
        console.log('Dataset deleted successfully:', datasetId);
        
        // Handle selection logic and refresh all panes
        if (selectedDataset === datasetId) {
          // Clear current files and pagination immediately
          setInputFiles([]);
          setOutputFiles([]);
          setInputFilesLoaded(false);
          setInputFilesPagination({ offset: 0, hasMore: false, loading: false, total: 0 });
          setOutputFilesPagination({ offset: 0, hasMore: false, loading: false, total: 0 });
          
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
            setDatasetsError(null);
            setInputFilesError(null);
            setOutputFilesError(null);
          }
        }
        
        // Refresh datasets list to get updated list
        // Force auto-selection if no dataset is currently selected
        const shouldForceAutoSelect = !selectedDataset || selectedDataset === '';
        console.log('🔧 Dataset deletion - shouldForceAutoSelect:', shouldForceAutoSelect, 'selectedDataset:', selectedDataset);
        await loadDatasets(false, shouldForceAutoSelect);
        
        // If we have a selected dataset after deletion, refresh its files
        if (selectedDataset && selectedDataset !== datasetId) {
          // Force refresh the input files for the currently selected dataset
          await loadInputFiles();
        }
        
        // Give user feedback
        console.log(`✅ Dataset "${datasetName}" deleted successfully - all panes refreshed`);
      } else {
        console.error('❌ Failed to delete dataset:', result.message);
        setError(result.message || 'Failed to delete dataset');
      }
    } catch (error) {
      console.error('❌ Error deleting dataset:', error.message);
      setError(error.message || 'Failed to delete dataset');
    } finally {
      // Always clear loading states when deletion is complete
      setLoading(false);
      setInputFilesLoading(false);
      setOutputFilesLoading(false);
      console.log('🏁 Dataset deletion process completed');
    }
  };

  // Delete file function
  const handleDeleteFile = async (fileKey, fileName, isInputFile = true) => {
    const confirmed = window.confirm(`Are you sure you want to delete the file "${fileName}"?`);
    
    if (!confirmed) {
      return;
    }

    try {
      console.log('Deleting file:', fileKey);
      
      // Immediately remove the file from the UI for instant feedback
      if (isInputFile) {
        setInputFiles(prevFiles => prevFiles.filter(file => file.key !== fileKey));
      } else {
        setOutputFiles(prevFiles => prevFiles.filter(file => file.key !== fileKey));
      }
      
      const result = await deleteFile(fileKey, processorType);
      
      if (result.success) {
        console.log('File deleted successfully');
        // Remove the file from the UI optimistically
        if (isInputFile) {
          setInputFiles(prev => prev.filter(f => f.key !== fileKey));
        } else {
          setOutputFiles(prev => prev.filter(f => f.key !== fileKey));
        }
      } else {
        console.error('Failed to delete file:', result.message);
        
        // Restore the file in the UI if deletion failed, then refresh
        await loadFiles();
      }
    } catch (error) {
      console.error('Error deleting file:', error);
      
      // Restore the correct state if deletion failed
      await loadFiles();
    }
  };

  return (
    <div className="pipeline-wrapper">
      {/* File Upload Container */}
      <div style={{ marginBottom: '0.75rem' }}>
        <FileUpload 
          ref={fileUploadRef}
          isCollapsed={isUploadCollapsed} 
          onToggleCollapse={toggleUploadCollapsed} 
          processorType={processorType}
          onDatasetCreated={handleDatasetCreated}
        />
      </div>
      
      {/* Three-pane layout Container */}
      <div className="pipeline-container" style={{ marginBottom: '0.75rem' }}>
        {/* Parent Header for 3-pane section */}
        <div className="pane-header">
          <div className="pane-header-left">
            <button 
              className="collapse-toggle"
              onClick={toggleDatasetsCollapsed}
              title={isDatasetsCollapsed ? "Expand Dataset Management" : "Collapse Dataset Management"}
            >
              <span className={`toggle-icon ${isDatasetsCollapsed ? 'collapsed' : ''}`}>
                {isDatasetsCollapsed ? '▲' : '▼'}
              </span>
            </button>
            <h3>Dataset Management</h3>
          </div>
        </div>
        
        {!isDatasetsCollapsed && (
          <div className="pipeline-panes">
            
            {/* Left Pane - Dataset Selection */}
            <div className="pipeline-pane datasets-pane">
              <div className="pane-header">
                <div className="pane-header-left">
                  <h3>Datasets</h3>
                </div>
                <div className="pane-header-right">
                  <button 
                    className="refresh-btn" 
                    onClick={handleRefreshDatasets}
                    disabled={loading}
                    title="Refresh datasets"
                  >
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                      <path d="M23 4v6h-6"/>
                      <path d="M1 20v-6h6"/>
                      <path d="m3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
                    </svg>
                  </button>
                  <div className="pane-count">{datasets.length}</div>
                </div>
              </div>
              <div className="pane-content">
              {loading ? (
                <div className="astro-loading-compact" style={{ minHeight: '150px' }}>
                  <div className="astro-loader-galaxy" style={{ width: '20px', height: '20px' }}></div>
                  <div className="astro-loading-text" style={{ fontSize: '12px' }}>Loading datasets...</div>
                </div>
              ) : datasetsError ? (
                <div className="empty-pane">
                  <div className="empty-icon">❌</div>
                  <p>Error loading datasets</p>
                  <p style={{ fontSize: '12px', color: '#FF6B6B' }}>{datasetsError}</p>
                </div>
              ) : datasets.length > 0 ? (
                datasets.map(dataset => {
                  const processingComplete = isProcessingComplete(dataset.name);
                  
                  return (
                    <div key={dataset.id} className={`dataset-item-container ${selectedDataset === dataset.id ? 'active' : ''}`}>
                      <button
                        className="dataset-item"
                        onClick={() => setSelectedDataset(dataset.id)}
                      >
                        <div className="dataset-info">
                          <div className="dataset-name">{dataset.name}</div>
                        </div>
                      </button>
                      <div className="dataset-actions">
                        <button
                          className="dataset-process-btn"
                          onClick={(e) => {
                            e.stopPropagation();
                            if (!processingComplete) {
                              handleStartProcessing(dataset.name);
                            }
                          }}
                          disabled={processingComplete}
                          title={processingComplete ? 'Processing complete' : `Start processing "${dataset.name}"`}
                          style={{
                            opacity: processingComplete ? 0.6 : 1,
                            cursor: processingComplete ? 'not-allowed' : 'pointer'
                          }}
                        >
                          ▶
                        </button>
                      <button
                        className="dataset-delete-btn"
                        onClick={(e) => {
                          e.stopPropagation();
                          handleDeleteDataset(dataset.id, dataset.name);
                        }}
                        title={`Delete dataset "${dataset.name}"`}
                      >
                        ×
                      </button>
                    </div>
                  </div>
                  );
                })
              ) : (
                <div className="empty-pane">
                  <div className="empty-icon">📊</div>
                  <p>No datasets found</p>
                </div>
              )}
            </div>
            </div>

            {/* Middle Pane - Input Files */}
            <div className="pipeline-pane files-pane">
              <div className="pane-header">
                <div className="pane-header-left">
                  <h3>Input Files - {datasetName}</h3>
                </div>
                <div className="pane-header-right">
                  <button 
                    className="refresh-btn" 
                    onClick={handleRefreshInputFiles}
                    disabled={inputFilesLoading || !selectedDataset}
                    title="Refresh input files"
                  >
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                      <path d="M23 4v6h-6"/>
                      <path d="M1 20v-6h6"/>
                      <path d="m3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
                    </svg>
                  </button>
                  <div className="pane-count">
                    {inputFilesPagination.total > 0 ? inputFilesPagination.total : inputFiles.length}
                  </div>
                </div>
              </div>
              <div className="pane-content">
                {(() => {
                  console.log('Input files render check:', {
                    inputFilesLoading,
                    inputFilesError,
                    inputFilesLength: inputFiles.length,
                    renderCondition: !inputFilesLoading && !inputFilesError && inputFiles.length === 0 ? 'empty' : 
                                     inputFilesLoading ? 'loading' : 
                                     inputFilesError ? 'error' : 
                                     inputFiles.length > 0 ? 'show-files' : 'fallback-empty'
                  });
                  return null;
                })()}
                {!inputFilesLoading && !inputFilesError && inputFiles.length === 0 && selectedDataset ? (
                  <div className="empty-pane">
                    <div className="empty-icon">📁</div>
                    <p>No input files available</p>
                  </div>
                ) : !selectedDataset ? (
                  <div className="empty-pane">
                    <div className="empty-icon">📂</div>
                    <p>Select a dataset to view input files</p>
                  </div>
                ) : inputFilesLoading ? (
                  <div className="astro-loading-container" style={{ padding: '0.5rem 0', gap: '0.5rem' }}>
                    <div className="astro-loader-galaxy" style={{ width: '24px', height: '24px' }}></div>
                    <div className="astro-loading-text" style={{ fontSize: '12px' }}>Loading input files...</div>
                  </div>
                ) : inputFilesError ? (
                  <div className="empty-pane">
                    <div className="empty-icon">❌</div>
                    <p>Error loading input files</p>
                    <p style={{ fontSize: '12px', color: '#FF6B6B' }}>{inputFilesError}</p>
                  </div>
                ) : (
                  <VirtualizedFileList
                    items={inputFiles}
                    isLoading={inputFilesLoading}
                    error={inputFilesError}
                    emptyMessage="No input files available"
                    emptyIcon="📁"
                    selectedDataset={selectedDataset}
                    onDelete={handleDeleteFile}
                    onProcessFile={handleProcessFile}
                    loadingMessage="Loading input files..."
                    onLoadMore={loadMoreInputFiles}
                    hasNextPage={inputFilesPagination.hasMore}
                    isLoadingMore={inputFilesPagination.loading}
                    itemHeight={48}
                    truncateFileName={truncateFileName}
                    getFileStatusColor={getFileStatusColor}
                    isInputFile={true}
                    processorType={processorType}
                  />
                )}
              </div>
            </div>

            {/* Right Pane - Output Files */}
            <div className="pipeline-pane files-pane">
              <div className="pane-header">
                <div className="pane-header-left">
                  <h3>Output Files - {datasetName}</h3>
                </div>
                <div className="pane-header-right">
                  <button 
                    className="refresh-btn" 
                    onClick={handleRefreshOutputFiles}
                    disabled={outputFilesLoading || !selectedDataset}
                    title="Refresh output files"
                  >
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                      <path d="M23 4v6h-6"/>
                      <path d="M1 20v-6h6"/>
                      <path d="m3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
                    </svg>
                  </button>
                  <div className="pane-count">
                    {outputFilesPagination.total > 0 ? outputFilesPagination.total : outputFiles.length}
                  </div>
                </div>
              </div>
              <div className="pane-content">
                {!outputFilesLoading && !outputFilesError && outputFiles.length === 0 && selectedDataset ? (
                  <div className="empty-pane">
                    <div className="empty-icon">📁</div>
                    <p>No output files available</p>
                  </div>
                ) : !selectedDataset ? (
                  <div className="empty-pane">
                    <div className="empty-icon">📂</div>
                    <p>Select a dataset to view output files</p>
                  </div>
                ) : outputFilesLoading ? (
                  <div className="astro-loading-container" style={{ padding: '0.5rem 0', gap: '0.5rem' }}>
                    <div className="astro-loader-galaxy" style={{ width: '24px', height: '24px' }}></div>
                    <div className="astro-loading-text" style={{ fontSize: '12px' }}>Loading output files...</div>
                  </div>
                ) : outputFilesError ? (
                  <div className="empty-pane">
                    <div className="empty-icon">❌</div>
                    <p>Error loading output files</p>
                    <p style={{ fontSize: '12px', color: '#FF6B6B' }}>{outputFilesError}</p>
                  </div>
                ) : (
                  <VirtualizedFileList
                    items={outputFiles}
                    isLoading={outputFilesLoading}
                    error={outputFilesError}
                    emptyMessage="No output files available"
                    emptyIcon="📁"
                    selectedDataset={selectedDataset}
                    onDelete={handleDeleteFile}
                    loadingMessage="Loading output files..."
                    onLoadMore={loadMoreOutputFiles}
                    hasNextPage={outputFilesPagination.hasMore}
                    isLoadingMore={outputFilesPagination.loading}
                    itemHeight={48}
                    truncateFileName={truncateFileName}
                    getFileStatusColor={getFileStatusColor}
                    isInputFile={false}
                    processorType={processorType}
                  />
                )}
              </div>
            </div>

          </div>
        )}
      </div>
      
      {/* Pipeline Progress Monitor */}
      <PipelineProgress 
        datasets={datasets} 
        inputFiles={inputFiles}
        inputFilesTotalCount={inputFilesPagination.total}
        outputFiles={outputFiles}
        outputFilesTotalCount={outputFilesPagination.total}
        processorType={processorType}
        isCollapsed={isPipelineProgressCollapsed}
        onToggleCollapse={togglePipelineProgressCollapsed}
      />
      
    </div>
  );
}

DatasetsList.propTypes = {
  processorType: PropTypes.string.isRequired,
};

export default DatasetsList; 
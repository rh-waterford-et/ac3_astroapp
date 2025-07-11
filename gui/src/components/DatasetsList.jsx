import React, { useState, useEffect, useCallback, useRef } from 'react';
import PropTypes from 'prop-types';
// Re-enabling FileUpload component
import FileUpload from './FileUpload';
import PipelineProgress from './PipelineProgress';
import { getDatasets, getDatasetFiles, getDatasetOutputFiles, deleteDataset, deleteFile } from '../services/api';
import { getProcessorConfig } from '../config/processorConfig';

function DatasetsList({ processorType = 'starlight' }) {
  const [selectedDataset, setSelectedDataset] = useState('');
  const lastRefreshTime = useRef(0);
  const currentProcessorType = useRef(processorType);
  
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

  // Real datasets from S3
  const [datasets, setDatasets] = useState([]);
  const [loadingDatasets, setLoadingDatasets] = useState(false);
  const [datasetError, setDatasetError] = useState(null);

  // Real input files from S3
  const [inputFiles, setInputFiles] = useState([]);
  const [loadingFiles, setLoadingFiles] = useState(false);
  const [filesError, setFilesError] = useState(null);

  // Real output files from S3
  const [outputFiles, setOutputFiles] = useState([]);
  const [loadingOutputFiles, setLoadingOutputFiles] = useState(false);
  const [outputFilesError, setOutputFilesError] = useState(null);

  // Load datasets function with useCallback for stability
  const loadDatasets = useCallback(async (showLoading = true) => {
    const currentProcessor = processorType; // Capture current processor type
    
    if (showLoading) {
      setLoadingDatasets(true);
    }
    setDatasetError(null);
    try {
      const datasetNames = await getDatasets(currentProcessor);
      
      // Check if processor type changed while we were loading
      if (currentProcessor !== processorType) {
        console.log('Processor type changed during datasets loading, discarding results');
        return;
      }
      
      // Sort dataset names alphabetically (case-insensitive)
      const sortedDatasetNames = datasetNames.sort((a, b) => 
        a.toLowerCase().localeCompare(b.toLowerCase())
      );
      
      // Load input and output files for each dataset to determine completion status
      const datasetObjects = await Promise.all(
        sortedDatasetNames.map(async (name) => {
          let status = 'ready';
          let progress = 0;
          let stage = 'Ready for processing';
          
          try {
            // Load input and output files for this dataset
            const [inputFilesData, outputFilesData] = await Promise.all([
              getDatasetFiles(name, currentProcessor),
              getDatasetOutputFiles(name, currentProcessor)
            ]);
            
            const inputCount = inputFilesData.length;
            const outputCount = outputFilesData.length;
            
            // Determine status based on file counts
            if (inputCount > 0 && outputCount >= inputCount) {
              status = 'completed';
              progress = 100;
              stage = 'Completed';
            } else if (inputCount > 0 && outputCount > 0) {
              status = 'processing';
              progress = Math.min((outputCount / inputCount) * 100, 100);
              stage = getProcessorConfig(currentProcessor).statusLabels.processing;
            } else if (inputCount > 0) {
              status = 'queued';
              progress = 0;
              stage = 'Queued for processing';
            } else {
              status = 'ready';
              progress = 0;
              stage = 'Ready for processing';
            }
            
            console.log(`Dataset ${name}: ${inputCount} input files, ${outputCount} output files, status: ${status}`);
            
          } catch (error) {
            console.warn(`Failed to load files for dataset ${name}:`, error);
            // Keep default status on error
          }
          
          return {
            id: name,
            name: name,
            status: status,
            progress: progress,
            stage: stage
          };
        })
      );
      
      // Final check before setting state
      if (currentProcessor !== processorType) {
        console.log('Processor type changed during dataset processing, discarding results');
        return;
      }
      
      setDatasets(datasetObjects);
      
      // Auto-select logic: only when selection is invalid or missing
      console.log('Dataset auto-selection logic: found', datasetObjects.length, 'datasets, current selection:', selectedDataset, 'for processor:', currentProcessor);
      
      if (datasetObjects.length > 0) {
        // Check if current selectedDataset is valid for this processor type
        const currentDatasetExists = datasetObjects.find(d => d.id === selectedDataset);
        console.log('Current dataset exists in new list:', !!currentDatasetExists);
        
        // Only auto-select if there's no valid selection (don't override user choices)
        if (!selectedDataset || !currentDatasetExists) {
          const firstDataset = datasetObjects[0];
          console.log('Auto-selecting first dataset:', firstDataset.id, 'from processor type:', currentProcessor);
          setSelectedDataset(firstDataset.id);
        } else {
          console.log('Keeping valid user selection:', selectedDataset);
        }
      } else {
        // No datasets available, clear selection
        console.log('No datasets available for processor type:', currentProcessor, ', clearing selection');
        setSelectedDataset('');
      }
    } catch (error) {
      console.error('Failed to load datasets:', error);
      
      // Only set error if processor type hasn't changed
      if (currentProcessor === processorType) {
        setDatasetError(error.message || 'Failed to load datasets');
      }
    } finally {
      if (showLoading && currentProcessor === processorType) {
        setLoadingDatasets(false);
      }
    }
  }, [processorType]);

  // Load dataset files function with useCallback for stability
  const loadDatasetFiles = useCallback(async (datasetName, showLoading = true) => {
    const currentProcessor = processorType; // Capture current processor type
    
    if (showLoading) {
      setLoadingFiles(true);
    }
    setFilesError(null);
    try {
      const files = await getDatasetFiles(datasetName, currentProcessor);
      
      // Check if processor type changed while we were loading
      if (currentProcessor !== processorType) {
        console.log('Processor type changed during file loading, discarding results');
        return;
      }
      
      // Transform API response to match current file structure
      const transformedFiles = files.map(file => ({
        name: file.name,
        size: formatFileSize(file.size),
        uploaded: file.timestamp,
        status: 'processed', // Default status for existing files
        key: file.key
      }));
      
      // Sort files alphabetically by name (case-insensitive)
      const sortedFiles = transformedFiles.sort((a, b) => 
        a.name.toLowerCase().localeCompare(b.name.toLowerCase())
      );
      
      setInputFiles(sortedFiles);
    } catch (error) {
      console.error('Failed to load dataset files:', error);
      
      // Only set error if processor type hasn't changed
      if (currentProcessor === processorType) {
        setFilesError(error.message || 'Failed to load dataset files');
        setInputFiles([]);
      }
    } finally {
      if (showLoading && currentProcessor === processorType) {
        setLoadingFiles(false);
      }
    }
  }, [processorType]);

  // Load dataset output files function with useCallback for stability
  const loadDatasetOutputFiles = useCallback(async (datasetName, showLoading = true) => {
    const currentProcessor = processorType; // Capture current processor type
    
    if (showLoading) {
      setLoadingOutputFiles(true);
    }
    setOutputFilesError(null);
    try {
      const files = await getDatasetOutputFiles(datasetName, currentProcessor);
      
      // Check if processor type changed while we were loading
      if (currentProcessor !== processorType) {
        console.log('Processor type changed during output file loading, discarding results');
        return;
      }
      
      // Transform API response to match current file structure
      const transformedFiles = files.map(file => ({
        name: file.name,
        size: formatFileSize(file.size),
        uploaded: file.timestamp,
        status: 'completed', // Default status for output files
        key: file.key
      }));
      
      // Sort files alphabetically by name (case-insensitive)
      const sortedFiles = transformedFiles.sort((a, b) => 
        a.name.toLowerCase().localeCompare(b.name.toLowerCase())
      );
      
      setOutputFiles(sortedFiles);
    } catch (error) {
      console.error('Failed to load dataset output files:', error);
      
      // Only set error if processor type hasn't changed
      if (currentProcessor === processorType) {
        setOutputFilesError(error.message || 'Failed to load dataset output files');
        setOutputFiles([]);
      }
    } finally {
      if (showLoading && currentProcessor === processorType) {
        setLoadingOutputFiles(false);
      }
    }
  }, [processorType]);

  // Auto-refresh function (background, no loading indicators)
  const autoRefresh = useCallback(async () => {
    const now = Date.now();
    
    // Much longer throttle to prevent interference (minimum 10 seconds between refreshes)
    if (now - lastRefreshTime.current < 10000) {
      return;
    }
    
    // Skip if processor type changed (component will handle this separately)
    if (currentProcessorType.current !== processorType) {
      return;
    }
    
    // Skip if any loading is in progress to avoid conflicts
    if (loadingDatasets || loadingFiles || loadingOutputFiles) {
      return;
    }
    
    // Skip if user recently interacted (don't interfere with user activity)
    const recentInteraction = now - lastRefreshTime.current < 30000; // 30 seconds
    if (recentInteraction && selectedDataset) {
      return;
    }
    
    lastRefreshTime.current = now;
    console.log('Auto-refresh triggered (background, non-intrusive) for processorType:', processorType);
    
    try {
      // Very conservative refresh - only check for new datasets, don't reload files
      const currentDatasetNames = await getDatasets(processorType);
      const currentNames = datasets.map(d => d.name).sort();
      const newNames = currentDatasetNames.sort();
      
      // Only refresh if the dataset list actually changed
      if (JSON.stringify(currentNames) !== JSON.stringify(newNames)) {
        console.log('Dataset list changed, refreshing...');
        await loadDatasets(false);
      }
    } catch (error) {
      console.warn('Auto-refresh failed:', error);
    }
  }, [processorType, loadingDatasets, loadingFiles, loadingOutputFiles, datasets, selectedDataset, loadDatasets]);

  // Load datasets on component mount
  useEffect(() => {
    console.log('Component mounted, loading datasets for processor type:', processorType);
    loadDatasets(true);
  }, [loadDatasets]);

  // Fallback auto-selection: ONLY when there's no selection at all (not when user makes manual selection)
  useEffect(() => {
    // Only auto-select if we have datasets, no current selection, not loading, and this is the initial load
    if (datasets.length > 0 && !selectedDataset && !loadingDatasets) {
      // Check if this is truly a fresh state (no previous selection)
      const firstDataset = datasets[0];
      console.log('Initial auto-selection: selecting first dataset:', firstDataset.id, 'for processor:', processorType);
      setSelectedDataset(firstDataset.id);
    }
  }, [datasets.length, loadingDatasets]); // Remove selectedDataset and processorType from dependencies to prevent conflicts

  // Handle processor type changes
  useEffect(() => {
    if (currentProcessorType.current !== processorType) {
      console.log('Processor type changed from', currentProcessorType.current, 'to', processorType);
      currentProcessorType.current = processorType;
      
      // Clear current state immediately and synchronously
      console.log('Clearing all state for processor type change');
      setSelectedDataset('');
      setInputFiles([]);
      setOutputFiles([]);
      setFilesError(null);
      setOutputFilesError(null);
      setDatasets([]);
      setDatasetError(null);
      setLoadingFiles(false);
      setLoadingOutputFiles(false);
      
      // Reset throttle timer
      lastRefreshTime.current = 0;
      
      // Reload datasets for the new processor type immediately
      console.log('Loading datasets for new processor type:', processorType);
      loadDatasets(true);
    }
  }, [processorType, loadDatasets]);

  // Load files when selected dataset changes (only if datasets have been loaded)
  useEffect(() => {
    console.log('File loading effect triggered - selectedDataset:', selectedDataset, 'loadingDatasets:', loadingDatasets, 'datasets.length:', datasets.length, 'currentProcessor:', currentProcessorType.current, 'processorType:', processorType);
    
    // Primary guard: don't load files if no datasets are loaded for current processor
    if (datasets.length === 0) {
      console.log('No datasets loaded, clearing files');
      setInputFiles([]);
      setOutputFiles([]);
      setFilesError(null);
      setOutputFilesError(null);
      return;
    }
    
    // Additional check: ensure processor type hasn't changed since datasets were loaded
    if (currentProcessorType.current !== processorType) {
      console.log('Processor type mismatch during file loading, skipping');
      return;
    }
    
    // Extra safety: don't load files if datasets are still loading
    if (loadingDatasets) {
      console.log('Datasets still loading, clearing files and waiting');
      setInputFiles([]);
      setOutputFiles([]);
      setFilesError(null);
      setOutputFilesError(null);
      return;
    }
    
    if (selectedDataset) {
      // Verify the selected dataset actually exists in the current dataset list
      const datasetExists = datasets.find(d => d.id === selectedDataset);
      if (datasetExists) {
        console.log('Loading files for selected dataset:', selectedDataset, 'processor:', processorType);
        loadDatasetFiles(selectedDataset, true);
        loadDatasetOutputFiles(selectedDataset, true);
      } else {
        console.log('Selected dataset', selectedDataset, 'not found in current dataset list, clearing files');
        setInputFiles([]);
        setOutputFiles([]);
        setFilesError(null);
        setOutputFilesError(null);
      }
    } else {
      console.log('Clearing files - no dataset selected');
      setInputFiles([]);
      setOutputFiles([]);
      setFilesError(null);
      setOutputFilesError(null);
    }
  }, [selectedDataset, loadingDatasets, datasets, loadDatasetFiles, loadDatasetOutputFiles]);

  // Set up auto-refresh interval
  useEffect(() => {
    const interval = setInterval(() => {
      autoRefresh();
    }, 30000); // Refresh every 30 seconds (much less frequent)

    return () => clearInterval(interval);
  }, [autoRefresh]);

  const formatFileSize = (bytes) => {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
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

  // Delete dataset function
  const handleDeleteDataset = async (datasetId, datasetName) => {
    const confirmed = window.confirm(`Are you sure you want to delete the dataset "${datasetName}"?`);
    
    if (!confirmed) {
      return;
    }

    try {
      console.log('Deleting dataset:', datasetId);
      
      const result = await deleteDataset(datasetId, processorType);
      
      if (result.success) {
        console.log('Dataset deleted successfully:', datasetId);
        
        // Handle selection logic before refreshing datasets
        if (selectedDataset === datasetId) {
          // Find remaining datasets (excluding the deleted one)
          const remainingDatasets = datasets.filter(d => d.id !== datasetId);
          
          if (remainingDatasets.length > 0) {
            // Select the first remaining dataset (alphabetically sorted)
            const nextDataset = remainingDatasets[0];
            setSelectedDataset(nextDataset.id);
            // File loading will be handled by the useEffect when selectedDataset changes
          } else {
            // No datasets left, clear everything
            setSelectedDataset('');
            setInputFiles([]);
            setOutputFiles([]);
            setFilesError(null);
            setOutputFilesError(null);
          }
        }
        
        // Refresh datasets list to get updated list
        await loadDatasets(true);
        
        // Give user feedback
        console.log(`Dataset "${datasetName}" deleted successfully`);
      } else {
        console.error('Failed to delete dataset:', result.message);
      }
    } catch (error) {
      console.error('Error deleting dataset:', error.message);
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
        if (isInputFile) {
          await loadDatasetFiles(selectedDataset, true);
        } else {
          await loadDatasetOutputFiles(selectedDataset, true);
        }
      }
    } catch (error) {
      console.error('Error deleting file:', error);
      
      // Restore the correct state if deletion failed
      if (isInputFile) {
        await loadDatasetFiles(selectedDataset, true);
      } else {
        await loadDatasetOutputFiles(selectedDataset, true);
      }
    }
  };

  return (
    <div className="pipeline-wrapper">
      {/* File Upload Container */}
      <div style={{ marginBottom: '0.75rem' }}>
        <FileUpload isCollapsed={isUploadCollapsed} onToggleCollapse={toggleUploadCollapsed} processorType={processorType} />
      </div>
      
      {/* Three-pane layout Container */}
      <div className="pipeline-container" style={{ marginBottom: '0.75rem' }}>
        <div className={`pipeline-panes ${isDatasetsCollapsed ? 'datasets-collapsed' : ''}`}>
        
        {/* Left Pane - Dataset Selection */}
        <div className="pipeline-pane datasets-pane">
          <div className="pane-header">
            <div className="pane-header-left">
              <button 
                className="collapse-toggle"
                onClick={toggleDatasetsCollapsed}
                title={isDatasetsCollapsed ? "Expand Datasets" : "Collapse Datasets"}
              >
                <span className={`toggle-icon ${isDatasetsCollapsed ? 'collapsed' : ''}`}>
                  {isDatasetsCollapsed ? '▲' : '▼'}
                </span>
              </button>
              <h3>Datasets</h3>
            </div>
            <div className="pane-count">{datasets.length}</div>
          </div>
          {!isDatasetsCollapsed && (
            <div className="pane-content">
            {loadingDatasets ? (
              <div className="astro-loading-container">
                <div className="astro-loader-galaxy"></div>
                <div className="astro-loading-text">Loading datasets...</div>
              </div>
            ) : datasetError ? (
              <div className="astro-loading-container" style={{ padding: '0.5rem 0', gap: '0.5rem' }}>
                <div className="astro-loader-galaxy" style={{ width: '24px', height: '24px' }}></div>
                <div className="astro-loading-text" style={{ fontSize: '12px' }}>Loading datasets...</div>
              </div>
            ) : datasets.length > 0 ? (
              datasets.map(dataset => (
                <div key={dataset.id} className={`dataset-item-container ${selectedDataset === dataset.id ? 'active' : ''}`}>
                  <button
                    className="dataset-item"
                    onClick={() => setSelectedDataset(dataset.id)}
                  >
                    <div className="dataset-info">
                      <div className="dataset-name">{dataset.name}</div>
                      <div className="dataset-stage">{dataset.stage}</div>
                    </div>
                    <div className="dataset-status">
                      <span 
                        className="status-dot"
                        style={{ backgroundColor: getDatasetStatusColor(dataset.status) }}
                        title={dataset.status}
                      ></span>
                    </div>
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
              ))
            ) : (
              <div className="empty-pane">
                <div className="empty-icon">📊</div>
                <p>No datasets found</p>
              </div>
            )}
          </div>
          )}
        </div>

        {!isDatasetsCollapsed && (
          <>
        {/* Middle Pane - Input Files */}
        <div className="pipeline-pane files-pane">
          <div className="pane-header">
            <h3>Input Files - {datasetName}</h3>
            <div className="pane-count">{inputFiles.length}</div>
          </div>
          <div className="pane-content">
            {!loadingFiles && !filesError && inputFiles.length === 0 ? (
              <div className="empty-pane">
                <div className="empty-icon">📁</div>
                <p>No input files available</p>
              </div>
            ) : loadingFiles ? (
              <div className="astro-loading-container" style={{ padding: '0.5rem 0', gap: '0.5rem' }}>
                <div className="astro-loader-galaxy" style={{ width: '24px', height: '24px' }}></div>
                <div className="astro-loading-text" style={{ fontSize: '12px' }}>Loading files...</div>
              </div>
            ) : filesError ? (
              <div className="astro-loading-container" style={{ padding: '0.5rem 0', gap: '0.5rem' }}>
                <div className="astro-loader-galaxy" style={{ width: '24px', height: '24px' }}></div>
                <div className="astro-loading-text" style={{ fontSize: '12px' }}>Loading files...</div>
              </div>
            ) : inputFiles.length > 0 ? (
              inputFiles.map((file, index) => (
                <div key={index} className="file-item-container">
                  <div className="file-item">
                    <div className="file-info">
                      <div className="file-name" title={file.name} style={{ whiteSpace: 'nowrap', overflow: 'hidden' }}>{truncateFileName(file.name)}</div>
                      <div className="file-details">
                        <div className="file-size">{file.size}</div>
                      </div>
                    </div>
                    <div className="file-status">
                      <span className="status-dot" style={{ backgroundColor: getFileStatusColor(file.status) }}></span>
                    </div>
                  </div>
                  <button 
                    className="file-delete-btn"
                    onClick={(e) => {
                      e.stopPropagation();
                      handleDeleteFile(file.key, file.name, true);
                    }}
                    title={`Delete file "${file.name}"`}
                  >
                    ×
                  </button>
                </div>
              ))
            ) : (
              <div className="empty-pane">
                <div className="empty-icon">📁</div>
                <p>No processed input files</p>
              </div>
            )}
          </div>
        </div>

        {/* Right Pane - Output Files */}
        <div className="pipeline-pane files-pane">
          <div className="pane-header">
            <h3>Output Files - {datasetName}</h3>
            <div className="pane-count">{outputFiles.length}</div>
          </div>
          <div className="pane-content">
            {!loadingOutputFiles && !outputFilesError && outputFiles.length === 0 ? (
              <div className="empty-pane">
                <div className="empty-icon">📁</div>
                <p>No output files available</p>
              </div>
            ) : loadingOutputFiles ? (
              <div className="astro-loading-container" style={{ padding: '0.5rem 0', gap: '0.5rem' }}>
                <div className="astro-loader-galaxy" style={{ width: '24px', height: '24px' }}></div>
                <div className="astro-loading-text" style={{ fontSize: '12px' }}>Loading output files...</div>
              </div>
            ) : outputFilesError ? (
              <div className="astro-loading-container" style={{ padding: '0.5rem 0', gap: '0.5rem' }}>
                <div className="astro-loader-galaxy" style={{ width: '24px', height: '24px' }}></div>
                <div className="astro-loading-text" style={{ fontSize: '12px' }}>Loading output files...</div>
              </div>
            ) : outputFiles.length > 0 ? (
              outputFiles.map((file, index) => (
                <div key={index} className="file-item-container">
                  <div className="file-item">
                    <div className="file-info">
                      <div className="file-name" title={file.name} style={{ whiteSpace: 'nowrap', overflow: 'hidden' }}>{truncateFileName(file.name)}</div>
                    </div>
                    <div className="file-status" style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <span className="file-size">{file.size}</span>
                      <span 
                        className="status-dot"
                        style={{ backgroundColor: getFileStatusColor(file.status) }}
                        title={file.status}
                      ></span>
                    </div>
                  </div>
                  <button
                    className="file-delete-btn"
                    onClick={(e) => {
                      e.stopPropagation();
                      handleDeleteFile(file.key, file.name, false);
                    }}
                    title={`Delete file "${file.name}"`}
                  >
                    ×
                  </button>
                </div>
              ))
            ) : (
              <div className="empty-pane">
                <div className="empty-icon">📁</div>
                <p>No processed output files</p>
              </div>
            )}
          </div>
        </div>
        </>
        )}

        </div>
      </div>
      
      {/* Pipeline Progress Monitor */}
      <PipelineProgress 
        datasets={datasets} 
        inputFiles={inputFiles}
        outputFiles={outputFiles}
        isCollapsed={isPipelineProgressCollapsed}
        onToggleCollapse={togglePipelineProgressCollapsed}
      />
      
    </div>
  );
}

DatasetsList.propTypes = {
  processorType: PropTypes.string,
};

export default DatasetsList; 
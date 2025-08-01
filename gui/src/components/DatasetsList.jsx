import React, { useState, useEffect, useCallback, useRef } from 'react';
import PropTypes from 'prop-types';
// Re-enabling FileUpload component
import FileUpload from './FileUpload';
import PipelineProgress from './PipelineProgress';
import { getDatasets, getDatasetFiles, getDatasetOutputFiles, deleteDataset, deleteFile, startProcessing } from '../services/api';
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

  // Simple refresh tracking
  const refreshTimer = useRef(null);
  const isRefreshing = useRef(false);
  const fileUploadRef = useRef(null);

    // Clear all data when processor type changes
  useEffect(() => {
    console.log('🔄 ProcessorType changed to:', processorType, '- clearing all state');
    console.log('🔍 Current selectedDataset before clearing:', selectedDataset);
    
    // Stop any existing refresh timer
    if (refreshTimer.current) {
      clearInterval(refreshTimer.current);
      refreshTimer.current = null;
    }
    
    // Clear all state immediately
    setDatasets([]);
    setInputFiles([]);
    setOutputFiles([]);
    setSelectedDataset('');
    setError(null);
    setInputFilesLoading(false);
    setOutputFilesLoading(false);
    setInputFilesLoaded(false);
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

  // Load input files for current dataset
  const loadInputFiles = useCallback(async (silent = false) => {
    if (!selectedDataset || isRefreshing.current) return;
    
    console.log(silent ? '🔄 Background refresh input files for' : '🔄 Loading input files for', selectedDataset);
    isRefreshing.current = true;
    
    // Only show loading spinner for user actions, not background refreshes
    if (!silent) {
      setInputFilesLoading(true);
    }
    
    try {
      const inputFilesData = await getDatasetFiles(selectedDataset, processorType);

      // Process input files
      const inputFiles = inputFilesData
        .filter(file => isValidFile(file.name))
        .map(file => ({
          name: file.name,
          size: formatFileSize(file.size),
          uploaded: file.timestamp,
          status: 'processed',
          key: file.key
        }))
        .sort((a, b) => a.name.toLowerCase().localeCompare(b.name.toLowerCase()));

      setInputFiles(inputFiles);
      setInputFilesLoaded(true);
    } catch (err) {
      console.error('❌ Failed to load input files:', err);
      // Only show error for user actions, not background refreshes
      if (!silent) {
        setError(err.message || 'Failed to load input files');
      }
    } finally {
      // Only hide loading spinner if we showed it
      if (!silent) {
        setInputFilesLoading(false);
      }
      isRefreshing.current = false;
    }
  }, [selectedDataset, processorType]);

  // Load output files for current dataset
  const loadOutputFiles = useCallback(async (silent = false) => {
    if (!selectedDataset || isRefreshing.current) return;
    
    console.log(silent ? '🔄 Background refresh output files for' : '🔄 Loading output files for', selectedDataset);
    isRefreshing.current = true;
    
    // Only show loading spinner for user actions, not background refreshes
    if (!silent) {
      setOutputFilesLoading(true);
    }
    
    try {
      const outputFilesData = await getDatasetOutputFiles(selectedDataset, processorType);

      // Process output files
      const outputFiles = outputFilesData
        .filter(file => isValidFile(file.name))
        .map(file => ({
          name: file.name,
          size: formatFileSize(file.size),
          uploaded: file.timestamp,
          status: 'completed',
          key: file.key
        }))
        .sort((a, b) => a.name.toLowerCase().localeCompare(b.name.toLowerCase()));

      setOutputFiles(outputFiles);
    } catch (err) {
      console.error('❌ Failed to load output files:', err);
      // Only show error for user actions, not background refreshes
      if (!silent) {
        setError(err.message || 'Failed to load output files');
      }
    } finally {
      // Only hide loading spinner if we showed it
      if (!silent) {
        setOutputFilesLoading(false);
      }
      isRefreshing.current = false;
    }
  }, [selectedDataset, processorType]);

  // Start refresh timer
  useEffect(() => {
    // Clear any existing timer
    if (refreshTimer.current) {
      clearInterval(refreshTimer.current);
    }

    // Start new timer - refresh all three panes (datasets, input files, then output files)
    refreshTimer.current = setInterval(async () => {
      if (!loading && !isRefreshing.current && !inputFilesLoading && !outputFilesLoading) {
        console.log('🔄 3s refresh cycle - refreshing all panes');
        
        // First refresh datasets (silent, no force auto-select)
        await loadDatasets(true, false);
        
        // Then refresh files if we have a selected dataset
        if (selectedDataset) {
          await loadInputFiles(true); // Silent background refresh
          
          // After input files load, refresh output files
          setTimeout(() => {
            if (!isRefreshing.current) {
              loadOutputFiles(true);
            }
          }, 100);
        }
      }
    }, 3000);

    // Cleanup on unmount
    return () => {
      if (refreshTimer.current) {
        clearInterval(refreshTimer.current);
      }
    };
  }, [loadInputFiles, loadOutputFiles, loading, selectedDataset, inputFilesLoading, outputFilesLoading]);

  // Load input files when dataset selection changes
  useEffect(() => {
    if (selectedDataset) {
      setInputFilesLoaded(false);
      setOutputFiles([]); // Clear output files when changing datasets
      loadInputFiles();
    } else {
      setInputFiles([]);
      setOutputFiles([]);
      setInputFilesLoaded(false);
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
        
        // Handle selection logic and refresh all panes
        if (selectedDataset === datasetId) {
          // Clear current files immediately
          setInputFiles([]);
          setOutputFiles([]);
          setInputFilesLoaded(false);
          
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
        console.log(`Dataset "${datasetName}" deleted successfully - all panes refreshed`);
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
                <div className="pane-count">{datasets.length}</div>
              </div>
              <div className="pane-content">
              {loading ? (
                <div className="astro-loading-compact" style={{ minHeight: '150px' }}>
                  <div className="astro-loader-galaxy" style={{ width: '20px', height: '20px' }}></div>
                  <div className="astro-loading-text" style={{ fontSize: '12px' }}>Loading datasets...</div>
                </div>
              ) : error ? (
                <div className="empty-pane">
                  <div className="empty-icon">❌</div>
                  <p>Error loading datasets</p>
                  <p style={{ fontSize: '12px', color: '#FF6B6B' }}>{error}</p>
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
                        <div className="dataset-status">
                          <span 
                            className="status-dot"
                            style={{ 
                              backgroundColor: processingComplete ? '#4FD1C5' : getDatasetStatusColor(dataset.status)
                            }}
                            title={processingComplete ? 'Processing complete' : dataset.status}
                          ></span>
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
                <h3>Input Files - {datasetName}</h3>
                <div className="pane-count">{inputFiles.length}</div>
              </div>
              <div className="pane-content">
                {(() => {
                  console.log('Input files render check:', {
                    inputFilesLoading,
                    error,
                    inputFilesLength: inputFiles.length,
                    renderCondition: !inputFilesLoading && !error && inputFiles.length === 0 ? 'empty' : 
                                     inputFilesLoading ? 'loading' : 
                                     error ? 'error' : 
                                     inputFiles.length > 0 ? 'show-files' : 'fallback-empty'
                  });
                  return null;
                })()}
                {!inputFilesLoading && !error && inputFiles.length === 0 && selectedDataset ? (
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
                ) : error ? (
                  <div className="empty-pane">
                    <div className="empty-icon">❌</div>
                    <p>Error loading input files</p>
                    <p style={{ fontSize: '12px', color: '#FF6B6B' }}>{error}</p>
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
                {(() => {
                  console.log('Output files render check:', {
                    outputFilesLoading,
                    inputFilesLoaded,
                    error,
                    outputFilesLength: outputFiles.length,
                    renderCondition: !outputFilesLoading && !error && outputFiles.length === 0 ? 'empty' : 
                                     outputFilesLoading ? 'loading' : 
                                     error ? 'error' : 
                                     outputFiles.length > 0 ? 'show-files' : 'fallback-empty'
                  });
                  return null;
                })()}
                {!outputFilesLoading && !error && outputFiles.length === 0 && inputFilesLoaded ? (
                  <div className="empty-pane">
                    <div className="empty-icon">📁</div>
                    <p>No output files available</p>
                  </div>
                ) : !selectedDataset ? (
                  <div className="empty-pane">
                    <div className="empty-icon">📂</div>
                    <p>Select a dataset to view output files</p>
                  </div>
                ) : !inputFilesLoaded ? (
                  <div className="empty-pane">
                    <div className="cyber-hourglass">
                      <div className="hourglass-top"></div>
                      <div className="hourglass-bottom"></div>
                      <div className="sand-particle"></div>
                    </div>
                  </div>
                ) : outputFilesLoading ? (
                  <div className="astro-loading-container" style={{ padding: '0.5rem 0', gap: '0.5rem' }}>
                    <div className="astro-loader-galaxy" style={{ width: '24px', height: '24px' }}></div>
                    <div className="astro-loading-text" style={{ fontSize: '12px' }}>Loading output files...</div>
                  </div>
                ) : error ? (
                  <div className="empty-pane">
                    <div className="empty-icon">❌</div>
                    <p>Error loading output files</p>
                    <p style={{ fontSize: '12px', color: '#FF6B6B' }}>{error}</p>
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

          </div>
        )}
      </div>
      
      {/* Pipeline Progress Monitor */}
      <PipelineProgress 
        datasets={datasets} 
        inputFiles={inputFiles}
        outputFiles={outputFiles}
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
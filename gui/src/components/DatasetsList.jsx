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
  const [error, setError] = useState(null);

  // Simple refresh tracking
  const refreshTimer = useRef(null);
  const isRefreshing = useRef(false);

  // Clear all data when processor type changes
  useEffect(() => {
    console.log('🔄 ProcessorType changed to:', processorType, '- clearing all state');
    
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
    isRefreshing.current = false;
    
         // Start fresh
     loadDatasets();
   }, [processorType]);

  // Load datasets only
  const loadDatasets = useCallback(async (silent = false) => {
    if (isRefreshing.current) return;
    
    console.log(silent ? '🔄 Background refresh datasets for' : '🔄 Loading datasets for', processorType);
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

      // Auto-select first dataset if none selected
      if (!selectedDataset && datasetObjects.length > 0) {
        console.log('🎯 Auto-selecting first dataset:', datasetObjects[0].id);
        setSelectedDataset(datasetObjects[0].id);
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

  // Load files for current dataset
  const loadFiles = useCallback(async (silent = false) => {
    if (!selectedDataset || isRefreshing.current) return;
    
    console.log(silent ? '🔄 Background refresh files for' : '🔄 Loading files for', selectedDataset);
    isRefreshing.current = true;
    
    // Only show loading spinner for user actions, not background refreshes
    if (!silent) {
      setLoading(true);
    }
    
    try {
      const [inputFilesData, outputFilesData] = await Promise.all([
        getDatasetFiles(selectedDataset, processorType),
        getDatasetOutputFiles(selectedDataset, processorType)
      ]);

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

      setInputFiles(inputFiles);
      setOutputFiles(outputFiles);
    } catch (err) {
      console.error('❌ Failed to load files:', err);
      // Only show error for user actions, not background refreshes
      if (!silent) {
        setError(err.message || 'Failed to load files');
      }
    } finally {
      // Only hide loading spinner if we showed it
      if (!silent) {
        setLoading(false);
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

    // Start new timer - only refresh files
    refreshTimer.current = setInterval(() => {
      if (!loading && !isRefreshing.current && selectedDataset) {
        console.log('🔄 5s refresh cycle');
        loadFiles(true); // Silent background refresh
      }
    }, 5000);

    // Cleanup on unmount
    return () => {
      if (refreshTimer.current) {
        clearInterval(refreshTimer.current);
      }
    };
  }, [loadFiles, loading, selectedDataset]);

     // Load files when dataset selection changes
   useEffect(() => {
     if (selectedDataset) {
       loadFiles();
     } else {
       setInputFiles([]);
       setOutputFiles([]);
     }
   }, [selectedDataset, loadFiles]);

   // Handle dataset creation callback
   const handleDatasetCreated = useCallback((datasetName) => {
     console.log('✅ Dataset created:', datasetName, '- refreshing...');
     loadDatasets(true); // Silent reload since upload already showed loading
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
            setError(null);
          }
        }
        
        // Refresh datasets list to get updated list
        await loadDatasets();
        
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
          isCollapsed={isUploadCollapsed} 
          onToggleCollapse={toggleUploadCollapsed} 
          processorType={processorType}
          onDatasetCreated={handleDatasetCreated}
        />
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
            {loading ? (
              <div className="astro-loading-container">
                <div className="astro-loader-galaxy"></div>
                <div className="astro-loading-text">Loading datasets...</div>
              </div>
            ) : error ? (
              <div className="empty-pane">
                <div className="empty-icon">❌</div>
                <p>Error loading datasets</p>
                <p style={{ fontSize: '12px', color: '#FF6B6B' }}>{error}</p>
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
                  <div className="dataset-actions">
                    <button
                      className="dataset-process-btn"
                      onClick={(e) => {
                        e.stopPropagation();
                        handleStartProcessing(dataset.name);
                      }}
                      title={`Start processing "${dataset.name}"`}
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
            {(() => {
              console.log('Input files render check:', {
                loading,
                error,
                inputFilesLength: inputFiles.length,
                renderCondition: !loading && !error && inputFiles.length === 0 ? 'empty' : 
                                 loading ? 'loading' : 
                                 error ? 'error' : 
                                 inputFiles.length > 0 ? 'show-files' : 'fallback-empty'
              });
              return null;
            })()}
            {!loading && !error && inputFiles.length === 0 ? (
              <div className="empty-pane">
                <div className="empty-icon">📁</div>
                <p>No input files available</p>
              </div>
            ) : loading ? (
              <div className="astro-loading-container" style={{ padding: '0.5rem 0', gap: '0.5rem' }}>
                <div className="astro-loader-galaxy" style={{ width: '24px', height: '24px' }}></div>
                <div className="astro-loading-text" style={{ fontSize: '12px' }}>Loading files...</div>
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
                loading,
                error,
                outputFilesLength: outputFiles.length,
                renderCondition: !loading && !error && outputFiles.length === 0 ? 'empty' : 
                                 loading ? 'loading' : 
                                 error ? 'error' : 
                                 outputFiles.length > 0 ? 'show-files' : 'fallback-empty'
              });
              return null;
            })()}
            {!loading && !error && outputFiles.length === 0 ? (
              <div className="empty-pane">
                <div className="empty-icon">📁</div>
                <p>No output files available</p>
              </div>
            ) : loading ? (
              <div className="astro-loading-container" style={{ padding: '0.5rem 0', gap: '0.5rem' }}>
                <div className="astro-loader-galaxy" style={{ width: '24px', height: '24px' }}></div>
                <div className="astro-loading-text" style={{ fontSize: '12px' }}>Loading files...</div>
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
        </>
        )}

        </div>
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
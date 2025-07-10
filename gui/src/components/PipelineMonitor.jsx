import React, { useState, useEffect, useCallback } from 'react';
import PropTypes from 'prop-types';
// Re-enabling FileUpload component
import FileUpload from './ProgressMonitor';
import PipelineProgressMonitor from './PipelineProgressMonitor';
import { getDatasets, getDatasetFiles, getDatasetOutputFiles, deleteDataset, deleteFile } from '../services/api';

function PipelineMonitor({ selectedApp }) {
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
    if (showLoading) {
      setLoadingDatasets(true);
    }
    setDatasetError(null);
    try {
      const datasetNames = await getDatasets();
      
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
              getDatasetFiles(name),
              getDatasetOutputFiles(name)
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
              stage = 'Starlight analysis';
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
      
      setDatasets(datasetObjects);
      
      // If no dataset is selected and we have datasets, select the first one (alphabetically)
      if (!selectedDataset && datasetObjects.length > 0) {
        setSelectedDataset(datasetObjects[0].id);
      }
      // If selectedDataset doesn't exist in the loaded datasets, select the first one (alphabetically)
      else if (selectedDataset && !datasetObjects.find(d => d.id === selectedDataset) && datasetObjects.length > 0) {
        setSelectedDataset(datasetObjects[0].id);
      }
    } catch (error) {
      console.error('Failed to load datasets:', error);
      setDatasetError(error.message || 'Failed to load datasets');
    } finally {
      if (showLoading) {
        setLoadingDatasets(false);
      }
    }
  }, [selectedDataset]);

  // Load dataset files function with useCallback for stability
  const loadDatasetFiles = useCallback(async (datasetName, showLoading = true) => {
    if (showLoading) {
      setLoadingFiles(true);
    }
    setFilesError(null);
    try {
      const files = await getDatasetFiles(datasetName);
      
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
      setFilesError(error.message || 'Failed to load dataset files');
      setInputFiles([]);
    } finally {
      if (showLoading) {
        setLoadingFiles(false);
      }
    }
  }, []);

  // Load dataset output files function with useCallback for stability
  const loadDatasetOutputFiles = useCallback(async (datasetName, showLoading = true) => {
    if (showLoading) {
      setLoadingOutputFiles(true);
    }
    setOutputFilesError(null);
    try {
      const files = await getDatasetOutputFiles(datasetName);
      
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
      setOutputFilesError(error.message || 'Failed to load dataset output files');
      setOutputFiles([]);
    } finally {
      if (showLoading) {
        setLoadingOutputFiles(false);
      }
    }
  }, []);

  // Auto-refresh function (background, no loading indicators)
  const autoRefresh = useCallback(async () => {
    // Skip auto-refresh if user is currently interacting with UI
    if (loadingDatasets || loadingFiles || loadingOutputFiles) {
      console.log('Skipping auto-refresh - operation in progress');
      return;
    }
    
    console.log('Auto-refresh triggered');
    try {
      // Refresh datasets first
      await loadDatasets(false);
      
      // Only refresh files if we have a selected dataset
      if (selectedDataset) {
        await Promise.all([
          loadDatasetFiles(selectedDataset, false),
          loadDatasetOutputFiles(selectedDataset, false)
        ]);
      }
    } catch (error) {
      console.warn('Auto-refresh failed:', error);
      // Don't show alerts for auto-refresh failures to avoid interrupting user
    }
  }, [selectedDataset, loadDatasets, loadDatasetFiles, loadDatasetOutputFiles, loadingDatasets, loadingFiles, loadingOutputFiles]);

  // Load datasets on component mount
  useEffect(() => {
    loadDatasets(true);
  }, [loadDatasets]);

  // Load files when selected dataset changes
  useEffect(() => {
    if (selectedDataset) {
      loadDatasetFiles(selectedDataset, true);
      loadDatasetOutputFiles(selectedDataset, true);
    } else {
      setInputFiles([]);
      setOutputFiles([]);
    }
  }, [selectedDataset, loadDatasetFiles, loadDatasetOutputFiles]);

  // Set up auto-refresh interval
  useEffect(() => {
    const interval = setInterval(() => {
      autoRefresh();
    }, 5000); // Refresh every 5 seconds

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
      
      const result = await deleteDataset(datasetId, selectedApp || 'starlight');
      
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
      
      const result = await deleteFile(fileKey, selectedApp || 'starlight');
      
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
        <FileUpload isCollapsed={isUploadCollapsed} onToggleCollapse={toggleUploadCollapsed} />
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
      <PipelineProgressMonitor 
        datasets={datasets} 
        inputFiles={inputFiles}
        outputFiles={outputFiles}
        isCollapsed={isPipelineProgressCollapsed}
        onToggleCollapse={togglePipelineProgressCollapsed}
      />
      
    </div>
  );
}

PipelineMonitor.propTypes = {
  selectedApp: PropTypes.string.isRequired,
};

export default PipelineMonitor; 
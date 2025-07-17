import React, { useState, useEffect, useCallback, useRef } from 'react';
import PropTypes from 'prop-types';
// Re-enabling FileUpload component
import FileUpload from './FileUpload';
import PipelineProgress from './PipelineProgress';
import { getDatasets, getDatasetFiles, getDatasetOutputFiles, deleteDataset, deleteFile } from '../services/api';
import { getProcessorConfig } from '../config/processorConfig';

function DatasetsList({ processorType = 'starlight' }) {
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

  // Simple state management - no complex tracking
  const [datasets, setDatasets] = useState([]);
  const [inputFiles, setInputFiles] = useState([]);
  const [outputFiles, setOutputFiles] = useState([]);
  
  // Simple loading states
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  // Track current processor to detect changes
  const currentProcessorType = useRef(processorType);
  
  // Refs to track current state for change detection
  const currentDatasets = useRef([]);
  const currentInputFiles = useRef([]);
  const currentOutputFiles = useRef([]);

  // Function to load datasets with optional silent mode
  const loadDatasets = useCallback(async (forceAutoSelect = false, silent = false) => {
    console.log('Loading datasets for processor:', processorType, 'forceAutoSelect:', forceAutoSelect, 'silent:', silent);
    
    if (!silent) {
      setLoading(true);
      setError(null);
    }
    
    try {
      const datasetNames = await getDatasets(processorType);
      
      // Sort dataset names alphabetically
      const sortedDatasetNames = datasetNames.sort((a, b) => 
        a.toLowerCase().localeCompare(b.toLowerCase())
      );
      
      // Create dataset objects
      const datasetObjects = sortedDatasetNames.map((name) => ({
        id: name,
        name: name,
        status: 'ready',
        progress: 0,
        stage: 'Ready for processing'
      }));
      
      // For silent refresh, check if datasets actually changed
      if (silent) {
        const currentDatasetNames = currentDatasets.current.map(d => d.name).sort();
        const newDatasetNames = datasetObjects.map(d => d.name).sort();
        
        if (JSON.stringify(currentDatasetNames) === JSON.stringify(newDatasetNames)) {
          console.log('Background refresh: datasets unchanged');
          return; // No change, skip update
        }
        console.log('Background refresh: datasets changed, updating');
      }
      
      setDatasets(datasetObjects);
      currentDatasets.current = datasetObjects; // Update ref
      
      // Auto-select first dataset if forced or none selected or current selection is invalid
      if (datasetObjects.length > 0) {
        const currentDatasetExists = datasetObjects.find(d => d.id === selectedDataset);
        if (forceAutoSelect || !selectedDataset || !currentDatasetExists) {
          const firstDataset = datasetObjects[0];
          console.log('Auto-selecting first dataset:', firstDataset.id, 'reason:', forceAutoSelect ? 'forced' : 'no valid selection');
          setSelectedDataset(firstDataset.id);
        }
      } else {
        // No datasets available, clear selection
        setSelectedDataset('');
      }
      
    } catch (err) {
      console.error('Failed to load datasets:', err);
      if (!silent) {
        setError(err.message || 'Failed to load datasets');
        setDatasets([]);
        setSelectedDataset('');
        currentDatasets.current = [];
      }
      // For silent refresh, don't update error state or clear data
    } finally {
      if (!silent) {
        setLoading(false);
      }
    }
  }, [processorType]);

  // Function to load input files with optional silent mode
  const loadInputFiles = useCallback(async (datasetName, silent = false) => {
    if (!datasetName) {
      setInputFiles([]);
      return;
    }
    
    console.log('Loading input files for dataset:', datasetName, 'silent:', silent);
    
    if (!silent) {
      setLoading(true);
    }
    
    try {
      const files = await getDatasetFiles(datasetName, processorType);
      
      // Transform and sort files, filtering out directories and invalid files
      const transformedFiles = files
        .filter(file => isValidFile(file.name))
        .map(file => ({
          name: file.name,
          size: formatFileSize(file.size),
          uploaded: file.timestamp,
          status: 'processed',
          key: file.key
        }));
      
      const sortedFiles = transformedFiles.sort((a, b) => 
        a.name.toLowerCase().localeCompare(b.name.toLowerCase())
      );
      
      setInputFiles(sortedFiles);
      currentInputFiles.current = sortedFiles; // Update ref
      console.log(`Loaded ${sortedFiles.length} input files`);
      
    } catch (err) {
      console.error('Failed to load input files:', err);
      if (!silent) {
        setError(err.message || 'Failed to load input files');
        setInputFiles([]);
        currentInputFiles.current = [];
      }
    } finally {
      if (!silent) {
        setLoading(false);
      }
    }
  }, [processorType]);

  // Function to load output files with optional silent mode
  const loadOutputFiles = useCallback(async (datasetName, silent = false) => {
    if (!datasetName) {
      setOutputFiles([]);
      return;
    }
    
    console.log('Loading output files for dataset:', datasetName, 'silent:', silent);
    
    if (!silent) {
      setLoading(true);
    }
    
    try {
      const files = await getDatasetOutputFiles(datasetName, processorType);
      
      // Transform and sort files, filtering out directories and invalid files
      const transformedFiles = files
        .filter(file => isValidFile(file.name))
        .map(file => ({
          name: file.name,
          size: formatFileSize(file.size),
          uploaded: file.timestamp,
          status: 'completed',
          key: file.key
        }));
      
      const sortedFiles = transformedFiles.sort((a, b) => 
        a.name.toLowerCase().localeCompare(b.name.toLowerCase())
      );
      
      setOutputFiles(sortedFiles);
      currentOutputFiles.current = sortedFiles; // Update ref
      console.log(`Loaded ${sortedFiles.length} output files`);
      
    } catch (err) {
      console.error('Failed to load output files:', err);
      if (!silent) {
        setError(err.message || 'Failed to load output files');
        setOutputFiles([]);
        currentOutputFiles.current = [];
      }
    } finally {
      if (!silent) {
        setLoading(false);
      }
    }
  }, [processorType]);

  // Function to load all files for a dataset with optional silent mode
  const loadAllFiles = useCallback(async (datasetName, silent = false) => {
    if (!datasetName) {
      setInputFiles([]);
      setOutputFiles([]);
      return;
    }
    
    console.log('Loading all files for dataset:', datasetName, 'silent:', silent);
    
    if (!silent) {
      setLoading(true);
      setError(null);
    }
    
    try {
      // Load both input and output files in parallel
      const [inputFilesData, outputFilesData] = await Promise.all([
        getDatasetFiles(datasetName, processorType),
        getDatasetOutputFiles(datasetName, processorType)
      ]);
      
      // Transform input files, filtering out directories and invalid files
      const transformedInputFiles = inputFilesData
        .filter(file => isValidFile(file.name))
        .map(file => ({
          name: file.name,
          size: formatFileSize(file.size),
          uploaded: file.timestamp,
          status: 'processed',
          key: file.key
        }));
      
      // Transform output files, filtering out directories and invalid files
      const transformedOutputFiles = outputFilesData
        .filter(file => isValidFile(file.name))
        .map(file => ({
          name: file.name,
          size: formatFileSize(file.size),
          uploaded: file.timestamp,
          status: 'completed',
          key: file.key
        }));
      
      // Sort files
      const sortedInputFiles = transformedInputFiles.sort((a, b) => 
        a.name.toLowerCase().localeCompare(b.name.toLowerCase())
      );
      
      const sortedOutputFiles = transformedOutputFiles.sort((a, b) => 
        a.name.toLowerCase().localeCompare(b.name.toLowerCase())
      );
      
      // For silent refresh, check if files actually changed
      if (silent) {
        const currentInputNames = currentInputFiles.current.map(f => f.name).sort();
        const newInputNames = sortedInputFiles.map(f => f.name).sort();
        const currentOutputNames = currentOutputFiles.current.map(f => f.name).sort();
        const newOutputNames = sortedOutputFiles.map(f => f.name).sort();
        
        const inputChanged = JSON.stringify(currentInputNames) !== JSON.stringify(newInputNames);
        const outputChanged = JSON.stringify(currentOutputNames) !== JSON.stringify(newOutputNames);
        
        if (!inputChanged && !outputChanged) {
          console.log('Background refresh: files unchanged');
          return; // No change, skip update
        }
        console.log('Background refresh: files changed', { inputChanged, outputChanged });
      }
      
      setInputFiles(sortedInputFiles);
      setOutputFiles(sortedOutputFiles);
      currentInputFiles.current = sortedInputFiles; // Update ref
      currentOutputFiles.current = sortedOutputFiles; // Update ref
      
      console.log(`Loaded ${sortedInputFiles.length} input files and ${sortedOutputFiles.length} output files`);
      
    } catch (err) {
      console.error('Failed to load files:', err);
      if (!silent) {
        setError(err.message || 'Failed to load files');
        setInputFiles([]);
        setOutputFiles([]);
        currentInputFiles.current = [];
        currentOutputFiles.current = [];
      }
      // For silent refresh, don't update error state or clear data
    } finally {
      if (!silent) {
        setLoading(false);
      }
    }
  }, [processorType]);

  // Handle processor type changes
  useEffect(() => {
    if (currentProcessorType.current !== processorType) {
      console.log('Processor type changed from', currentProcessorType.current, 'to', processorType);
      currentProcessorType.current = processorType;
      
      // Clear current state
      setSelectedDataset('');
      setInputFiles([]);
      setOutputFiles([]);
      setError(null);
      
      // Clear refs as well
      currentInputFiles.current = [];
      currentOutputFiles.current = [];
      
      // Load datasets for new processor type and force auto-selection
      loadDatasets(true);
    }
  }, [processorType, loadDatasets]);

  // Load datasets on initial mount
  useEffect(() => {
    loadDatasets();
  }, []);

  // Load files when dataset changes (user action - show loading)
  useEffect(() => {
    if (selectedDataset) {
      loadAllFiles(selectedDataset, false); // silent=false for user actions
    } else {
      setInputFiles([]);
      setOutputFiles([]);
      currentInputFiles.current = [];
      currentOutputFiles.current = [];
    }
  }, [selectedDataset, loadAllFiles]);

  // Background auto-refresh every 5 seconds (silent)
  useEffect(() => {
    const interval = setInterval(() => {
      if (!loading) {
        console.log('Background refresh...');
        loadDatasets(false, true); // forceAutoSelect=false, silent=true
        if (selectedDataset) {
          loadAllFiles(selectedDataset, true); // silent=true
        }
      }
    }, 5000); // 5 second interval for dynamic updates

    return () => clearInterval(interval);
  }, [loading, selectedDataset, loadDatasets, loadAllFiles]);

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
        if (isInputFile) {
          await loadInputFiles(selectedDataset);
        } else {
          await loadOutputFiles(selectedDataset);
        }
      }
    } catch (error) {
      console.error('Error deleting file:', error);
      
      // Restore the correct state if deletion failed
      if (isInputFile) {
        await loadInputFiles(selectedDataset);
      } else {
        await loadOutputFiles(selectedDataset);
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
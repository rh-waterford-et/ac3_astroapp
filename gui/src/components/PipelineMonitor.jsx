import React, { useState, useEffect } from 'react';
import PropTypes from 'prop-types';
// Re-enabling FileUpload component
import FileUpload from './ProgressMonitor';
import PipelineProgressMonitor from './PipelineProgressMonitor';
import { getDatasets, getDatasetFiles, getDatasetOutputFiles } from '../services/api';

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

  // Load datasets on component mount
  useEffect(() => {
    loadDatasets();
  }, []);

  // Load files when selected dataset changes
  useEffect(() => {
    if (selectedDataset) {
      loadDatasetFiles(selectedDataset);
      loadDatasetOutputFiles(selectedDataset);
    } else {
      setInputFiles([]);
      setOutputFiles([]);
    }
  }, [selectedDataset]);

  const loadDatasets = async () => {
    setLoadingDatasets(true);
    setDatasetError(null);
    try {
      const datasetNames = await getDatasets();
      
      // Sort dataset names alphabetically (case-insensitive)
      const sortedDatasetNames = datasetNames.sort((a, b) => 
        a.toLowerCase().localeCompare(b.toLowerCase())
      );
      
      // Transform sorted dataset names into dataset objects
      const datasetObjects = sortedDatasetNames.map(name => ({
        id: name,
        name: name,
        status: 'ready', // Default status - can be enhanced later
        progress: 0,
        stage: 'Ready for processing'
      }));
      
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
      setLoadingDatasets(false);
    }
  };

  const loadDatasetFiles = async (datasetName) => {
    setLoadingFiles(true);
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
      setLoadingFiles(false);
    }
  };

  const loadDatasetOutputFiles = async (datasetName) => {
    setLoadingOutputFiles(true);
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
      setLoadingOutputFiles(false);
    }
  };

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
      case 'processed': return '#68D391';
      case 'processing': return '#F6AD55';
      case 'queued': return '#9F7AEA';
      case 'error': return '#FC8181';
      default: return '#A0AEC0';
    }
  };

  const selectedDatasetInfo = datasets.find(dataset => dataset.id === selectedDataset);
  const datasetName = selectedDatasetInfo ? selectedDatasetInfo.name : 'Unknown';

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
              <div className="error-state">
                <div className="error-message">⚠️ {datasetError}</div>
              </div>
            ) : datasets.length > 0 ? (
              datasets.map(dataset => (
                <button
                  key={dataset.id}
                  className={`dataset-item ${selectedDataset === dataset.id ? 'active' : ''}`}
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
        {/* Middle Pane - Processed Input Files */}
        <div className="pipeline-pane files-pane">
          <div className="pane-header">
            <h3>Processed Input Files - {datasetName}</h3>
            <div className="pane-count">{inputFiles.length}</div>
          </div>
          <div className="pane-content">
            {loadingFiles ? (
              <div className="astro-loading-container">
                <div className="astro-loader-galaxy"></div>
                <div className="astro-loading-text">Loading files...</div>
              </div>
            ) : filesError ? (
              <div className="error-state">
                <div className="error-message">⚠️ {filesError}</div>
              </div>
            ) : inputFiles.length > 0 ? (
              inputFiles.map((file, index) => (
                <div key={index} className="file-item">
                  <div className="file-info">
                    <div className="file-name" title={file.name} style={{ whiteSpace: 'nowrap', overflow: 'hidden' }}>{truncateFileName(file.name)}</div>
                    <div className="file-timestamp" style={{ marginLeft: '3rem' }}>{file.uploaded}</div>
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
            {loadingOutputFiles ? (
              <div className="astro-loading-container">
                <div className="astro-loader-galaxy"></div>
                <div className="astro-loading-text">Loading output files...</div>
              </div>
            ) : outputFilesError ? (
              <div className="error-state">
                <div className="error-message">⚠️ {outputFilesError}</div>
              </div>
            ) : outputFiles.length > 0 ? (
              outputFiles.map((file, index) => (
                <div key={index} className="file-item">
                  <div className="file-info">
                    <div className="file-name" title={file.name} style={{ whiteSpace: 'nowrap', overflow: 'hidden' }}>{truncateFileName(file.name)}</div>
                    <div className="file-timestamp" style={{ marginLeft: '2rem' }}>{file.uploaded}</div>
                  </div>
                  <div className="file-status" style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                    <span className="file-size">{file.size}</span>
                    <span 
                      className="status-dot"
                      style={{ backgroundColor: getFileStatusColor(file.status) }}
                      title={file.status}
                    ></span>
                  </div>
                </div>
              ))
            ) : (
              <div className="empty-pane">
                <div className="empty-icon">📄</div>
                <p>No processed files</p>
                <div className="empty-hint">Processed files will appear here after processing</div>
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
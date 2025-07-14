import React, { useState, useRef, useEffect, useCallback } from 'react';
import PropTypes from 'prop-types';
import { uploadFiles as apiUploadFiles, getDatasets, createDataset } from '../services/api';
import { getProcessorConfig } from '../config/processorConfig';

function FileUpload({ isCollapsed = false, onToggleCollapse, processorType = 'starlight' }) {
  const [dragActive, setDragActive] = useState(false);
  const [uploadQueue, setUploadQueue] = useState([]);
  const fileInputRef = useRef(null);
  
  // Dataset management
  const [availableDatasets, setAvailableDatasets] = useState([]);
  const [currentDataset, setCurrentDataset] = useState('');
  const [newDatasetName, setNewDatasetName] = useState('');
  const [isCreatingNewDataset, setIsCreatingNewDataset] = useState(false);
  const [loadingDatasets, setLoadingDatasets] = useState(false);
  const [datasetError, setDatasetError] = useState(null);

  // Load datasets with useCallback for stability
  const loadDatasets = useCallback(async (showLoading = true) => {
    if (showLoading) {
      setLoadingDatasets(true);
    }
    setDatasetError(null);
    try {
      const datasets = await getDatasets(processorType);
      setAvailableDatasets(datasets);
      console.log('Loaded datasets:', datasets);
      
      // If no dataset is selected and we have datasets, select the first one
      if (!currentDataset && datasets.length > 0) {
        setCurrentDataset(datasets[0]);
      }
    } catch (error) {
      console.error('Failed to load datasets:', error);
      setDatasetError(error.message || 'Failed to load datasets');
    } finally {
      if (showLoading) {
        setLoadingDatasets(false);
      }
    }
  }, [currentDataset, processorType]);

  // Auto-refresh function for datasets
  const autoRefreshDatasetsFunc = useCallback(() => {
    console.log('Auto-refreshing datasets in FileUpload');
    loadDatasets(false);
  }, [loadDatasets]);

  // Load datasets on component mount
  useEffect(() => {
    loadDatasets(true);
  }, [loadDatasets]);



  // Set up auto-refresh interval for datasets
  useEffect(() => {
    const interval = setInterval(() => {
      autoRefreshDatasetsFunc();
    }, 5000); // Refresh every 5 seconds for dynamic updates

    return () => clearInterval(interval);
  }, [autoRefreshDatasetsFunc]);

  const handleDrag = (e) => {
    e.preventDefault();
    e.stopPropagation();
    if (e.type === "dragenter" || e.type === "dragover") {
      setDragActive(true);
    } else if (e.type === "dragleave") {
      setDragActive(false);
    }
  };

  const handleDrop = (e) => {
    e.preventDefault();
    e.stopPropagation();
    setDragActive(false);
    
    if (e.dataTransfer.files && e.dataTransfer.files[0]) {
      handleFiles(e.dataTransfer.files);
    }
  };

  const handleChange = (e) => {
    e.preventDefault();
    if (e.target.files && e.target.files[0]) {
      handleFiles(e.target.files);
    }
  };

  const handleFiles = (files) => {
    const newFiles = Array.from(files).map(file => ({
      id: Date.now() + Math.random(),
      file: file,
      name: file.name,
      size: formatFileSize(file.size),
      rawSize: file.size, // Store raw size for total calculation
      status: 'ready',
      progress: 0
    }));
    
    setUploadQueue(prev => [...prev, ...newFiles]);
    
    // Reset file input to allow selecting the same files again
    if (fileInputRef.current) {
      fileInputRef.current.value = '';
    }
  };

  const formatFileSize = (bytes) => {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const calculateTotalSize = () => {
    const totalBytes = uploadQueue.reduce((sum, file) => sum + (file.rawSize || 0), 0);
    return formatFileSize(totalBytes);
  };

  const removeFile = (fileId) => {
    setUploadQueue(prev => prev.filter(f => f.id !== fileId));
  };

  const uploadFiles = async () => {
    // Validate dataset selection
    if (!currentDataset || currentDataset.trim() === '') {
      alert('Please select a dataset before uploading files');
      return;
    }

    // Get files that are ready to upload BEFORE changing their status
    const filesToUpload = uploadQueue
      .filter(queueItem => queueItem.status === 'ready')
      .map(queueItem => queueItem.file);

    if (filesToUpload.length === 0) {
      console.log('No files to upload');
      return;
    }

    // Set all files to uploading status
    setUploadQueue(prev => prev.map(file => ({
      ...file,
      status: 'uploading',
      progress: 0
    })));

    try {
      // Upload files using the API service
      const results = await apiUploadFiles(
        filesToUpload,
        currentDataset,
        // Progress callback for individual files
        (file, progress) => {
          setUploadQueue(prev => prev.map(queueItem => 
            queueItem.file === file 
              ? { ...queueItem, progress: progress }
              : queueItem
          ));
        },
        // Overall progress callback
        (overallProgress) => {
          console.log(`Overall upload progress: ${overallProgress}%`);
        },
        processorType
      );

      // Update file statuses based on results
      results.forEach(result => {
        setUploadQueue(prev => prev.map(queueItem => 
          queueItem.file === result.file 
            ? { 
                ...queueItem, 
                status: result.success ? 'completed' : 'error',
                progress: result.success ? 100 : 0,
                error: result.success ? null : result.error
              }
            : queueItem
        ));
      });

      console.log('Upload completed:', results);
    } catch (error) {
      console.error('Upload failed:', error);
      
      // Set all uploading files to error status
      setUploadQueue(prev => prev.map(file => 
        file.status === 'uploading' 
          ? { ...file, status: 'error', progress: 0, error: error.message }
          : file
      ));
    } finally {
      // Reset file input to allow selecting files again
      if (fileInputRef.current) {
        fileInputRef.current.value = '';
      }
    }
  };

  const clearCompleted = () => {
    setUploadQueue(prev => prev.filter(f => f.status !== 'completed'));
  };

  const clearAll = () => {
    setUploadQueue([]);
  };

  const handleNewDatasetCreate = async () => {
    if (newDatasetName.trim()) {
      const sanitizedName = newDatasetName.trim().replace(/[^a-zA-Z0-9_-]/g, '');
      if (sanitizedName) {
        setLoadingDatasets(true);
        setDatasetError(null);
        
        try {
          // Create the dataset in S3
          const result = await createDataset(sanitizedName, processorType);
          
          if (result.success) {
            setCurrentDataset(sanitizedName);
            setIsCreatingNewDataset(false);
            setNewDatasetName('');
            
            // Reload datasets to get the updated list from S3
            await loadDatasets(true);
            
            console.log('Dataset created successfully:', sanitizedName);
          } else {
            setDatasetError(result.message || 'Failed to create dataset');
            console.error('Failed to create dataset:', result.message);
          }
        } catch (error) {
          setDatasetError(error.message || 'Failed to create dataset');
          console.error('Error creating dataset:', error);
        } finally {
          setLoadingDatasets(false);
        }
      }
    }
  };

  const handleNewDatasetCancel = () => {
    setIsCreatingNewDataset(false);
    setNewDatasetName('');
  };

  const getStatusColor = (status) => {
    switch (status) {
      case 'ready': return '#A0AEC0';
      case 'uploading': return '#4FD1C5';
      case 'completed': return '#68D391';
      case 'error': return '#FC8181';
      default: return '#A0AEC0';
    }
  };

  /**
   * Handle file input trigger from click or keyboard
   */
  const triggerFileInput = () => {
    fileInputRef.current?.click();
  };

  /**
   * Handle keyboard interactions for upload zone
   * @param {KeyboardEvent} e - Keyboard event
   */
  const handleKeyDown = (e) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      triggerFileInput();
    }
  };

  return (
    <div className="file-upload">
      <div className="pane-header">
        <div className="pane-header-left">
          {onToggleCollapse && (
            <button 
              className="collapse-toggle"
              onClick={onToggleCollapse}
              title={isCollapsed ? "Expand File Upload" : "Collapse File Upload"}
            >
              <span className={`toggle-icon ${isCollapsed ? 'collapsed' : ''}`}>
                {isCollapsed ? '▲' : '▼'}
              </span>
            </button>
          )}
          <h3>File Upload</h3>
        </div>
        <div className="upload-actions">
          {!isCollapsed && uploadQueue.length > 0 && (
            <>
              <button 
                className="upload-btn"
                onClick={uploadFiles}
                disabled={uploadQueue.every(f => f.status !== 'ready') || !currentDataset}
              >
                Upload All ({uploadQueue.filter(f => f.status === 'ready').length})
              </button>
              <button 
                className="clear-btn"
                onClick={clearCompleted}
                disabled={!uploadQueue.some(f => f.status === 'completed')}
              >
                Clear Completed
              </button>
              <button 
                className="clear-btn clear-all-btn"
                onClick={clearAll}
              >
                Clear All
              </button>
            </>
          )}
        </div>
      </div>
      
      {!isCollapsed && (
        <div className="upload-content">
        {/* Dataset Selection Section */}
        <div className="upload-section dataset-section">
          <div className="section-header">
            <h4>Select Dataset</h4>
            {loadingDatasets && (
              <div className="astro-loading-container" style={{ padding: '0.5rem 0', gap: '0.5rem' }}>
                <div className="astro-loader-galaxy" style={{ width: '24px', height: '24px' }}></div>
                <div className="astro-loading-text" style={{ fontSize: '12px' }}>Loading datasets...</div>
              </div>
            )}
            {datasetError && (
              <div className="astro-loading-container" style={{ padding: '0.5rem 0', gap: '0.5rem' }}>
                <div className="astro-loader-galaxy" style={{ width: '24px', height: '24px' }}></div>
                <div className="astro-loading-text" style={{ fontSize: '12px' }}>Loading datasets...</div>
              </div>
            )}
          </div>
          
          <div className="dataset-selection">
            <div className="dataset-select-wrapper">
              <select 
                className="dataset-select"
                value={currentDataset}
                onChange={(e) => setCurrentDataset(e.target.value)}
                disabled={loadingDatasets}
              >
                <option value="">-- Select Dataset --</option>
                {availableDatasets.map(dataset => (
                  <option key={dataset} value={dataset}>{dataset}</option>
                ))}
              </select>
              {currentDataset && (
                <div className="dataset-info">
                  <span className="dataset-path">📁 {getProcessorConfig(processorType).paths.input}/{currentDataset}</span>
                </div>
              )}
            </div>
            
            {/* Create Dataset Button */}
            {!isCreatingNewDataset && (
              <div className="create-dataset-section">
                <button 
                  className="create-dataset-toggle-btn"
                  onClick={() => setIsCreatingNewDataset(!isCreatingNewDataset)}
                  disabled={loadingDatasets}
                >
                  Create Dataset
                </button>
              </div>
            )}
            
            {/* Create Dataset Form */}
            {isCreatingNewDataset && (
              <>
                <div className="dataset-form-divider"></div>
                <div className="new-dataset-form">
                  <div className="new-dataset-input-group">
                    <input
                      type="text"
                      className="new-dataset-input"
                      placeholder="Enter dataset name (e.g., NGC7025)"
                      value={newDatasetName}
                      onChange={(e) => setNewDatasetName(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') {
                          handleNewDatasetCreate();
                        } else if (e.key === 'Escape') {
                          handleNewDatasetCancel();
                        }
                      }}
                    />
                  </div>
                  {newDatasetName.trim() && (
                    <div className="dataset-preview">
                      📁 {getProcessorConfig(processorType).paths.input}/{newDatasetName.trim().replace(/[^a-zA-Z0-9_-]/g, '')}
                    </div>
                  )}
                  
                  {/* Action buttons side-by-side */}
                  <div className="dataset-form-actions">
                    <button 
                      className="create-dataset-btn"
                      onClick={handleNewDatasetCreate}
                      disabled={!newDatasetName.trim() || loadingDatasets}
                    >
                      Create
                    </button>
                    <button 
                      className="create-dataset-toggle-btn cancel-variant"
                      onClick={() => setIsCreatingNewDataset(false)}
                      disabled={loadingDatasets}
                    >
                      Cancel
                    </button>
                  </div>
                </div>
              </>
            )}
          </div>
        </div>

        {/* Upload Files Section */}
        <div className="upload-section upload-files-section">
          <div className="section-header">
            <h4>Upload Files</h4>
          </div>
          <div 
            className={`upload-zone ${dragActive ? 'drag-active' : ''}`}
            onDragEnter={handleDrag}
            onDragLeave={handleDrag}
            onDragOver={handleDrag}
            onDrop={handleDrop}
            onClick={triggerFileInput}
            onKeyDown={handleKeyDown}
            tabIndex={0}
            role="button"
            aria-label="Upload files - click or press Enter/Space to browse files, or drag and drop files here"
          >
            <input
              ref={fileInputRef}
              type="file"
              multiple
              onChange={handleChange}
              style={{ display: 'none' }}
              accept=".fits,.txt,.csv,.log,.in"
            />
            <div className="upload-icon">📁</div>
            <div className="upload-text">
              <div className="upload-primary">Drop files here or click to browse</div>
              <div className="upload-secondary">Supports: .fits, .txt, .csv, .log, .in files</div>
            </div>
          </div>
        </div>

        {/* Files List Section */}
        <div className="upload-section files-list-section">
          <div className="section-header">
            <h4>Files List</h4>
            {uploadQueue.length > 0 && (
              <div className="queue-summary">{uploadQueue.length} files • {calculateTotalSize()}</div>
            )}
          </div>
          <div className="upload-queue">
            {uploadQueue.length > 0 ? (
              uploadQueue.map(file => (
                <div key={file.id} className="queue-item" data-upload-status={file.status}>
                  <div className="queue-file-info">
                    <div className="queue-file-name">{file.name}</div>
                    <div className="queue-file-size">{file.size}</div>
                    {file.status === 'error' && file.error && (
                      <div className="queue-error-message">
                        <div className="astro-loading-container" style={{ padding: '0.25rem 0', gap: '0.25rem' }}>
                          <div className="astro-loader-galaxy" style={{ width: '12px', height: '12px' }}></div>
                          <div className="astro-loading-text" style={{ fontSize: '10px' }}>Retrying...</div>
                        </div>
                      </div>
                    )}
                  </div>
                  
                  <div className="queue-status">
                    {file.status === 'uploading' && (
                      <div className="upload-progress">
                        <div 
                          className="upload-progress-fill"
                          style={{ 
                            width: `${file.progress}%`,
                            backgroundColor: getStatusColor(file.status)
                          }}
                        ></div>
                      </div>
                    )}
                    <span 
                      className="queue-status-dot"
                      style={{ backgroundColor: getStatusColor(file.status) }}
                      title={file.status === 'error' && file.error ? file.error : file.status}
                    ></span>
                  </div>
                  
                  {(file.status === 'ready' || file.status === 'error') && (
                    <button 
                      className="remove-file-btn"
                      onClick={() => removeFile(file.id)}
                    >
                      ×
                    </button>
                  )}
                </div>
              ))
            ) : (
              <div className="empty-files-list">
                <div className="empty-message">No files selected</div>
                <div className="empty-hint">Add files using the upload area above</div>
              </div>
            )}
          </div>
        </div>
      </div>
      )}
    </div>
  );
}

FileUpload.propTypes = {
  isCollapsed: PropTypes.bool,
  onToggleCollapse: PropTypes.func,
  processorType: PropTypes.string,
};

export default FileUpload; 
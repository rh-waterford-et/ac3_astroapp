import React, { useState, useRef } from 'react';
import PropTypes from 'prop-types';

function FileUpload({ selectedDataset, datasetName }) {
  const [dragActive, setDragActive] = useState(false);
  const [uploadQueue, setUploadQueue] = useState([]);
  const fileInputRef = useRef(null);

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
      status: 'ready',
      progress: 0
    }));
    
    setUploadQueue(prev => [...prev, ...newFiles]);
  };

  const formatFileSize = (bytes) => {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const removeFile = (fileId) => {
    setUploadQueue(prev => prev.filter(f => f.id !== fileId));
  };

  /**
   * Update file progress to 50%
   * @param {string} fileId - The file ID to update
   */
  const updateFileToHalfProgress = (fileId) => {
    setUploadQueue(prev => prev.map(f => 
      f.id === fileId ? { ...f, status: 'uploading', progress: 50 } : f
    ));
  };

  /**
   * Complete file upload (100% progress)
   * @param {string} fileId - The file ID to complete
   */
  const completeFileUpload = (fileId) => {
    setUploadQueue(prev => prev.map(f => 
      f.id === fileId ? { ...f, status: 'completed', progress: 100 } : f
    ));
  };

  /**
   * Simulate upload progress for a single file
   * @param {Object} file - File object with ID
   * @param {number} delay - Initial delay before starting upload
   */
  const simulateFileUpload = (file, delay) => {
    setTimeout(() => {
      updateFileToHalfProgress(file.id);
      setTimeout(() => completeFileUpload(file.id), 1000);
    }, delay);
  };

  const uploadFiles = () => {
    // Mock upload process - in real app this would upload to S3
    setUploadQueue(prev => prev.map(file => ({
      ...file,
      status: 'uploading',
      progress: 0
    })));

    // Simulate upload progress for each file
    uploadQueue.forEach((file, index) => {
      simulateFileUpload(file, index * 200);
    });
  };

  const clearCompleted = () => {
    setUploadQueue(prev => prev.filter(f => f.status !== 'completed'));
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
    // Trigger file input on Enter or Space key
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      triggerFileInput();
    }
  };

  return (
    <div className="file-upload">
      <div className="upload-header">
        <h3>Upload Files to S3</h3>
        <div className="upload-actions">
          {uploadQueue.length > 0 && (
            <>
              <button 
                className="upload-btn"
                onClick={uploadFiles}
                disabled={uploadQueue.every(f => f.status !== 'ready')}
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
            </>
          )}
        </div>
      </div>
      
      <div className="upload-content">
        {/* Upload Zone */}
        <div className="upload-section">
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

        {/* Upload Queue */}
        {uploadQueue.length > 0 && (
          <div className="upload-section">
            <div className="section-header">
              <h4>Upload Queue</h4>
              <div className="queue-summary">{uploadQueue.length} files</div>
            </div>
            <div className="upload-queue">
              {uploadQueue.map(file => (
                <div key={file.id} className="queue-item">
                  <div className="queue-file-info">
                    <div className="queue-file-name">{file.name}</div>
                    <div className="queue-file-size">{file.size}</div>
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
                      title={file.status}
                    ></span>
                  </div>
                  
                  {file.status === 'ready' && (
                    <button 
                      className="remove-file-btn"
                      onClick={() => removeFile(file.id)}
                    >
                      ×
                    </button>
                  )}
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

FileUpload.propTypes = {
  selectedDataset: PropTypes.string.isRequired,
  datasetName: PropTypes.string.isRequired
};

export default FileUpload; 
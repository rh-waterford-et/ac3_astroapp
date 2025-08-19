import React, { useState, useRef, useCallback } from 'react';
import PropTypes from 'prop-types';

function UploadZone({ onFilesSelected, accept = '.fits,.txt,.csv,.log,.in' }) {
  const [dragActive, setDragActive] = useState(false);
  const fileInputRef = useRef(null);

  const triggerFileInput = useCallback(() => {
    fileInputRef.current?.click();
  }, []);

  const handleKeyDown = useCallback((e) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      triggerFileInput();
    }
  }, [triggerFileInput]);

  const handleDrag = useCallback((e) => {
    e.preventDefault();
    e.stopPropagation();
    if (e.type === 'dragenter' || e.type === 'dragover') {
      setDragActive(true);
    } else if (e.type === 'dragleave') {
      setDragActive(false);
    }
  }, []);

  const handleDrop = useCallback((e) => {
    e.preventDefault();
    e.stopPropagation();
    setDragActive(false);
    if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      onFilesSelected?.(e.dataTransfer.files);
      e.dataTransfer.clearData();
    }
  }, [onFilesSelected]);

  const handleChange = useCallback((e) => {
    e.preventDefault();
    if (e.target.files && e.target.files.length > 0) {
      onFilesSelected?.(e.target.files);
      // Clear value to allow reselecting same files
      e.target.value = '';
    }
  }, [onFilesSelected]);

  return (
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
        accept={accept}
      />
      <div className="upload-icon">📁</div>
      <div className="upload-text">
        <div className="upload-primary">Drop files here or click to browse</div>
        <div className="upload-secondary">Supports: .fits, .txt, .csv, .log, .in files</div>
      </div>
    </div>
  );
}

UploadZone.propTypes = {
  onFilesSelected: PropTypes.func,
  accept: PropTypes.string,
};

export default UploadZone; 
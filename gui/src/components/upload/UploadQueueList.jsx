import React from 'react';
import PropTypes from 'prop-types';

function UploadQueueList({ uploadQueue = [], onRemove, getStatusColor }) {
  return (
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
                onClick={() => onRemove?.(file.id)}
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
  );
}

UploadQueueList.propTypes = {
  uploadQueue: PropTypes.array,
  onRemove: PropTypes.func,
  getStatusColor: PropTypes.func.isRequired,
};

export default UploadQueueList; 
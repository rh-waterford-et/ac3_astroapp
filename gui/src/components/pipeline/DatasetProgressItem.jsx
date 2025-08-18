import React from 'react';
import PropTypes from 'prop-types';
import DatasetProgressBar from './DatasetProgressBar';

const DatasetProgressItem = ({ 
  dataset, 
  progress: currentProgress, 
  processorType, 
  getEstimatedTime
}) => {
  return (
    <div className="pipeline-progress-item">
      <div className="pipeline-progress-item-header">
        <div className="pipeline-progress-item-info">
          <div className="pipeline-progress-item-title">
            <span className="pipeline-progress-dataset-name">
              {dataset.name}
            </span>
            <span className="pipeline-progress-dataset-type">
              {processorType === 'ppxf' ? 'PPXF' : 'STARLIGHT'}
            </span>
          </div>
          <div className="pipeline-progress-stage">
            {currentProgress.stage}
            {currentProgress.status === 'processing' && (
              <span style={{ marginLeft: '0.25rem' }}>⚡</span>
            )}
          </div>
        </div>
        <div className="pipeline-progress-time">
          {getEstimatedTime(
            currentProgress.progress,
            currentProgress.status,
            currentProgress.filesProcessed,
            currentProgress.filesTotal,
            currentProgress.processingHistory
          )}
        </div>
      </div>
      <DatasetProgressBar 
        progress={currentProgress.progress} 
        status={currentProgress.status} 
      />
      {(currentProgress.status === 'processing' || currentProgress.status === 'completed') && 
       processorType !== 'ppxf' && (
        <div className="pipeline-progress-processing-info">
          <span>
            {currentProgress.filesProcessed} of {currentProgress.filesTotal} files processed
          </span>
        </div>
      )}
      
      {(currentProgress.status === 'processing' || currentProgress.status === 'completed') && 
       processorType === 'ppxf' && (
        <div className="pipeline-progress-processing-info">
          <span>
            {Math.floor(currentProgress.filesProcessed / 5)} of {currentProgress.filesTotal} input files processed ({currentProgress.filesProcessed} output files)
          </span>
        </div>
      )}
      
      {currentProgress.status === 'queued' && currentProgress.filesTotal > 0 && (
        <div className="pipeline-progress-processing-info">
          <span>
            {currentProgress.filesTotal} files ready for processing
          </span>
        </div>
      )}
      
      {currentProgress.status === 'error' && currentProgress.errorMessage && (
        <div className="pipeline-progress-processing-info" style={{ color: '#4FD1C5' }}>
          <div className="astro-loading-container" style={{ padding: '0.5rem 0', gap: '0.5rem' }}>
            <div className="astro-loader-galaxy" style={{ width: '16px', height: '16px' }}></div>
            <div className="astro-loading-text" style={{ fontSize: '11px' }}>Processing...</div>
          </div>
        </div>
      )}
    </div>
  );
};

DatasetProgressItem.propTypes = {
  dataset: PropTypes.object.isRequired,
  progress: PropTypes.object.isRequired,
  processorType: PropTypes.string.isRequired,
  getEstimatedTime: PropTypes.func.isRequired
};

export default DatasetProgressItem; 
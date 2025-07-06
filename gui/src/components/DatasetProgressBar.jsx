import React from 'react';
import PropTypes from 'prop-types';

function DatasetProgressBar({ 
  progress = 0, 
  status = 'queued', 
  showPercentage = true, 
  height = 8,
  animated = false 
}) {
  const getProgressText = () => {
    switch (status) {
      case 'completed': return '100%';
      case 'processing': return `${Math.round(progress)}%`;
      case 'queued': return 'Queued';
      case 'ready': return 'Ready';
      case 'error': return 'Error';
      default: return '0%';
    }
  };

  const getFillClasses = () => {
    const baseClasses = ['futuristic-progress-fill'];
    
    // Add status-specific class
    baseClasses.push(status);
    
    // Add processing animation if enabled
    if (animated && status === 'processing') {
      baseClasses.push('processing');
    }
    
    return baseClasses.join(' ');
  };

  const getPercentageClasses = () => {
    return `futuristic-progress-percentage ${status}`;
  };

  return (
    <div className="futuristic-progress-bar">
      <div 
        className="futuristic-progress-track" 
        style={{ height: `${height}px` }}
      >
        <div 
          className={getFillClasses()}
          style={{ 
            width: `${Math.min(Math.max(progress, 0), 100)}%`,
            borderRadius: `${height / 2 - 1}px`
          }}
        ></div>
      </div>
      {showPercentage && (
        <div className={getPercentageClasses()}>
          {getProgressText()}
        </div>
      )}
    </div>
  );
}

DatasetProgressBar.propTypes = {
  progress: PropTypes.number,
  status: PropTypes.oneOf(['completed', 'processing', 'queued', 'ready', 'error']),
  showPercentage: PropTypes.bool,
  height: PropTypes.number,
  animated: PropTypes.bool,
};

export default DatasetProgressBar; 
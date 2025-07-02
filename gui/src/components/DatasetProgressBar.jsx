import React from 'react';

function DatasetProgressBar({ 
  progress = 0, 
  status = 'queued', 
  showPercentage = true, 
  height = 8,
  animated = false 
}) {
  const getProgressColor = (status, progress) => {
    switch (status) {
      case 'completed': return '#68D391';
      case 'processing': return '#4FD1C5';  
      case 'queued': return '#F6AD55';
      case 'ready': return '#9F7AEA';
      case 'error': return '#FC8181';
      default: return '#A0AEC0';
    }
  };

  const getProgressBarStyles = () => ({
    width: '100%',
    height: `${height}px`,
    backgroundColor: '#2D3748',
    borderRadius: '4px',
    overflow: 'hidden',
    position: 'relative',
    marginTop: '4px'
  });

  const getProgressFillStyles = () => ({
    height: '100%',
    width: `${Math.min(Math.max(progress, 0), 100)}%`,
    backgroundColor: getProgressColor(status, progress),
    borderRadius: '4px',
    transition: 'width 0.3s ease, background-color 0.3s ease',
    position: 'relative',
    overflow: 'hidden',
    ...(animated && status === 'processing' && {
      backgroundImage: 'linear-gradient(45deg, rgba(255,255,255,0.1) 25%, transparent 25%, transparent 50%, rgba(255,255,255,0.1) 50%, rgba(255,255,255,0.1) 75%, transparent 75%, transparent)',
      backgroundSize: '20px 20px',
      animation: 'progress-animation 1s linear infinite'
    })
  });

  const getPercentageStyles = () => ({
    fontSize: '10px',
    color: '#A0AEC0',
    marginTop: '2px',
    textAlign: 'right'
  });

  // Add keyframes for animation via a style tag (only if animated)
  React.useEffect(() => {
    if (animated && status === 'processing') {
      const style = document.createElement('style');
      style.textContent = `
        @keyframes progress-animation {
          0% { background-position: 0 0; }
          100% { background-position: 20px 0; }
        }
      `;
      document.head.appendChild(style);
      
      return () => {
        document.head.removeChild(style);
      };
    }
  }, [animated, status]);

  const getProgressText = () => {
    switch (status) {
      case 'completed': return '100%';
      case 'processing': return `${progress}%`;
      case 'queued': return 'Queued';
      case 'ready': return 'Ready';
      case 'error': return 'Error';
      default: return '0%';
    }
  };

  return (
    <div className="dataset-progress-bar">
      <div style={getProgressBarStyles()}>
        <div style={getProgressFillStyles()}></div>
      </div>
      {showPercentage && (
        <div style={getPercentageStyles()}>
          {getProgressText()}
        </div>
      )}
    </div>
  );
}

export default DatasetProgressBar; 
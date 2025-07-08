import React, { useState, useEffect } from 'react';
import PropTypes from 'prop-types';
import DatasetProgressBar from './DatasetProgressBar';

function PipelineProgressMonitor({ datasets, inputFiles, outputFiles, isCollapsed = false, onToggleCollapse }) {
  const [progressData, setProgressData] = useState({});
  const [refreshing, setRefreshing] = useState(false);

  // Calculate progress data from the file counts
  useEffect(() => {
    console.log('PipelineProgressMonitor calculating progress from props');
    console.log('Datasets:', datasets);
    console.log('Input files:', inputFiles.length);
    console.log('Output files:', outputFiles.length);
    
    const progressMap = {};
    
    datasets.forEach(dataset => {
      const processedCount = inputFiles.length;
      const outputCount = outputFiles.length;
      
      let progress = 0;
      let status = 'ready';
      let stage = 'Ready';
      
      if (processedCount > 0) {
        progress = Math.min((outputCount / processedCount) * 100, 100);
        
        console.log(`Dataset ${dataset.name}: ${processedCount} processed files, ${outputCount} output files, ${progress.toFixed(1)}% progress`);
        
        // Determine status and stage based on progress
        if (progress >= 100) {
          status = 'completed';
          stage = 'Completed';
        } else if (progress > 0) {
          status = 'processing';
          stage = 'Starlight analysis';
        } else {
          status = 'queued';
          stage = 'Queued for processing';
        }
      } else if (outputCount > 0) {
        // Edge case: output files exist but no processed files
        progress = 100;
        status = 'completed';
        stage = 'Completed';
      }
      
      progressMap[dataset.id] = {
        progress: progress,
        status: status,
        stage: stage,
        lastUpdated: new Date(),
        filesTotal: processedCount,
        filesProcessed: outputCount,
        errorMessage: ''
      };
    });
    
    console.log('Final progress map:', progressMap);
    setProgressData(progressMap);
    
  }, [datasets, inputFiles, outputFiles]);

  // Periodic refresh indicator
  useEffect(() => {
    const interval = setInterval(() => {
      setRefreshing(true);
      setTimeout(() => setRefreshing(false), 500);
    }, 3000);

    return () => clearInterval(interval);
  }, []);

  const getStatusColor = (status) => {
    switch (status) {
      case 'completed': return '#4FD1C5';
      case 'processing': return '#4FD1C5';
      case 'queued': return '#FF7849';
      case 'ready': return '#A0AEC0';
      case 'error': return '#E53E3E';
      default: return '#A0AEC0';
    }
  };

  const getEstimatedTime = (progress, status, filesProcessed, filesTotal) => {
    if (status === 'completed') return 'Completed';
    if (status === 'queued') return 'Waiting to start';
    if (status === 'ready') return 'Ready to start';
    if (status === 'error') return 'Error occurred';
    
    if (status === 'processing' && filesProcessed > 0 && filesTotal > 0) {
      const remainingFiles = filesTotal - filesProcessed;
      if (remainingFiles === 0) return 'Finalizing...';
      
      // Estimate 2-3 minutes per file for STARLIGHT processing
      const estimatedMinutes = remainingFiles * 2.5;
      if (estimatedMinutes > 60) {
        return `~${Math.ceil(estimatedMinutes / 60)}h remaining`;
      }
      return `~${Math.ceil(estimatedMinutes)}m remaining`;
    }
    
    return 'Calculating...';
  };

  return (
    <div className="pipeline-progress-monitor">
      <div className="pane-header">
        <div className="pane-header-left">
          {onToggleCollapse && (
            <button 
              className="collapse-toggle"
              onClick={onToggleCollapse}
              title={isCollapsed ? "Expand Pipeline Progress" : "Collapse Pipeline Progress"}
            >
              <span className={`toggle-icon ${isCollapsed ? 'collapsed' : ''}`}>
                {isCollapsed ? '▲' : '▼'}
              </span>
            </button>
          )}
          <h3>Pipeline Progress</h3>
        </div>
        <div className="pane-header-right" style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          {datasets.length > 0 && (
            <div className="pane-count">{datasets.length}</div>
          )}
          {refreshing && (
            <span className="pipeline-progress-refresh-indicator">
              🔄
            </span>
          )}
        </div>
      </div>
      
      {!isCollapsed && (
        <>
          <div className="progress-list">
            {datasets.map(dataset => {
              const currentProgress = progressData[dataset.id] || {
                progress: 0,
                status: 'ready',
                stage: 'Ready',
                lastUpdated: new Date(),
                filesTotal: 0,
                filesProcessed: 0,
                errorMessage: ''
              };

              return (
                <div key={dataset.id} className="pipeline-progress-item">
                  <div className="pipeline-progress-item-header">
                    <div className="pipeline-progress-item-info">
                      <div className="pipeline-progress-item-title">
                        <span 
                          className="pipeline-progress-status-dot"
                          style={{ 
                            backgroundColor: getStatusColor(currentProgress.status)
                          }}
                        ></span>
                        <span className="pipeline-progress-dataset-name">
                          {dataset.name}
                        </span>
                        <span className="pipeline-progress-dataset-type">
                          {dataset.type || 'STARLIGHT'}
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
                      {getEstimatedTime(currentProgress.progress, currentProgress.status, currentProgress.filesProcessed, currentProgress.filesTotal)}
                    </div>
                  </div>
                  
                  <DatasetProgressBar 
                    progress={currentProgress.progress}
                    status={currentProgress.status}
                  />
                  
                  {(currentProgress.status === 'processing' || currentProgress.status === 'completed') && (
                    <div className="pipeline-progress-processing-info">
                      <span>
                        {currentProgress.filesProcessed} of {currentProgress.filesTotal} files processed
                      </span>
                      <span>Last updated: {currentProgress.lastUpdated?.toLocaleTimeString()}</span>
                    </div>
                  )}
                  
                  {currentProgress.status === 'queued' && currentProgress.filesTotal > 0 && (
                    <div className="pipeline-progress-processing-info">
                      <span>
                        {currentProgress.filesTotal} files ready for processing
                      </span>
                      <span>Last updated: {currentProgress.lastUpdated?.toLocaleTimeString()}</span>
                    </div>
                  )}
                  
                  {currentProgress.status === 'error' && currentProgress.errorMessage && (
                    <div className="pipeline-progress-processing-info" style={{ color: '#E53E3E' }}>
                      <span>❌ Error: {currentProgress.errorMessage}</span>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
          
          {/* Summary stats at the bottom */}
          <div className="pipeline-progress-summary">
            <div>
              Total datasets: {datasets.length}
            </div>
            <div>
              Processing: {Object.values(progressData).filter(d => d.status === 'processing').length} | 
              Completed: {Object.values(progressData).filter(d => d.status === 'completed').length} | 
              Queued: {Object.values(progressData).filter(d => d.status === 'queued').length}
            </div>
          </div>
          
          {/* Show helpful message when no active processing */}
          {datasets.length > 0 && 
           Object.values(progressData).every(d => d.status === 'ready') && (
            <div className="pipeline-progress-summary" style={{ 
              borderColor: 'rgba(160, 174, 192, 0.3)',
              color: '#A0AEC0',
              fontSize: '0.6rem',
              padding: '0.25rem 0.5rem'
            }}>
              💡 No active pipeline processing. Upload files to start processing.
            </div>
          )}
        </>
      )}
    </div>
  );
}

PipelineProgressMonitor.propTypes = {
  datasets: PropTypes.arrayOf(
    PropTypes.shape({
      id: PropTypes.string.isRequired,
      name: PropTypes.string.isRequired,
      progress: PropTypes.number,
      status: PropTypes.string,
      stage: PropTypes.string,
    })
  ).isRequired,
  inputFiles: PropTypes.array.isRequired,
  outputFiles: PropTypes.array.isRequired,
  isCollapsed: PropTypes.bool,
  onToggleCollapse: PropTypes.func.isRequired,
};

export default PipelineProgressMonitor; 
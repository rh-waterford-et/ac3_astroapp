import React, { useState, useEffect } from 'react';
import PropTypes from 'prop-types';
import DatasetProgressItem from './DatasetProgressItem';
import { useTimeEstimation } from '../../hooks/data/useTimeEstimation';
import { useFileTracking } from '../../hooks/data/useFileTracking';
import { useProgressCalculation } from '../../hooks/data/useProgressCalculation';

function PipelineProgress({ datasets, inputFiles, inputFilesTotalCount, outputFiles, outputFilesTotalCount, processorType, isCollapsed = false, onToggleCollapse }) {
  const [refreshing, setRefreshing] = useState(false);

  // Time estimation hook
  const { getEstimatedTime, getProcessingStats } = useTimeEstimation(processorType);
  
  // File tracking hook (replaces manual state management)
  const { processingHistory, fileProcessingTimes } = useFileTracking(inputFiles, outputFiles, datasets);
  
  // Progress calculation hook (replaces manual progress state and calculation)
  const { progressData } = useProgressCalculation(
    datasets, 
    inputFiles, 
    inputFilesTotalCount, 
    outputFiles, 
    outputFilesTotalCount, 
    processingHistory, 
    processorType
  );

  // Periodic refresh indicator
  useEffect(() => {
    const interval = setInterval(() => {
      setRefreshing(true);
      setTimeout(() => setRefreshing(false), 500);
    }, 5000);

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
                errorMessage: '',
                processingHistory: []
              };

              const stats = getProcessingStats(currentProgress.processingHistory);

              return (
                <DatasetProgressItem
                  key={dataset.id}
                  dataset={dataset}
                  currentProgress={currentProgress}
                  processorType={processorType}
                  getEstimatedTime={getEstimatedTime}
                  getProcessingStats={getProcessingStats}
                  getStatusColor={getStatusColor}
                />
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

PipelineProgress.propTypes = {
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
  inputFilesTotalCount: PropTypes.number,
  outputFiles: PropTypes.array.isRequired,
  outputFilesTotalCount: PropTypes.number,
  processorType: PropTypes.string,
  isCollapsed: PropTypes.bool,
  onToggleCollapse: PropTypes.func.isRequired,
};

export default PipelineProgress; 
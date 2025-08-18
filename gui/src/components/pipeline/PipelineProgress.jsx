import React from 'react';
import PropTypes from 'prop-types';
import DatasetProgressItem from './DatasetProgressItem';
import { useTimeEstimation } from '../../hooks/data/useTimeEstimation';
import { useFileTracking } from '../../hooks/data/useFileTracking';
import { useProgressCalculation } from '../../hooks/data/useProgressCalculation';

function PipelineProgress({ datasets, inputFiles, inputFilesTotalCount, outputFiles, outputFilesTotalCount, processorType, isCollapsed = false, onToggleCollapse }) {
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
          <h3>Pipeline Progress Monitor</h3>
        </div>
        <div className="pane-header-right">
          <div className="pane-count">{datasets.length}</div>
        </div>
      </div>
      
      {!isCollapsed && (
        <div className="pipeline-progress-content">
          {datasets.length > 0 ? (
            <div className="pipeline-progress-list">
              {datasets.map(dataset => {
                const currentProgress = progressData[dataset.name] || {
                  progress: 0,
                  status: 'ready',
                  stage: 'Ready for processing',
                  filesProcessed: 0,
                  filesTotal: 0,
                  processingHistory: []
                };

                return (
                  <DatasetProgressItem
                    key={dataset.id}
                    dataset={dataset}
                    progress={currentProgress}
                    getEstimatedTime={getEstimatedTime}
                    processorType={processorType}
                  />
                );
              })}
            </div>
          ) : (
            <div className="empty-pane">
              <div className="empty-icon">⏳</div>
              <p>No datasets being processed</p>
            </div>
          )}
        </div>
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
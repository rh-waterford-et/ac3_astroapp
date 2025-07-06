import React, { useState, useEffect } from 'react';
import PropTypes from 'prop-types';
import DatasetProgressBar from './DatasetProgressBar';
import { getAllPipelineProgress, getDatasetPipelineProgress } from '../services/api';

function PipelineProgressMonitor({ datasets, isCollapsed = false, onToggleCollapse }) {
  const [progressData, setProgressData] = useState({});
  const [loading, setLoading] = useState(false);
  const [initialLoad, setInitialLoad] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState(null);

  // Fetch progress data from backend
  const fetchProgressData = async (isInitialLoad = false) => {
    if (datasets.length === 0) return;

    // Only show loading spinner on initial load, not on polling refreshes
    if (isInitialLoad) {
      setLoading(true);
    } else {
      setRefreshing(true);
    }
    setError(null);

    try {
      // Fetch all pipeline progress data
      const allProgress = await getAllPipelineProgress();

      // Convert backend progress to UI format
      const progressMap = {};
      
      datasets.forEach(dataset => {
        // Try to find progress for this dataset
        const datasetProgress = Object.values(allProgress).find(
          progress => progress.dataset_name === dataset.name || 
                     progress.dataset_id === dataset.id ||
                     progress.dataset_id.includes(dataset.name)
        );

        if (datasetProgress) {
          progressMap[dataset.id] = {
            progress: datasetProgress.progress || 0,
            status: mapBackendStage(datasetProgress.stage),
            stage: mapBackendStageToDescription(datasetProgress.stage),
            lastUpdated: new Date(datasetProgress.last_updated),
            filesTotal: datasetProgress.files_total || 0,
            filesProcessed: datasetProgress.files_processed || 0,
            errorMessage: datasetProgress.error_message || ''
          };
        } else {
          // Default progress for datasets not in pipeline
          progressMap[dataset.id] = {
            progress: dataset.progress || 0,
            status: dataset.status || 'ready',
            stage: dataset.stage || 'Ready',
            lastUpdated: new Date(),
            filesTotal: 0,
            filesProcessed: 0,
            errorMessage: ''
          };
        }
      });

      setProgressData(progressMap);
      
      // Mark initial load as complete
      if (isInitialLoad) {
        setInitialLoad(false);
      }
    } catch (err) {
      console.error('Error fetching pipeline progress:', err);
      
      // Since API functions now handle 404s gracefully, only show errors for genuine problems
      setError('Unable to connect to pipeline progress service. Please check your connection.');

      // Always provide fallback data even if there's an error
      const fallbackProgress = {};
      datasets.forEach(dataset => {
        fallbackProgress[dataset.id] = {
          progress: dataset.progress || 0,
          status: dataset.status || 'ready',
          stage: dataset.stage || 'Ready',
          lastUpdated: new Date(),
          filesTotal: 0,
          filesProcessed: 0,
          errorMessage: ''
        };
      });
      setProgressData(fallbackProgress);
    } finally {
      if (isInitialLoad) {
        setLoading(false);
      } else {
        setRefreshing(false);
      }
    }
  };

  // Initial fetch and refresh on datasets change
  useEffect(() => {
    setInitialLoad(true);
    fetchProgressData(true);
  }, [datasets]);

  // Periodic refresh of pipeline progress data
  useEffect(() => {
    const interval = setInterval(() => {
      fetchProgressData(false); // Background refresh, no loading spinner
    }, 3000); // Refresh every 3 seconds

    return () => clearInterval(interval);
  }, [datasets]);

  /**
   * Map backend stage to frontend status
   * @param {string} backendStage - Backend stage string
   * @returns {string} - Frontend status string
   */
  const mapBackendStage = (backendStage) => {
    switch (backendStage) {
      case 'processing': return 'processing';
      case 'analysis': return 'processing';
      case 'complete': return 'completed';
      case 'queued': return 'queued';
      case 'error': return 'error';
      case 'ready': return 'ready';
      default: return 'ready';
    }
  };

  /**
   * Map backend stage to frontend stage description
   * @param {string} backendStage - Backend stage string
   * @returns {string} - Frontend stage description
   */
  const mapBackendStageToDescription = (backendStage) => {
    switch (backendStage) {
      case 'ready': return 'Ready';
      case 'queued': return 'Queued for processing';
      case 'processing': return 'Data preprocessing';
      case 'analysis': return 'Starlight analysis';
      case 'complete': return 'Completed';
      case 'error': return 'Error occurred';
      default: return 'Ready';
    }
  };

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

  const getEstimatedTime = (progress, status) => {
    if (status === 'completed') return 'Completed';
    if (status === 'queued') return 'Waiting in queue';
    if (status === 'ready') return 'Ready to start';
    if (status === 'error') return 'Error occurred';
    
    if (progress > 0) {
      const remainingProgress = 100 - progress;
      const estimatedMinutes = Math.ceil((remainingProgress / progress) * 2); // Rough estimation
      if (estimatedMinutes > 60) {
        return `~${Math.ceil(estimatedMinutes / 60)}h remaining`;
      }
      return `~${estimatedMinutes}m remaining`;
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
          {loading && (
            <div className="loading-state">
              <div className="astro-loader-galaxy"></div>
              <span>Loading pipeline progress...</span>
            </div>
          )}
          
          {error && (
            <div className="error-state">
              <span>⚠️ Error loading progress: {error}</span>
              <button className="retry-btn" onClick={() => fetchProgressData(true)}>Retry</button>
            </div>
          )}
          
          <div className="progress-list">
            {datasets.map(dataset => {
              const currentProgress = progressData[dataset.id] || {
                progress: dataset.progress || 0,
                status: dataset.status,
                stage: dataset.stage
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
                      {getEstimatedTime(currentProgress.progress, currentProgress.status)}
                    </div>
                  </div>
                  
                  <DatasetProgressBar 
                    progress={currentProgress.progress}
                    status={currentProgress.status}
                  />
                  
                  {currentProgress.status === 'processing' && (
                    <div className="pipeline-progress-processing-info">
                      <span>Processing...</span>
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
          {!loading && !error && datasets.length > 0 && 
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
  isCollapsed: PropTypes.bool,
  onToggleCollapse: PropTypes.func.isRequired,
};

export default PipelineProgressMonitor; 
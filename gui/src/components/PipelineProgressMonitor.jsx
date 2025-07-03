import React, { useState, useEffect } from 'react';
import PropTypes from 'prop-types';
import DatasetProgressBar from './DatasetProgressBar';

function PipelineProgressMonitor({ datasets }) {
  const [progressData, setProgressData] = useState({});

  // Initialize progress data from datasets
  useEffect(() => {
    const initialProgress = {};
    datasets.forEach(dataset => {
      initialProgress[dataset.id] = {
        progress: dataset.progress || 0,
        status: dataset.status,
        stage: dataset.stage,
        lastUpdated: new Date()
      };
    });
    setProgressData(initialProgress);
  }, [datasets]);

  /**
   * Update stage based on progress percentage
   * @param {number} progress - Current progress percentage
   * @returns {string} - Stage description
   */
  const getStageForProgress = (progress) => {
    if (progress >= 90) return 'Finalizing results';
    if (progress >= 70) return 'Generating output files';
    if (progress >= 40) return 'Stellar population analysis';
    if (progress >= 10) return 'Data preprocessing';
    return 'Starting...';
  };

  /**
   * Update progress for a single dataset
   * @param {Object} dataset - Dataset configuration
   * @param {Object} currentData - Current progress data for the dataset
   * @returns {Object} - Updated progress data or null if no update needed
   */
  const updateDatasetProgress = (dataset, currentData) => {
    if (dataset.status !== 'processing' || !currentData || currentData.progress >= 100) {
      return null;
    }

    // Simulate progress increment (0.5-2% every 2 seconds)
    // Note: Math.random() is safe here - only used for UI simulation, not security
    const increment = Math.random() * 1.5 + 0.5;
    const newProgress = Math.min(currentData.progress + increment, 100);
    
    return {
      ...currentData,
      progress: newProgress,
      stage: getStageForProgress(newProgress),
      lastUpdated: new Date()
    };
  };

  /**
   * Update progress data for all datasets
   * @param {Object} prevData - Previous progress data
   * @returns {Object} - Updated progress data
   */
  const updateAllProgress = (prevData) => {
    const updated = { ...prevData };
    let hasUpdates = false;

    datasets.forEach(dataset => {
      const updatedDataset = updateDatasetProgress(dataset, updated[dataset.id]);
      if (updatedDataset) {
        updated[dataset.id] = updatedDataset;
        hasUpdates = true;
      }
    });

    return hasUpdates ? updated : prevData;
  };

  // Simulate real-time progress updates for processing datasets
  useEffect(() => {
    const interval = setInterval(() => {
      setProgressData(updateAllProgress);
    }, 2000);

    return () => clearInterval(interval);
  }, [datasets]);

  const getStatusColor = (status) => {
    switch (status) {
      case 'completed': return '#68D391';
      case 'processing': return '#4FD1C5';
      case 'queued': return '#F6AD55';
      case 'ready': return '#9F7AEA';
      case 'error': return '#FC8181';
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
      const estimatedMinutes = Math.ceil((remainingProgress / progress) * 15); // Assuming 15 min elapsed
      if (estimatedMinutes > 60) {
        return `~${Math.ceil(estimatedMinutes / 60)}h remaining`;
      }
      return `~${estimatedMinutes}m remaining`;
    }
    
    return 'Starting soon...';
  };

  return (
    <div className="pipeline-progress-monitor" style={{
      backgroundColor: 'rgba(45, 55, 72, 0.3)',
      border: '1px solid rgba(79, 209, 197, 0.2)',
      borderRadius: '6px',
      padding: '12px',
      marginTop: '12px',
      width: '100%',
      maxWidth: '100%',
      boxSizing: 'border-box',
      overflow: 'hidden',
      backdropFilter: 'blur(8px)'
    }}>
      <div className="progress-header" style={{
        marginBottom: '12px',
        borderBottom: '1px solid #2D3748',
        paddingBottom: '8px'
      }}>
        <h3 style={{
          margin: 0,
          color: '#E2E8F0',
          fontSize: '16px',
          fontWeight: '600'
        }}>Pipeline Progress</h3>
      </div>

      <div className="progress-list">
        {datasets.map(dataset => {
          const currentProgress = progressData[dataset.id] || {
            progress: dataset.progress || 0,
            status: dataset.status,
            stage: dataset.stage
          };

          return (
            <div key={dataset.id} className="progress-item" style={{
              backgroundColor: '#2D3748',
              borderRadius: '4px',
              padding: '10px',
              marginBottom: '8px',
              border: '1px solid #4A5568',
              width: '100%',
              maxWidth: '100%',
              boxSizing: 'border-box',
              overflow: 'hidden'
            }}>
              <div style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'flex-start',
                marginBottom: '8px'
              }}>
                <div>
                  <div style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '8px',
                    marginBottom: '4px'
                  }}>
                    <span 
                      style={{ 
                        width: '10px',
                        height: '10px',
                        borderRadius: '50%',
                        backgroundColor: getStatusColor(currentProgress.status),
                        display: 'inline-block',
                        flexShrink: 0
                      }}
                    ></span>
                    <span style={{
                      color: '#E2E8F0',
                      fontSize: '14px',
                      fontWeight: '600'
                    }}>
                      {dataset.name}
                    </span>
                    <span style={{
                      color: '#A0AEC0',
                      fontSize: '12px',
                      backgroundColor: '#4A5568',
                      padding: '2px 6px',
                      borderRadius: '3px'
                    }}>
                      {dataset.type}
                    </span>
                  </div>
                  <div style={{
                    color: '#A0AEC0',
                    fontSize: '12px',
                    marginBottom: '4px'
                  }}>
                    {currentProgress.stage}
                  </div>
                </div>
                <div style={{
                  textAlign: 'right',
                  fontSize: '11px',
                  color: '#A0AEC0'
                }}>
                  {getEstimatedTime(currentProgress.progress, currentProgress.status)}
                </div>
              </div>

              <DatasetProgressBar 
                progress={currentProgress.progress}
                status={currentProgress.status}
                showPercentage={true}
                height={6}
                animated={currentProgress.status === 'processing'}
              />

              {currentProgress.status === 'processing' && (
                <div style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  marginTop: '6px',
                  fontSize: '10px',
                  color: '#A0AEC0'
                }}>
                  <span>Processing...</span>
                  <span>Last updated: {currentProgress.lastUpdated?.toLocaleTimeString()}</span>
                </div>
              )}
            </div>
          );
        })}
      </div>

      {/* Summary stats at the bottom */}
      <div style={{
        marginTop: '12px',
        paddingTop: '8px',
        borderTop: '1px solid #2D3748',
        display: 'flex',
        justifyContent: 'space-between',
        fontSize: '11px',
        color: '#4FD1C5'
      }}>
        <div>
          Total datasets: {datasets.length}
        </div>
        <div>
          Processing: {datasets.filter(d => d.status === 'processing').length} | 
          Completed: {datasets.filter(d => d.status === 'completed').length} | 
          Queued: {datasets.filter(d => d.status === 'queued').length}
        </div>
      </div>
    </div>
  );
}

PipelineProgressMonitor.propTypes = {
  datasets: PropTypes.array.isRequired,
};

export default PipelineProgressMonitor; 
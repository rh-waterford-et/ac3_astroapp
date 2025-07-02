import React, { useState, useEffect } from 'react';
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

  // Simulate real-time progress updates for processing datasets
  useEffect(() => {
    const interval = setInterval(() => {
      setProgressData(prev => {
        const updated = { ...prev };
        let hasUpdates = false;

        datasets.forEach(dataset => {
          if (dataset.status === 'processing' && updated[dataset.id] && updated[dataset.id].progress < 100) {
            // Simulate progress increment (0.5-2% every 2 seconds)
            const increment = Math.random() * 1.5 + 0.5;
            const newProgress = Math.min(updated[dataset.id].progress + increment, 100);
            
            updated[dataset.id] = {
              ...updated[dataset.id],
              progress: newProgress,
              lastUpdated: new Date()
            };
            hasUpdates = true;

            // Update stage based on progress
            if (newProgress >= 90) {
              updated[dataset.id].stage = 'Finalizing results';
            } else if (newProgress >= 70) {
              updated[dataset.id].stage = 'Generating output files';
            } else if (newProgress >= 40) {
              updated[dataset.id].stage = 'Stellar population analysis';
            } else if (newProgress >= 10) {
              updated[dataset.id].stage = 'Data preprocessing';
            }
          }
        });

        return hasUpdates ? updated : prev;
      });
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

export default PipelineProgressMonitor; 
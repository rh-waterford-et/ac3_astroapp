import React, { useState, useEffect, useRef } from 'react';
import PropTypes from 'prop-types';
import { mean, median, standardDeviation } from 'simple-statistics';
import DatasetProgressBar from './DatasetProgressBar';

function PipelineProgress({ datasets, inputFiles, inputFilesTotalCount, outputFiles, outputFilesTotalCount, processorType, isCollapsed = false, onToggleCollapse }) {
  const [progressData, setProgressData] = useState({});
  const [refreshing, setRefreshing] = useState(false);
  
  // Enhanced state for tracking processing times
  const [processingHistory, setProcessingHistory] = useState({});
  const [fileProcessingTimes, setFileProcessingTimes] = useState({});
  const previousOutputFiles = useRef([]);
  const processingStartTimes = useRef({});

  // Track when files start and complete processing
  useEffect(() => {
    const currentOutputCount = outputFiles.length;
    const previousOutputCount = previousOutputFiles.current.length;
    
    console.log(`File tracking: Current output ${currentOutputCount}, Previous output ${previousOutputCount}, Input ${inputFiles.length}`);
    
    // If we have more output files than before, some files were just processed
    if (currentOutputCount > previousOutputCount) {
      const newFiles = outputFiles.slice(previousOutputCount);
      const processingEndTime = Date.now();
      
      console.log(`New output files detected:`, newFiles.map(f => f.name));
      
      newFiles.forEach(file => {
        const fileId = file.key || file.name;
        const startTime = processingStartTimes.current[fileId];
        
        console.log(`Processing file ${fileId}: startTime = ${startTime}, endTime = ${processingEndTime}`);
        
        if (startTime) {
          const processingTime = processingEndTime - startTime;
          const fileSize = file.size || 0;
          
          console.log(`File ${fileId} processed in ${processingTime}ms (${(processingTime/1000/60).toFixed(2)} minutes)`);
          
          // Store processing time with metadata
          setFileProcessingTimes(prev => ({
            ...prev,
            [fileId]: {
              processingTime,
              fileSize,
              timestamp: processingEndTime,
              fileName: file.name
            }
          }));
          
          // Update processing history for the dataset
          const datasetId = datasets.find(d => d.id)?.id || 'default';
          console.log(`Adding to processing history for dataset: ${datasetId}`);
          
          setProcessingHistory(prev => ({
            ...prev,
            [datasetId]: [
              ...(prev[datasetId] || []),
              {
                processingTime,
                fileSize,
                timestamp: processingEndTime,
                fileName: file.name
              }
            ].slice(-20) // Keep only last 20 processing times
          }));
          
          // Clean up start time tracking
          delete processingStartTimes.current[fileId];
        } else {
          console.log(`No start time found for file ${fileId}, cannot calculate processing time`);
        }
      });
    }
    
    // Track when files start processing (when we have input files but fewer output files)
    if (inputFiles.length > currentOutputCount) {
      const processingFiles = inputFiles.slice(currentOutputCount);
      const processingStartTime = Date.now();
      
      console.log(`Files starting to process:`, processingFiles.map(f => f.name));
      
      processingFiles.forEach(file => {
        const fileId = file.key || file.name;
        if (!processingStartTimes.current[fileId]) {
          processingStartTimes.current[fileId] = processingStartTime;
          console.log(`Set start time for file ${fileId}: ${processingStartTime}`);
        }
      });
      
      console.log(`Current processing start times:`, Object.keys(processingStartTimes.current));
    }
    
    // Update reference
    previousOutputFiles.current = outputFiles;
  }, [outputFiles, inputFiles, datasets]);

  // Calculate progress data from props
  useEffect(() => {
    console.log('PipelineProgress calculating progress from props');
    console.log('Datasets:', datasets);
    console.log('Input files loaded:', inputFiles.length);
    console.log('Input files total:', inputFilesTotalCount);
    console.log('Output files loaded:', outputFiles.length);
    console.log('Output files total:', outputFilesTotalCount);
    
    const progressMap = {};
    
    datasets.forEach(dataset => {
      const processedCount = inputFilesTotalCount || inputFiles.length;
      const outputCount = outputFilesTotalCount || outputFiles.length;
      
      // Use the dataset status that's already calculated in the parent component
      let progress = dataset.progress || 0;
      let status = dataset.status || 'ready';
      let stage = dataset.stage || 'Ready';
      
      // Only recalculate if we have input/output files for the selected dataset
      if (datasets.length === 1 && processedCount > 0) {
        // Calculate expected output count based on app type
        let expectedOutputCount;
        if (processorType === 'ppxf') {
          // PPXF: Each input file produces 5 output files
          expectedOutputCount = processedCount * 5;
        } else {
          // Starlight: 1:1 ratio (default)
          expectedOutputCount = processedCount;
        }
        
        progress = Math.min((outputCount / expectedOutputCount) * 100, 100);
        
        console.log(`Dataset ${dataset.name} (${processorType || 'STARLIGHT'}): ${processedCount} input files, ${outputCount} output files, expected: ${expectedOutputCount}, ${progress.toFixed(1)}% progress`);
        
        // Determine status and stage based on progress only if not already set
        if (progress >= 100) {
          status = 'completed';
          stage = 'Completed';
        } else if (progress > 0) {
          status = 'processing';
          stage = processorType === 'ppxf' ? 'PPXF analysis' : 'Starlight analysis';
        } else {
          status = 'queued';
          stage = 'Queued for processing';
        }
      } else if (datasets.length === 1 && outputCount > 0) {
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
        errorMessage: '',
        // Add processing history reference
        processingHistory: processingHistory[dataset.id] || []
      };
    });
    
    console.log('Final progress map:', progressMap);
    setProgressData(progressMap);
    
  }, [datasets, inputFiles, inputFilesTotalCount, outputFiles, outputFilesTotalCount, processingHistory, processorType]);

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

  const getEstimatedTime = (progress, status, filesProcessed, filesTotal, processingHistory = []) => {
    if (status === 'completed') return 'Completed';
    if (status === 'queued') return 'Waiting to start';
    if (status === 'ready') return 'Ready to start';
    if (status === 'error') return 'Error occurred';
    
    if (status === 'processing' && filesProcessed > 0 && filesTotal > 0) {
      // Calculate remaining files correctly based on processor type
      let remainingFiles;
      let actualFilesProcessed;
      
      if (processorType === 'ppxf') {
        // For pPXF: filesTotal = input files, filesProcessed = output files
        // Each input file produces 5 output files, so divide by 5 to get input files processed
        actualFilesProcessed = Math.floor(filesProcessed / 5);
        remainingFiles = filesTotal - actualFilesProcessed;
      } else {
        // For STARLIGHT: 1:1 ratio, so use as-is
        actualFilesProcessed = filesProcessed;
        remainingFiles = filesTotal - filesProcessed;
      }
      
      if (remainingFiles <= 0) return 'Finalizing...';
      
      // Debug logging
      console.log(`Time estimation for ${processorType}: ${filesTotal} input files, ${filesProcessed} output files, ${actualFilesProcessed} input files processed, ${remainingFiles} remaining`);
      console.log(`Processing history available:`, processingHistory.length, 'entries');
      
      // Use actual processing history if available
      if (processingHistory.length > 0) {
        const recentHistory = processingHistory.slice(-10); // Use last 10 files
        const processingTimes = recentHistory.map(h => h.processingTime / 1000 / 60); // Convert to minutes
        
        console.log(`Using processing history:`, processingTimes.map(t => `${t.toFixed(2)}m`));
        
        let estimatedMinutesPerFile;
        
        if (processingTimes.length >= 3) {
          // Use statistical analysis for better estimates
          const avgTime = mean(processingTimes);
          const medianTime = median(processingTimes);
          const stdDev = standardDeviation(processingTimes);
          
          console.log(`Stats - Avg: ${avgTime.toFixed(2)}m, Median: ${medianTime.toFixed(2)}m, StdDev: ${stdDev.toFixed(2)}m`);
          
          // Use median if there's high variance, otherwise use mean
          if (stdDev > avgTime * 0.5) {
            estimatedMinutesPerFile = medianTime;
            console.log(`High variance detected, using median: ${medianTime.toFixed(2)}m`);
          } else {
            estimatedMinutesPerFile = avgTime;
            console.log(`Low variance, using mean: ${avgTime.toFixed(2)}m`);
          }
          
          // Add buffer for uncertainty (10-20% based on standard deviation)
          const uncertaintyBuffer = Math.min(0.2, stdDev / avgTime * 0.5);
          estimatedMinutesPerFile *= (1 + uncertaintyBuffer);
          
        } else {
          // Use simple average for small sample sizes
          estimatedMinutesPerFile = mean(processingTimes);
          console.log(`Small sample, using simple average: ${estimatedMinutesPerFile.toFixed(2)}m`);
        }
        
        const estimatedMinutes = remainingFiles * estimatedMinutesPerFile;
        console.log(`Final estimate: ${remainingFiles} files × ${estimatedMinutesPerFile.toFixed(2)}m = ${estimatedMinutes.toFixed(1)}m`);
        
        if (estimatedMinutes > 60) {
          const hours = Math.ceil(estimatedMinutes / 60);
          return `~${hours}h remaining`;
        }
        return `~${Math.ceil(estimatedMinutes)}m remaining`;
      }
      
      // Improved fallback estimate based on user feedback: 10 minutes total / ~40-50 files = ~15 seconds per file
      console.log(`No processing history, using fallback estimate`);
      const estimatedSeconds = remainingFiles * 15; // 15 seconds per file
      const estimatedMinutes = estimatedSeconds / 60;
      
      if (estimatedMinutes > 60) {
        return `~${Math.ceil(estimatedMinutes / 60)}h remaining`;
      }
      return `~${Math.ceil(estimatedMinutes)}m remaining`;
    }
    
    return 'Calculating...';
  };

  const getProcessingStats = (processingHistory = []) => {
    if (processingHistory.length === 0) return null;
    
    const recentHistory = processingHistory.slice(-10);
    const processingTimes = recentHistory.map(h => h.processingTime / 1000 / 60); // Convert to minutes
    
    if (processingTimes.length < 2) return null;
    
    const avgTime = mean(processingTimes);
    const medianTime = median(processingTimes);
    const stdDev = standardDeviation(processingTimes);
    
    return {
      avgTime: avgTime.toFixed(1),
      medianTime: medianTime.toFixed(1),
      stdDev: stdDev.toFixed(1),
      sampleSize: processingTimes.length
    };
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
                  
                  {(currentProgress.status === 'processing' || currentProgress.status === 'completed') && processorType !== 'ppxf' && (
                    <div className="pipeline-progress-processing-info">
                      <span>
                        {currentProgress.filesProcessed} of {currentProgress.filesTotal} files processed
                      </span>
                    </div>
                  )}
                  
                  {(currentProgress.status === 'processing' || currentProgress.status === 'completed') && processorType === 'ppxf' && (
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
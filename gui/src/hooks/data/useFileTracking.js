import { useState, useEffect, useRef } from 'react';

export const useFileTracking = (inputFiles, outputFiles, datasets) => {
  // State for tracking processing times and history
  const [processingHistory, setProcessingHistory] = useState({});
  const [fileProcessingTimes, setFileProcessingTimes] = useState({});
  const previousOutputFiles = useRef([]);
  const processingStartTimes = useRef({});

  // Track when files start and complete processing
  useEffect(() => {
    const currentOutputCount = outputFiles.length;
    const previousOutputCount = previousOutputFiles.current.length;
    
    // If we have more output files than before, some files were just processed
    if (currentOutputCount > previousOutputCount) {
      const newFiles = outputFiles.slice(previousOutputCount);
      const processingEndTime = Date.now();
      
      newFiles.forEach(file => {
        const fileId = file.key || file.name;
        const startTime = processingStartTimes.current[fileId];
        
        if (startTime) {
          const processingTime = processingEndTime - startTime;
          const fileSize = file.size || 0;
          
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
        }
      });
    }
    
    // Track when files start processing (when we have input files but fewer output files)
    if (inputFiles.length > currentOutputCount) {
      const processingFiles = inputFiles.slice(currentOutputCount);
      const processingStartTime = Date.now();
      
      processingFiles.forEach(file => {
        const fileId = file.key || file.name;
        if (!processingStartTimes.current[fileId]) {
          processingStartTimes.current[fileId] = processingStartTime;
        }
      });
      
    }
    
    // Update reference
    previousOutputFiles.current = outputFiles;
  }, [outputFiles, inputFiles, datasets]);

  return {
    processingHistory,
    fileProcessingTimes
  };
}; 
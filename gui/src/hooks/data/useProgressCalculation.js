import { useState, useEffect } from 'react';

export const useProgressCalculation = (
  datasets, 
  selectedDataset,
  datasetFileCounts,
  processingHistory, 
  processorType
) => {
  const [progressData, setProgressData] = useState({});

  // Calculate progress data from props
  useEffect(() => {
    
    const progressMap = {};
    
    datasets.forEach(dataset => {
      // Determine if this is the selected dataset (for processing history only)
      const isSelectedDataset = dataset.id === selectedDataset;
      
      // Get file counts for this dataset
      // ALWAYS use datasetFileCounts to keep progress bars independent from dataset management pane
      const counts = datasetFileCounts?.[dataset.id] || { input: 0, output: 0 };
      const inputCount = counts.input;
      const outputCount = counts.output;
      
      // Use the dataset status that's already calculated in the parent component
      let progress = dataset.progress || 0;
      let status = dataset.status || 'ready';
      let stage = dataset.stage || 'Ready';
      
      // Calculate progress if we have input files
      if (inputCount > 0) {
        // Calculate expected output count based on app type
        let expectedOutputCount;
        if (processorType === 'ppxf') {
          // PPXF: Each input file produces 5 output files
          expectedOutputCount = inputCount * 5;
        } else {
          // Starlight: 1:1 ratio (default)
          expectedOutputCount = inputCount;
        }
        
        progress = Math.min((outputCount / expectedOutputCount) * 100, 100);
        
        
        // Determine status and stage based on progress
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
      } else if (outputCount > 0) {
        // Edge case: output files exist but no input files
        progress = 100;
        status = 'completed';
        stage = 'Completed';
      }
      
      progressMap[dataset.id] = {
        progress: progress,
        status: status,
        stage: stage,
        lastUpdated: new Date(),
        filesTotal: inputCount,
        filesProcessed: outputCount,
        errorMessage: '',
        // Add processing history reference (only available for selected dataset)
        processingHistory: isSelectedDataset ? (processingHistory[dataset.id] || []) : []
      };
    });
    
    setProgressData(progressMap);
    
  }, [datasets, selectedDataset, datasetFileCounts, processingHistory, processorType]);

  return { progressData };
}; 
import { useState, useEffect } from 'react';

export const useProgressCalculation = (
  datasets, 
  inputFiles, 
  inputFilesTotalCount, 
  outputFiles, 
  outputFilesTotalCount, 
  processingHistory, 
  processorType
) => {
  const [progressData, setProgressData] = useState({});

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

  return { progressData };
}; 
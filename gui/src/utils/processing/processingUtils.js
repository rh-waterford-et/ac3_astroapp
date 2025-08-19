/**
 * Processing-related business logic utilities
 */

/**
 * Calculate expected output files based on processor type
 */
export const getExpectedOutputCount = (inputCount, processorType) => {
  if (processorType === 'starlight') {
    return inputCount; // 1:1 ratio
  } else if (processorType === 'ppxf') {
    return inputCount * 5; // 1:5 ratio
  }
  return inputCount; // Default 1:1
};

/**
 * Check if processing is complete for a dataset
 */
export const isProcessingComplete = (selectedDataset, datasetName, inputFiles, outputFiles, processorType) => {
  // Only check if this is the currently selected dataset
  if (selectedDataset !== datasetName) {
    return false;
  }
  
  const datasetInputFiles = inputFiles.filter(file => file.name);
  const datasetOutputFiles = outputFiles.filter(file => file.name);
  
  const inputCount = datasetInputFiles.length;
  const outputCount = datasetOutputFiles.length;
  const expectedOutputCount = getExpectedOutputCount(inputCount, processorType);
  
  return inputCount > 0 && outputCount >= expectedOutputCount && expectedOutputCount > 0;
}; 
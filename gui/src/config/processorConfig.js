export const PROCESSOR_CONFIGS = {
  starlight: {
    name: 'Starlight',
    displayName: 'Starlight',
    description: 'Stellar population synthesis analysis',
    fileTypes: ['.fits', '.txt', '.csv', '.log', '.in'],
    statusLabels: {
      processing: 'Starlight analysis',
      completed: 'Analysis complete',
      queued: 'Queued for analysis',
      error: 'Analysis failed'
    },
    paths: {
      input: 'starlight/input',
      output: 'starlight/output',
      processed: 'starlight/processed'
    },
    colors: {
      primary: '#4FD1C5',
      secondary: '#38B2AC',
      success: '#68D391',
      warning: '#F6AD55',
      error: '#FC8181'
    }
  },
  ppxf: {
    name: 'PPXF',
    displayName: 'PPXF',
    description: 'Penalized Pixel-Fitting analysis',
    fileTypes: ['.fits', '.dat', '.txt', '.csv', '.log'],
    statusLabels: {
      processing: 'PPXF analysis',
      completed: 'Analysis complete',
      queued: 'Queued for analysis',
      error: 'Analysis failed'
    },
    paths: {
      input: 'ppxf/input',
      output: 'ppxf/output',
      processed: 'ppxf/processed'
    },
    colors: {
      primary: '#9F7AEA',
      secondary: '#805AD5',
      success: '#68D391',
      warning: '#F6AD55',
      error: '#FC8181'
    }
  }
};

export const DEFAULT_PROCESSOR = 'starlight';

export const getProcessorConfig = (processorType) => {
  return PROCESSOR_CONFIGS[processorType] || PROCESSOR_CONFIGS[DEFAULT_PROCESSOR];
};

export const getValidProcessors = () => {
  return Object.keys(PROCESSOR_CONFIGS);
};

export const isValidProcessor = (processorType) => {
  return processorType && PROCESSOR_CONFIGS.hasOwnProperty(processorType);
}; 
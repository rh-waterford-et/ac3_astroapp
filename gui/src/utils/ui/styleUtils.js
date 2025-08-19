/**
 * Styling-related utility functions
 */

/**
 * Get status color for datasets
 */
export const getDatasetStatusColor = (status) => {
  switch (status) {
    case 'completed': return '#68D391';
    case 'processing': return '#4FD1C5';
    case 'queued': return '#F6AD55';
    case 'ready': return '#9F7AEA';
    case 'error': return '#FC8181';
    default: return '#A0AEC0';
  }
};

/**
 * Get status color for files
 */
export const getFileStatusColor = (status) => {
  switch (status) {
    case 'ready': return '#4FD1C5';
    case 'processed': return '#4FD1C5';
    case 'completed': return '#4FD1C5'; // Match pipeline progress completed color
    case 'processing': return '#F6AD55';
    case 'queued': return '#9F7AEA';
    case 'error': return '#FC8181';
    default: return '#A0AEC0';
  }
}; 
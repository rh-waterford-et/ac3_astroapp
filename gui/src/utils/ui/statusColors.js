/**
 * Status color utilities for upload functionality
 */

/**
 * Get color for upload-related statuses
 * @param {string} status - Upload status (ready, uploading, completed, error)
 * @returns {string} - Hex color code
 */
export const getUploadStatusColor = (status) => {
  switch (status) {
    case 'ready': return '#A0AEC0';
    case 'uploading': return '#4FD1C5';
    case 'completed': return '#68D391';
    case 'error': return '#FC8181';
    default: return '#A0AEC0';
  }
}; 
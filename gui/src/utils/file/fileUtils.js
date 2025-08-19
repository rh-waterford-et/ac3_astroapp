/**
 * File-related utility functions
 */

/**
 * Format bytes to human readable file size
 */
export const formatFileSize = (bytes) => {
  if (bytes === 0) return '0 Bytes';
  const k = 1024;
  const sizes = ['Bytes', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

/**
 * Check if an item is a valid file (has extension, not a directory)
 */
export const isValidFile = (fileName) => {
  // Filter out directory markers (ending with /)
  if (fileName.endsWith('/')) {
    return false;
  }
  
  // Filter out items without file extensions
  const lastDotIndex = fileName.lastIndexOf('.');
  if (lastDotIndex === -1 || lastDotIndex === fileName.length - 1) {
    return false;
  }
  
  // Additional check: file extension should be at least 1 character and at most 10 characters
  const extension = fileName.substring(lastDotIndex + 1);
  if (extension.length < 1 || extension.length > 10) {
    return false;
  }
  
  return true;
};

/**
 * Truncate filename while preserving file extension
 */
export const truncateFileName = (fileName, maxLength = 50) => {
  if (fileName.length <= maxLength) return fileName;
  
  const lastDotIndex = fileName.lastIndexOf('.');
  if (lastDotIndex === -1) {
    // No extension, just truncate from the end
    return fileName.substring(0, maxLength - 3) + '...';
  }
  
  const extension = fileName.substring(lastDotIndex);
  const nameWithoutExt = fileName.substring(0, lastDotIndex);
  
  const availableLength = maxLength - extension.length - 3; // 3 for "..."
  
  if (availableLength <= 0) {
    // Extension is too long, just show extension
    return '...' + extension;
  }
  
  return nameWithoutExt.substring(0, availableLength) + '...' + extension;
}; 
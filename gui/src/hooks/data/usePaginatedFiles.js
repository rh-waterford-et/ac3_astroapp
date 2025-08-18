import { useState, useEffect, useCallback, useRef } from 'react';
import { getDatasetFilesUnified, deleteFile as apiDeleteFile } from '../../services/api';

export const usePaginatedFiles = (selectedDataset, fileType, processorType) => {
  const [files, setFiles] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [filesLoaded, setFilesLoaded] = useState(false);
  const [pagination, setPagination] = useState({
    offset: 0,
    hasMore: false,
    loading: false,
    total: 0
  });

  const isRefreshing = useRef(false);
  const abortController = useRef(null);

  // Helper function to check if an item is a valid file (has extension, not a directory)
  const isValidFile = useCallback((fileName) => {
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
  }, []);

  // Helper function to format file size
  const formatFileSize = useCallback((bytes) => {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  }, []);

  // Cancel pending requests
  const cancelRequests = useCallback(() => {
    if (abortController.current) {
      abortController.current.abort();
      abortController.current = null;
    }
  }, [fileType]);

  // Load initial files with pagination
  const loadFiles = useCallback(async (silent = false) => {
    if (!selectedDataset || isRefreshing.current) return;
    
    isRefreshing.current = true;
    
    // Cancel any existing request
    if (abortController.current) {
      abortController.current.abort();
    }
    
    // Create new AbortController for this request
    abortController.current = new AbortController();
    
    if (!silent) {
      setLoading(true);
    }
    
    try {
      const response = await getDatasetFilesUnified(
        selectedDataset, 
        processorType, 
        fileType, 
        0, 
        50, 
        abortController.current.signal
      );

      // Process files
      const processedFiles = response.files
        .filter(file => isValidFile(file.name))
        .map(file => ({
          name: file.name,
          size: formatFileSize(file.size),
          uploaded: file.timestamp,
          status: fileType === 'input' ? 'processed' : 'completed',
          key: file.key
        }));

      setFiles(processedFiles);
      setPagination({
        offset: 50, // Next offset to load
        hasMore: response.pagination.hasMore,
        loading: false,
        total: response.pagination.total
      });
      setFilesLoaded(true);
    } catch (err) {
      if (err.name === 'AbortError') {
        return;
      }
      
      if (!silent) {
        setError(err.message || `Failed to load ${fileType} files`);
      }
      setPagination({ offset: 0, hasMore: false, loading: false, total: 0 });
    } finally {
      if (!silent) {
        setLoading(false);
      }
      isRefreshing.current = false;
    }
  }, [selectedDataset, processorType, fileType, isValidFile, formatFileSize]);

  // Load more files using additive offset-based loading
  const loadMoreFiles = useCallback(async () => {
    if (!selectedDataset || pagination.loading || !pagination.hasMore) return;

    setPagination(prev => ({ ...prev, loading: true }));

    try {
      // Load 100 files per batch for subsequent loads (after initial 50)
      
      const response = await getDatasetFilesUnified(
        selectedDataset, 
        processorType, 
        fileType,
        pagination.offset, 
        100,
        abortController.current?.signal
      );

      // Process new files
      const newFiles = response.files
        .filter(file => isValidFile(file.name))
        .map(file => ({
          name: file.name,
          size: formatFileSize(file.size),
          uploaded: file.timestamp,
          status: fileType === 'input' ? 'processed' : 'completed',
          key: file.key
        }));

      // ADDITIVE: Append to existing files, never overwrite
      setFiles(prev => [...prev, ...newFiles]);
      setPagination({
        offset: pagination.offset + 100,
        hasMore: response.pagination.hasMore,
        loading: false,
        total: response.pagination.total
      });
    } catch (err) {
      setPagination(prev => ({ ...prev, loading: false }));
    }
  }, [selectedDataset, processorType, fileType, pagination.offset, pagination.loading, pagination.hasMore, isValidFile, formatFileSize]);

  // Smart auto-refresh: only update total count, never disrupt loaded files
  const refreshFilesCount = useCallback(async () => {
    if (!selectedDataset || isRefreshing.current || pagination.loading) return;
    
    try {
      // Just check the first page to get updated total count
      const response = await getDatasetFilesUnified(selectedDataset, processorType, fileType, 0, 1);
      
      // Only update the total count if it has changed
      if (response.pagination.total !== pagination.total) {
        setPagination(prev => ({
          ...prev,
          total: response.pagination.total,
          hasMore: prev.offset < response.pagination.total
        }));
      }
    } catch (err) {
    }
  }, [selectedDataset, processorType, fileType, pagination.total, pagination.offset, pagination.loading]);

  // Delete file function
  const deleteFile = useCallback(async (fileKey, fileName) => {
    const confirmed = window.confirm(`Are you sure you want to delete the file "${fileName}"?`);
    
    if (!confirmed) {
      return { success: false, cancelled: true };
    }

    try {
      
      // Immediately remove the file from the UI for instant feedback
      setFiles(prevFiles => prevFiles.filter(file => file.key !== fileKey));
      
      const result = await apiDeleteFile(fileKey, processorType);
      
      if (result.success) {
        return { success: true };
      } else {
        
        // Restore the file in the UI if deletion failed, then refresh
        await loadFiles();
        return { success: false, error: result.message };
      }
    } catch (error) {
      
      // Restore the correct state if deletion failed
      await loadFiles();
      return { success: false, error: error.message };
    }
  }, [loadFiles, processorType]);

  // Manual refresh
  const refresh = useCallback(async () => {
    if (!selectedDataset) return;
    
    // Reset pagination and reload from start, but preserve current total
    const currentTotal = pagination.total;
    setFiles([]);
    setPagination({ offset: 0, hasMore: false, loading: false, total: currentTotal });
    setFilesLoaded(false);
    setError(null); // Clear any previous errors
    await loadFiles();
  }, [selectedDataset, fileType, pagination.total, loadFiles]);

  // Clear files when dataset selection changes
  useEffect(() => {
    if (selectedDataset) {
      setFilesLoaded(false);
      setFiles([]); // Clear files when changing datasets
      setPagination({ offset: 0, hasMore: false, loading: false, total: 0 }); // Reset pagination
      loadFiles();
    } else {
      setFiles([]);
      setFilesLoaded(false);
      setPagination({ offset: 0, hasMore: false, loading: false, total: 0 });
    }
  }, [selectedDataset, loadFiles]);

  // Cleanup: cancel requests when component unmounts
  useEffect(() => {
    return () => {
      cancelRequests();
    };
  }, [fileType, cancelRequests]);

  return {
    // State
    files,
    loading,
    error,
    filesLoaded,
    pagination,
    
    // Actions
    loadFiles,
    loadMoreFiles,
    refreshFilesCount,
    deleteFile,
    refresh,
    
    // Utils
    formatFileSize,
    isValidFile
  };
}; 
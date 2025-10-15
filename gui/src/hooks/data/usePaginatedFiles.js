import { useState, useEffect, useCallback, useRef } from 'react';
import { getDatasetFilesUnified, deleteFile as apiDeleteFile } from '../../services/api';

export const usePaginatedFiles = (selectedDataset, fileType, processorType) => {
  const [files, setFiles] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [filesLoaded, setFilesLoaded] = useState(false);
  const [loadedDataset, setLoadedDataset] = useState(null); // Track which dataset is loaded
  const [pagination, setPagination] = useState({
    offset: 0,
    hasMore: false,
    loading: false,
    total: 0
  });

  const isRefreshing = useRef(false);
  const currentProcessor = useRef(processorType);
  const isProcessorChanging = useRef(false);
  const abortControllerRef = useRef(null);

  // Clear all data when processor type changes
  useEffect(() => {
    // Abort any in-flight requests from previous processor
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
      abortControllerRef.current = null;
    }
    
    // Mark processor as changing to prevent stale loads
    isProcessorChanging.current = true;
    currentProcessor.current = processorType;
    
    setFiles([]);
    setFilesLoaded(false);
    setLoadedDataset(null);
    setPagination({ offset: 0, hasMore: false, loading: false, total: 0 });
    setError(null);
    setLoading(false);
    isRefreshing.current = false;
    
    // Reset processor changing flag after state updates
    setTimeout(() => {
      isProcessorChanging.current = false;
    }, 0);
  }, [processorType, fileType]);

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

  // Load initial files with pagination
  const loadFiles = useCallback(async (silent = false) => {
    if (!selectedDataset) return;
    
    if (isRefreshing.current) {
      return;
    }
    
    // Abort any previous request
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
    }
    
    // Create new abort controller
    abortControllerRef.current = new AbortController();
    const currentAbortController = abortControllerRef.current;
    
    const requestedDataset = selectedDataset; // Capture at request time
    const requestedProcessor = processorType; // Capture at request time
    
    isRefreshing.current = true;
    
    if (!silent) {
      setLoading(true);
    }
    
    try {
      const response = await getDatasetFilesUnified(
        requestedDataset, 
        requestedProcessor, 
        fileType, 
        0, 
        40,
        currentAbortController.signal
      );

      // Check if aborted
      if (currentAbortController.signal.aborted) {
        return;
      }
      
      // Ignore if processor changed (cross-processor contamination) OR dataset changed (within processor)
      if (processorType !== requestedProcessor || selectedDataset !== requestedDataset) {
        return; // Ignore stale files
      }

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
        offset: processedFiles.length,
        hasMore: response.pagination.total > processedFiles.length,
        loading: false,
        total: response.pagination.total
      });
      setFilesLoaded(true);
      setLoadedDataset(requestedDataset); // Mark which dataset is loaded
    } catch (error) {
      // Ignore abort errors
      if (error.name === 'AbortError') {
        return;
      }
      
      // Ignore errors if processor or dataset changed
      if (processorType !== requestedProcessor || selectedDataset !== requestedDataset) {
        return;
      }
      
      setError(error.message || 'Failed to load files');
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

    const requestedDataset = selectedDataset; // Capture at request time
    const requestedProcessor = processorType; // Capture at request time
    const currentOffset = pagination.offset; // Capture current offset
    setPagination(prev => ({ ...prev, loading: true }));

    try {
      const response = await getDatasetFilesUnified(
        requestedDataset, 
        requestedProcessor, 
        fileType,
        currentOffset, 
        50,
        abortControllerRef.current?.signal
      );

      // Ignore if processor changed (cross-processor contamination) OR dataset changed (within processor)
      if (processorType !== requestedProcessor || selectedDataset !== requestedDataset) {
        return; // Ignore stale files
      }

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

      // Append to existing files
      setFiles(prev => [...prev, ...newFiles]);
      setPagination({
        offset: currentOffset + 50,
        hasMore: response.pagination.total > (currentOffset + newFiles.length),
        loading: false,
        total: response.pagination.total
      });
    } catch (error) {
      // Ignore abort errors
      if (error.name === 'AbortError') {
        return;
      }
      
      // Ignore errors if processor or dataset changed
      if (processorType !== requestedProcessor || selectedDataset !== requestedDataset) {
        return;
      }
      
      setPagination(prev => ({ ...prev, loading: false }));
    }
  }, [selectedDataset, processorType, fileType, pagination.offset, pagination.loading, pagination.hasMore, isValidFile, formatFileSize]);

  // Smart auto-refresh: only update total count, never disrupt loaded files
  const refreshFilesCount = useCallback(async () => {
    if (!selectedDataset || isRefreshing.current || pagination.loading) return;
    
    const requestedDataset = selectedDataset; // Capture at request time
    const requestedProcessor = processorType; // Capture at request time
    
    try {
      // Just check the first page to get updated total count
      const response = await getDatasetFilesUnified(
        requestedDataset, 
        requestedProcessor, 
        fileType, 
        0, 
        1,
        abortControllerRef.current?.signal
      );
      
      // Ignore if processor changed (cross-processor contamination) OR dataset changed (within processor)
      if (processorType !== requestedProcessor || selectedDataset !== requestedDataset) {
        return; // Ignore stale count
      }
      
      // Only update the total count if it has changed
      if (response.pagination.total !== pagination.total) {
        setPagination(prev => ({
          ...prev,
          total: response.pagination.total,
          hasMore: prev.offset < response.pagination.total
        }));
      }
    } catch (err) {
      // Ignore abort errors
      if (err.name === 'AbortError') {
        return;
      }
      
      // Ignore errors if processor or dataset changed
      if (processorType !== requestedProcessor || selectedDataset !== requestedDataset) {
        return;
      }
      
      // Silently fail for auto-refresh (don't show user error)
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
    setError(null); // Clear any previous errors
    await loadFiles(false);
  }, [loadFiles]);

  // Clear files when dataset selection changes
  useEffect(() => {
    
    // Don't load files if processor is changing or if dataset is from wrong processor
    if (isProcessorChanging.current) {
      return;
    }
    
    if (selectedDataset) {
      setFilesLoaded(false);
      setLoadedDataset(null); // Clear which dataset is loaded
      setFiles([]); // Clear files when changing datasets
      setPagination({ offset: 0, hasMore: false, loading: false, total: 0 }); // Reset pagination
      isRefreshing.current = false; // Reset refreshing state to allow new calls
      loadFiles();
    } else {
      setFiles([]);
      setFilesLoaded(false);
      setLoadedDataset(null);
      setPagination({ offset: 0, hasMore: false, loading: false, total: 0 });
      isRefreshing.current = false; // Reset refreshing state
    }
  }, [selectedDataset, loadFiles]);

  return {
    // State
    files,
    loading,
    error,
    filesLoaded,
    loadedDataset,
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
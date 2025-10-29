// API service for backend communication
// Use relative API URLs - nginx will proxy /api/ to the backend service
const API_BASE_URL = '/api';

/**
 * Standard headers for JSON API requests
 */
const STANDARD_HEADERS = {
  'Content-Type': 'application/json',
};

/**
 * Build URL with query parameters
 * @param {string} endpoint - API endpoint path
 * @param {Object} params - Query parameters object
 * @returns {string} - Complete URL with encoded parameters
 */
const buildUrl = (endpoint, params = {}) => {
  const url = new URL(`${API_BASE_URL}/${endpoint}`, window.location.origin);
  
  Object.entries(params).forEach(([key, value]) => {
    if (value !== null && value !== undefined) {
      url.searchParams.append(key, String(value));
    }
  });
  
  return url.toString();
};

/**
 * Process API response with standard error handling
 * @param {Response} response - Fetch response object
 * @param {string} context - Context for error messages
 * @returns {Promise<Object>} - Parsed response data
 */
const processResponse = async (response, context) => {
  if (!response.ok) {
    throw new Error(`Failed to ${context}: ${response.status}`);
  }
  
  const data = await response.json();
  
  if (data.success) {
    return data;
  } else {
    throw new Error(data.message || `Failed to ${context}`);
  }
};

/**
 * Standard API request with consistent error handling
 * @param {string} endpoint - API endpoint path
 * @param {Object} options - Request options
 * @returns {Promise<Object>} - API response data
 */
const apiRequest = async (endpoint, options = {}) => {
  const { 
    method = 'GET', 
    body, 
    signal, 
    params = {}, 
    context = endpoint.replace(/[/-]/g, ' ')
  } = options;
  
  const url = buildUrl(endpoint, params);
  
  const fetchOptions = {
    method,
    headers: STANDARD_HEADERS,
    signal
  };
  
  if (body) {
    fetchOptions.body = JSON.stringify(body);
  }
  
  const response = await fetch(url, fetchOptions);
  return processResponse(response, context);
};

/**
 * API request with try-catch wrapper returning success/error format
 * @param {string} endpoint - API endpoint path
 * @param {Object} options - Request options
 * @returns {Promise<Object>} - {success: boolean, message?: string, ...data}
 */
const apiRequestSafe = async (endpoint, options = {}) => {
  try {
    const data = await apiRequest(endpoint, options);
    return { success: true, ...data };
  } catch (error) {
    return { success: false, message: error.message };
  }
};

/**
 * Upload a single file to the backend
 * @param {File} file - The file to upload
 * @param {string} dataset - The dataset name for organizing files
 * @param {function} onProgress - Progress callback function
 * @returns {Promise<Object>} - Upload response
 */
export const uploadFile = async (file, dataset, onProgress = null, processorType, isConnectorMode = false) => {
  const formData = new FormData();
  formData.append('file', file);
  formData.append('dataset', dataset);
  formData.append('app', processorType);
  formData.append('connectorMode', isConnectorMode);

  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();

    // Track upload progress
    if (onProgress) {
      xhr.upload.addEventListener('progress', (event) => {
        if (event.lengthComputable) {
          const percentComplete = (event.loaded / event.total) * 100;
          onProgress(percentComplete);
        }
      });
    }

    xhr.onload = function() {
      if (xhr.status === 200) {
        try {
          const response = JSON.parse(xhr.responseText);
          resolve(response);
        } catch {
          reject(new Error('Invalid response format'));
        }
      } else {
        reject(new Error(`Upload failed with status: ${xhr.status}`));
      }
    };

    xhr.onerror = function() {
      reject(new Error('Network error occurred'));
    };

    xhr.open('POST', `${API_BASE_URL}/upload`);
    xhr.send(formData);
  });
};

/**
 * Check if the backend server is healthy
 * @returns {Promise<Object>} - Health check response
 */
export const checkHealth = async () => {
  return apiRequest('health', { context: 'health check' });
};

/**
 * Upload multiple files sequentially
 * @param {Array<File>} files - Array of files to upload
 * @param {string} dataset - The dataset name for organizing files
 * @param {function} onFileProgress - Progress callback for individual files
 * @param {function} onOverallProgress - Progress callback for overall upload
 * @returns {Promise<Array>} - Array of upload responses
 */
export const uploadFiles = async (files, dataset, onFileProgress = null, onOverallProgress = null, processorType, isConnectorMode = false) => {
  const results = [];
  const totalFiles = files.length;
  
  for (let i = 0; i < totalFiles; i++) {
    const file = files[i];
    
    try {
      const result = await uploadFile(file, dataset, (progress) => {
        if (onFileProgress) {
          onFileProgress(file, progress);
        }
      }, processorType, isConnectorMode);
      
      results.push({
        file: file,
        success: true,
        result: result
      });
      
      if (onOverallProgress) {
        onOverallProgress(((i + 1) / totalFiles) * 100);
      }
      
    } catch (error) {
      results.push({
        file: file,
        success: false,
        error: error.message
      });
      
      if (onOverallProgress) {
        onOverallProgress(((i + 1) / totalFiles) * 100);
      }
    }
  }
  
  return results;
};

/**
 * Get list of existing datasets
 * @param {string} processorType - The processor type (starlight, ppxf, etc.)
 * @param {AbortSignal} signal - Optional AbortSignal for request cancellation
 * @returns {Promise<Array>} - Array of dataset names
 */
export const getDatasets = async (processorType, signal = null) => {
  if (!processorType) {
    throw new Error('getDatasets: processorType is required');
  }
  
  try {
    const data = await apiRequest('datasets', { 
      params: { app: processorType },
      signal,
      context: 'fetch datasets'
    });
    
    return data.datasets || [];
  } catch (error) {
    throw error;
  }
};

/**
 * Create a new dataset in S3
 * @param {string} datasetName - The name of the dataset to create
 * @param {string} processorType - The processor type (starlight, ppxf, steckmap)
 * @param {Object} ppxfConfig - Optional pPXF configuration (only for ppxf datasets)
 * @returns {Promise<Object>} - Creation response
 */
export const createDataset = async (datasetName, processorType, ppxfConfig = null, isConnectorMode = false) => {
  const requestBody = {
    datasetName: datasetName,
    appType: processorType,
    connectorMode: isConnectorMode
  };
  
  // Add pPXF config if provided and processor is pPXF
  if (processorType.toLowerCase() === 'ppxf' && ppxfConfig) {
    requestBody.ppxfConfig = ppxfConfig;
  }
  
  return apiRequestSafe('datasets/create', {
    method: 'POST',
    body: requestBody,
    context: 'create dataset'
  });
};

/**
 * Delete a dataset by removing all its directories
 * @param {string} datasetName - The name of the dataset to delete
 * @param {string} appType - The application type (default: 'starlight')
 * @returns {Promise<Object>} - Delete response
 */
export const deleteDataset = async (datasetName, processorType) => {
  return apiRequestSafe('datasets/delete', {
    method: 'DELETE',
    params: { dataset: datasetName, app: processorType },
    context: 'delete dataset'
  });
};

/**
 * Delete a specific file from S3
 * @param {string} fileKey - The S3 key/path of the file to delete
 * @param {string} appType - The application type (default: 'starlight')
 * @returns {Promise<Object>} - Delete response
 */
export const deleteFile = async (fileKey, processorType) => {
  return apiRequestSafe('files/delete', {
    method: 'DELETE',
    params: { key: fileKey, app: processorType },
    context: 'delete file'
  });
};

/**
 * Get download URL for bulk downloading all files from a dataset as a zip
 * @param {string} datasetName - The dataset name
 * @param {string} processorType - The processor type (starlight, ppxf, etc.)
 * @param {string} fileType - The file type (input, processed, output)
 * @returns {string} - Download URL
 */
export const getDownloadAllUrl = (datasetName, processorType, fileType = 'output') => {
  const params = new URLSearchParams({
    dataset: datasetName,
    app: processorType,
    type: fileType
  });
  return `${API_BASE_URL}/files/download-all?${params.toString()}`;
};

/**
 * Get list of files in a specific dataset
 * @param {string} datasetName - The name of the dataset
 * @returns {Promise<Array>} - Array of file objects
 */
export const getDatasetFiles = async (datasetName, processorType) => {
  if (!processorType) {
    throw new Error('getDatasetFiles: processorType is required');
  }
  
  const data = await apiRequest('datasets/files', {
    params: { dataset: datasetName, app: processorType },
    context: 'fetch dataset files'
  });
  
  return data.files || [];
};

/**
 * UNIFIED: Get dataset files with robust offset-based pagination
 * @param {string} datasetName - Name of the dataset  
 * @param {string} processorType - Type of processor (starlight, ppxf, etc.)
 * @param {string} fileType - Type of files (input, processed, output)
 * @param {number} offset - Starting position (0-based)
 * @param {number} limit - Number of files to return per page
 * @param {AbortSignal} signal - Optional AbortSignal for request cancellation
 * @returns {Promise<Object>} - Object with files array, pagination info
 */
export const getDatasetFilesUnified = async (datasetName, processorType, fileType, offset = 0, limit = 50, signal = null) => {
  if (!processorType) {
    throw new Error('getDatasetFilesUnified: processorType is required');
  }
  if (!fileType) {
    throw new Error('getDatasetFilesUnified: fileType is required (input, processed, output)');
  }
  
  const data = await apiRequest('datasets/files-unified', {
    params: { 
      dataset: datasetName, 
      app: processorType, 
      type: fileType, 
      offset, 
      limit 
    },
    signal,
    context: `fetch ${fileType} files`
  });
  
  return {
    files: data.files || [],
    pagination: data.pagination || { offset: 0, limit: 50, total: 0, hasMore: false }
  };
};

/**
 * Get list of input files in a specific dataset with pagination
 * @param {string} datasetName - The name of the dataset
 * @param {string} processorType - The processor type (starlight, ppxf, etc.)
 * @param {number} page - Page number (0-based)
 * @param {number} limit - Number of files to return per page
 * @param {AbortSignal} signal - Optional AbortSignal for request cancellation
 * @returns {Promise<Object>} - Object with files array, hasMore, total, page, limit
 */
export const getDatasetFilesListPaginated = async (datasetName, processorType, page = 0, limit = 50, signal = null) => {
  if (!processorType) {
    throw new Error('getDatasetFilesListPaginated: processorType is required');
  }
  
  const data = await apiRequest('datasets/files', {
    params: { 
      dataset: datasetName, 
      app: processorType, 
      page, 
      limit 
    },
    signal,
    context: 'fetch paginated dataset input files list'
  });
  
  // Backend now supports pagination on the regular endpoint
  if (data.pagination) {
    return {
      files: data.files || [],
      hasMore: data.pagination.hasMore || false,
      total: data.pagination.total || 0,
      page: data.pagination.page || page,
      limit: data.pagination.limit || limit
    };
  } else {
    // No pagination requested, return all files
    return {
      files: data.files || [],
      hasMore: false,
      total: data.files?.length || 0,
      page,
      limit
    };
  }
};

/**
 * Get list of output files in a specific dataset
 * @param {string} datasetName - The name of the dataset
 * @returns {Promise<Array>} - Array of output file objects
 */
export const getDatasetOutputFiles = async (datasetName, processorType) => {
  if (!processorType) {
    throw new Error('getDatasetOutputFiles: processorType is required');
  }
  
  const data = await apiRequest('datasets/output-files', {
    params: { dataset: datasetName, app: processorType },
    context: 'fetch dataset output files'
  });
  
  return data.files || [];
};

/**
 * Get list of output files in a specific dataset with pagination
 * @param {string} datasetName - The name of the dataset
 * @param {string} processorType - The processor type (starlight, ppxf, etc.)
 * @param {number} page - Page number (0-based)
 * @param {number} limit - Number of files to return per page
 * @param {AbortSignal} signal - Optional AbortSignal for request cancellation
 * @returns {Promise<Object>} - Object with files array, hasMore, total, page, limit
 */
export const getDatasetOutputFilesListPaginated = async (datasetName, processorType, page = 0, limit = 50, signal = null) => {
  if (!processorType) {
    throw new Error('getDatasetOutputFilesListPaginated: processorType is required');
  }
  
  const data = await apiRequest('datasets/output-files', {
    params: { 
      dataset: datasetName, 
      app: processorType, 
      page, 
      limit 
    },
    signal,
    context: 'fetch paginated dataset output files list'
  });
  
  // Backend now supports pagination on the regular endpoint
  if (data.pagination) {
    return {
      files: data.files || [],
      hasMore: data.pagination.hasMore || false,
      total: data.pagination.total || 0,
      page: data.pagination.page || page,
      limit: data.pagination.limit || limit
    };
  } else {
    // No pagination requested, return all files
    return {
      files: data.files || [],
      hasMore: false,
      total: data.files?.length || 0,
      page,
      limit
    };
  }
};

/**
 * Get paginated output files for a dataset (special endpoint optimized for large datasets)
 * @param {string} datasetName - The name of the dataset
 * @param {string} processorType - The processor type (starlight, ppxf, etc.)
 * @param {number} limit - Number of files to return per page
 * @param {number} offset - Starting position (0-based)
 * @returns {Promise<Object>} - Object with files array, total, hasMore, offset, limit
 */
export const getDatasetOutputFilesPaginated = async (datasetName, processorType, limit = 50, offset = 0) => {
  if (!processorType) {
    throw new Error('getDatasetOutputFilesPaginated: processorType is required');
  }
  
  const data = await apiRequest('datasets/output-files-paginated', {
    params: { 
      dataset: datasetName, 
      app: processorType, 
      limit, 
      offset 
    },
    context: 'fetch paginated output files'
  });
  
  // Filter for PDF files that are in cell subdirectories (contain "/")
  const pdfFiles = data.files.filter(file => 
    file.name.includes('/') && 
    file.name.toLowerCase().endsWith('.pdf')
  );
  
  // Sort PDF files by cell number (numeric sort)
  pdfFiles.sort((a, b) => {
    const cellA = parseInt(a.name.split('/')[0]);
    const cellB = parseInt(b.name.split('/')[0]);
    return cellA - cellB;
  });
  
  return {
    files: pdfFiles,
    total: data.total,
    hasMore: data.hasMore,
    offset: offset,
    limit: limit
  };
};

// Pipeline Progress API functions
export const getAllPipelineProgress = async () => {
  try {
    const data = await apiRequest('progress/all', { 
      context: 'fetch all pipeline progress' 
    });
    return data.progress || {};
  } catch (error) {
    // Handle 404 or other "not found" responses gracefully
    if (error.message.includes('404') || error.message.includes('Failed to fetch all pipeline progress: 404')) {
      // Progress endpoint doesn't exist or no data available - return empty object
      return {};
    }
    
    // Handle network errors or other fetch failures
    if (error.message.includes('fetch') || error.name === 'TypeError') {
      return {}; // Return empty object for graceful fallback
    }
    
    throw error; // Re-throw for genuine errors
  }
};

export const getDatasetPipelineProgress = async (datasetId) => {
  try {
    const data = await apiRequest('progress', {
      params: { dataset_id: datasetId },
      context: 'fetch dataset pipeline progress'
    });
    return data.progress;
  } catch (error) {
    // Handle 404 or other "not found" responses gracefully
    if (error.message.includes('404') || error.message.includes('Failed to fetch dataset pipeline progress: 404')) {
      // Progress endpoint doesn't exist or no data available - return null
      return null;
    }
    
    // Handle network errors or other fetch failures
    if (error.message.includes('fetch') || error.name === 'TypeError') {
      return null; // Return null for graceful fallback
    }
    
    throw error; // Re-throw for genuine errors
  }
};

export const updatePipelineProgress = async (progressData) => {
  return apiRequest('progress/update', {
    method: 'POST',
    body: progressData,
    context: 'update pipeline progress'
  });
};

/**
 * Start processing for a specific dataset
 * @param {string} datasetName - The name of the dataset to process
 * @param {string} processorType - The processor type (starlight, ppxf, steckmap)
 * @returns {Promise<Object>} - Processing response
 */
export const startProcessing = async (datasetName, processorType) => {
  return apiRequest('datasets/process', {
    method: 'POST',
    body: {
      dataset: datasetName,
      processorType: processorType
    },
    context: 'start processing'
  });
};

/**
 * Start processing for a single file in a dataset
 * @param {string} datasetName - The name of the dataset
 * @param {string} fileName - The name of the file to process
 * @param {string} processorType - The processor type (starlight, ppxf, steckmap)
 * @returns {Promise<Object>} - Processing response
 */
export const startSingleFileProcessing = async (datasetName, fileName, processorType) => {
  return apiRequest('files/process', {
    method: 'POST',
    body: {
      dataset: datasetName,
      fileName: fileName,
      processorType: processorType
    },
    context: 'start single file processing'
  });
}; 
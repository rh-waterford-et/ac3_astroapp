// API service for backend communication
// Use relative API URLs - nginx will proxy /api/ to the backend service
const API_BASE_URL = '/api';

/**
 * Upload a single file to the backend
 * @param {File} file - The file to upload
 * @param {string} dataset - The dataset name for organizing files
 * @param {function} onProgress - Progress callback function
 * @returns {Promise<Object>} - Upload response
 */
export const uploadFile = async (file, dataset, onProgress = null, processorType) => {
  const formData = new FormData();
  formData.append('file', file);
  formData.append('dataset', dataset);
  formData.append('app', processorType);

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
  try {
    const response = await fetch(`${API_BASE_URL}/health`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
    });

    if (!response.ok) {
      throw new Error(`Health check failed with status: ${response.status}`);
    }

    return await response.json();
  } catch (error) {
    throw new Error(`Health check failed: ${error.message}`);
  }
};

/**
 * Upload multiple files sequentially
 * @param {Array<File>} files - Array of files to upload
 * @param {string} dataset - The dataset name for organizing files
 * @param {function} onFileProgress - Progress callback for individual files
 * @param {function} onOverallProgress - Progress callback for overall upload
 * @returns {Promise<Array>} - Array of upload responses
 */
export const uploadFiles = async (files, dataset, onFileProgress = null, onOverallProgress = null, processorType) => {
  const results = [];
  const totalFiles = files.length;
  
  for (let i = 0; i < totalFiles; i++) {
    const file = files[i];
    
    try {
      const result = await uploadFile(file, dataset, (progress) => {
        if (onFileProgress) {
          onFileProgress(file, progress);
        }
      }, processorType);
      
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
 * @returns {Promise<Array>} - Array of dataset names
 */
export const getDatasets = async (processorType) => {
  if (!processorType) {
    throw new Error('getDatasets: processorType is required');
  }
  const response = await fetch(`${API_BASE_URL}/datasets?app=${encodeURIComponent(processorType)}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
  });

  if (!response.ok) {
    throw new Error(`Failed to fetch datasets: ${response.status}`);
  }

  const data = await response.json();
  
  if (data.success) {
    return data.datasets || [];
  } else {
    throw new Error('Failed to fetch datasets');
  }
};

/**
 * Create a new dataset in S3
 * @param {string} datasetName - The name of the dataset to create
 * @param {string} processorType - The processor type (starlight, ppxf, steckmap)
 * @param {Object} ppxfConfig - Optional pPXF configuration (only for ppxf datasets)
 * @returns {Promise<Object>} - Creation response
 */
export const createDataset = async (datasetName, processorType, ppxfConfig = null) => {
  try {
    
    const requestBody = {
      datasetName: datasetName,
      appType: processorType
    };
    
    // Add pPXF config if provided and processor is pPXF
    if (processorType.toLowerCase() === 'ppxf' && ppxfConfig) {
      requestBody.ppxfConfig = ppxfConfig;
    }
    
    const response = await fetch(`${API_BASE_URL}/datasets/create`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(requestBody),
    });


    if (!response.ok) {
      throw new Error(`Failed to create dataset: ${response.status}`);
    }

    const data = await response.json();
    
    if (data.success) {
      return { success: true, message: data.message };
    } else {
      throw new Error(data.message || 'Failed to create dataset');
    }
  } catch (error) {
    return { success: false, message: error.message || 'Failed to create dataset' };
  }
};

/**
 * Delete a dataset by removing all its directories
 * @param {string} datasetName - The name of the dataset to delete
 * @param {string} appType - The application type (default: 'starlight')
 * @returns {Promise<Object>} - Delete response
 */
export const deleteDataset = async (datasetName, processorType) => {
  try {
    const response = await fetch(`${API_BASE_URL}/datasets/delete?dataset=${encodeURIComponent(datasetName)}&app=${encodeURIComponent(processorType)}`, {
      method: 'DELETE',
      headers: {
        'Content-Type': 'application/json',
      },
    });

    if (!response.ok) {
      throw new Error(`Failed to delete dataset: ${response.status}`);
    }

    const data = await response.json();
    
    if (data.success) {
      return { success: true, message: data.message };
    } else {
      throw new Error(data.message || 'Failed to delete dataset');
    }
  } catch (error) {
    return { success: false, message: error.message || 'Failed to delete dataset' };
  }
};

/**
 * Delete a specific file from S3
 * @param {string} fileKey - The S3 key/path of the file to delete
 * @param {string} appType - The application type (default: 'starlight')
 * @returns {Promise<Object>} - Delete response
 */
export const deleteFile = async (fileKey, processorType) => {
  try {
    const response = await fetch(`${API_BASE_URL}/files/delete?key=${encodeURIComponent(fileKey)}&app=${encodeURIComponent(processorType)}`, {
      method: 'DELETE',
      headers: {
        'Content-Type': 'application/json',
      },
    });

    if (!response.ok) {
      throw new Error(`Failed to delete file: ${response.status}`);
    }

    const data = await response.json();
    
    if (data.success) {
      return { success: true, message: data.message };
    } else {
      throw new Error(data.message || 'Failed to delete file');
    }
  } catch (error) {
    return { success: false, message: error.message || 'Failed to delete file' };
  }
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
  const response = await fetch(`${API_BASE_URL}/datasets/files?dataset=${encodeURIComponent(datasetName)}&app=${encodeURIComponent(processorType)}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
  });

  if (!response.ok) {
    throw new Error(`Failed to fetch dataset files: ${response.status}`);
  }

  const data = await response.json();
  
  if (data.success) {
    return data.files || [];
  } else {
    throw new Error(data.message || 'Failed to fetch dataset files');
  }
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
  
  const response = await fetch(`${API_BASE_URL}/datasets/files-unified?dataset=${encodeURIComponent(datasetName)}&app=${encodeURIComponent(processorType)}&type=${encodeURIComponent(fileType)}&offset=${offset}&limit=${limit}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
    signal: signal,
  });

  if (!response.ok) {
    throw new Error(`Failed to fetch ${fileType} files: ${response.status}`);
  }

  const data = await response.json();
  
  if (data.success) {
    return {
      files: data.files || [],
      pagination: data.pagination || { offset: 0, limit: 50, total: 0, hasMore: false }
    };
  } else {
    throw new Error(data.message || `Failed to fetch ${fileType} files`);
  }
};

/**
 * DEPRECATED: Get input files for a dataset with pagination - USE getDatasetFilesUnified INSTEAD
 * @param {string} datasetName - Name of the dataset
 * @param {string} processorType - Type of processor (starlight, ppxf, etc.)
 * @param {number} page - Page number (0-based)
 * @param {number} limit - Number of files to return per page
 * @param {AbortSignal} signal - Optional AbortSignal for request cancellation
 * @returns {Promise<Object>} - Object with files array, hasMore, total, page, limit
 */
export const getDatasetFilesListPaginated = async (datasetName, processorType, page = 0, limit = 50, signal = null) => {
  if (!processorType) {
    throw new Error('getDatasetFilesListPaginated: processorType is required');
  }
  
  const response = await fetch(`${API_BASE_URL}/datasets/files?dataset=${encodeURIComponent(datasetName)}&app=${encodeURIComponent(processorType)}&page=${page}&limit=${limit}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
    signal: signal,
  });

  if (!response.ok) {
    throw new Error(`Failed to fetch paginated dataset input files list: ${response.status}`);
  }

  const data = await response.json();
  
  if (data.success) {
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
  } else {
    throw new Error(data.message || 'Failed to fetch paginated dataset input files list');
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
  const response = await fetch(`${API_BASE_URL}/datasets/output-files?dataset=${encodeURIComponent(datasetName)}&app=${encodeURIComponent(processorType)}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
  });

  if (!response.ok) {
    throw new Error(`Failed to fetch dataset output files: ${response.status}`);
  }

  const data = await response.json();
  
  if (data.success) {
    return data.files || [];
  } else {
    throw new Error(data.message || 'Failed to fetch dataset output files');
  }
};

/**
 * DEPRECATED: Get output files for a dataset with pagination - USE getDatasetFilesUnified INSTEAD
 * @param {string} datasetName - Name of the dataset
 * @param {string} processorType - Type of processor (starlight, ppxf, etc.)
 * @param {number} page - Page number (0-based)
 * @param {number} limit - Number of files to return per page
 * @param {AbortSignal} signal - Optional AbortSignal for request cancellation
 * @returns {Promise<Object>} - Object with files array, hasMore, total, page, limit
 */
export const getDatasetOutputFilesListPaginated = async (datasetName, processorType, page = 0, limit = 50, signal = null) => {
  if (!processorType) {
    throw new Error('getDatasetOutputFilesListPaginated: processorType is required');
  }
  
  const response = await fetch(`${API_BASE_URL}/datasets/output-files?dataset=${encodeURIComponent(datasetName)}&app=${encodeURIComponent(processorType)}&page=${page}&limit=${limit}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
    signal: signal,
  });

  if (!response.ok) {
    throw new Error(`Failed to fetch paginated dataset output files list: ${response.status}`);
  }

  const data = await response.json();
  
  if (data.success) {
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
  } else {
    throw new Error(data.message || 'Failed to fetch paginated dataset output files list');
  }
};

/**
 * Get output files for a dataset with pagination for progressive loading
 * @param {string} datasetName - Name of the dataset
 * @param {string} processorType - Type of processor (starlight, ppxf, etc.)
 * @param {number} limit - Number of files to return (default: 50)
 * @param {number} offset - Starting offset for pagination (default: 0)
 * @returns {Promise<Object>} - Object with files array, total, hasMore, offset, limit
 */
export const getDatasetOutputFilesPaginated = async (datasetName, processorType, limit = 50, offset = 0) => {
  if (!processorType) {
    throw new Error('getDatasetOutputFilesPaginated: processorType is required');
  }
  
  try {
    const response = await fetch(`${API_BASE_URL}/datasets/output-files-paginated?dataset=${encodeURIComponent(datasetName)}&app=${encodeURIComponent(processorType)}&limit=${limit}&offset=${offset}`);
    
    if (!response.ok) {
      throw new Error(`Failed to fetch paginated output files: ${response.status}`);
    }
    
    const data = await response.json();
    
    if (!data.success) {
      throw new Error(data.message || 'Failed to fetch paginated output files');
    }
    
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
  } catch (error) {
    throw error;
  }
};

// Pipeline Progress API functions
export const getAllPipelineProgress = async () => {
  try {
    const response = await fetch(`${API_BASE_URL}/progress/all`);
    
    // Handle 404 or other "not found" responses gracefully
    if (!response.ok) {
      if (response.status === 404) {
        // Progress endpoint doesn't exist or no data available - return empty object
        return {};
      }
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }
    
    const data = await response.json();
    
    if (data.success) {
      return data.progress || {};
    } else {
      // If API returns success: false, treat as "no data available"
      return {};
    }
  } catch (error) {
    // Handle network errors or other fetch failures
    if (error.message.includes('fetch') || error.name === 'TypeError') {
      return {}; // Return empty object for graceful fallback
    }
    throw error; // Re-throw for genuine errors
  }
};

export const getDatasetPipelineProgress = async (datasetId) => {
  try {
    const response = await fetch(`${API_BASE_URL}/progress?dataset_id=${encodeURIComponent(datasetId)}`);
    
    // Handle 404 or other "not found" responses gracefully
    if (!response.ok) {
      if (response.status === 404) {
        // Progress endpoint doesn't exist or no data available - return null
        return null;
      }
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }
    
    const data = await response.json();
    
    if (data.success) {
      return data.progress;
    } else {
      // If API returns success: false, treat as "no data available"
      return null;
    }
  } catch (error) {
    // Handle network errors or other fetch failures
    if (error.message.includes('fetch') || error.name === 'TypeError') {
      return null; // Return null for graceful fallback
    }
    throw error; // Re-throw for genuine errors
  }
};

export const updatePipelineProgress = async (progressData) => {
  try {
    const response = await fetch(`${API_BASE_URL}/progress/update`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(progressData),
    });
    
    const data = await response.json();
    
    if (data.success) {
      return data;
    } else {
      throw new Error(data.message || 'Failed to update progress');
    }
  } catch (error) {
    throw error;
  }
};

/**
 * Start processing for a specific dataset
 * @param {string} datasetName - The name of the dataset to process
 * @param {string} processorType - The processor type (starlight, ppxf, steckmap)
 * @returns {Promise<Object>} - Processing response
 */
export const startProcessing = async (datasetName, processorType) => {
  try {
    const response = await fetch(`${API_BASE_URL}/datasets/process`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        dataset: datasetName,
        processorType: processorType
      }),
    });
    
    const data = await response.json();
    
    if (data.success) {
      return data;
    } else {
      throw new Error(data.message || 'Failed to start processing');
    }
  } catch (error) {
    throw error;
  }
};

/**
 * Start processing for a single file
 * @param {string} datasetName - The name of the dataset containing the file
 * @param {string} fileName - The name of the specific file to process
 * @param {string} processorType - The processor type (starlight, ppxf, steckmap)
 * @returns {Promise<Object>} - Processing response
 */
export const startSingleFileProcessing = async (datasetName, fileName, processorType) => {
  try {
    const response = await fetch(`${API_BASE_URL}/files/process`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        dataset: datasetName,
        fileName: fileName,
        processorType: processorType
      }),
    });
    
    const data = await response.json();
    
    if (data.success) {
      return data;
    } else {
      throw new Error(data.message || 'Failed to start single file processing');
    }
  } catch (error) {
    throw error;
  }
}; 
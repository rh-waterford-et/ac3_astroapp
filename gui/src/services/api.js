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
  console.log('Fetching datasets from:', `${API_BASE_URL}/datasets?app=${processorType}`);
  const response = await fetch(`${API_BASE_URL}/datasets?app=${encodeURIComponent(processorType)}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
  });

  console.log('Response status:', response.status);
  console.log('Response ok:', response.ok);

  if (!response.ok) {
    throw new Error(`Failed to fetch datasets: ${response.status}`);
  }

  const data = await response.json();
  console.log('Datasets response data:', data);
  
  if (data.success) {
    console.log('Returning datasets:', data.datasets || []);
    return data.datasets || [];
  } else {
    throw new Error('Failed to fetch datasets');
  }
};

/**
 * Create a new dataset in S3
 * @param {string} datasetName - The name of the dataset to create
 * @returns {Promise<Object>} - Creation response
 */
export const createDataset = async (datasetName, processorType) => {
  try {
    console.log('Creating dataset:', datasetName, 'for processor:', processorType);
    const response = await fetch(`${API_BASE_URL}/datasets/create`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
              body: JSON.stringify({
          datasetName: datasetName,
          appType: processorType
        }),
    });

    console.log('Create dataset response status:', response.status);
    console.log('Create dataset response ok:', response.ok);

    if (!response.ok) {
      throw new Error(`Failed to create dataset: ${response.status}`);
    }

    const data = await response.json();
    console.log('Create dataset response data:', data);
    
    if (data.success) {
      return { success: true, message: data.message };
    } else {
      throw new Error(data.message || 'Failed to create dataset');
    }
  } catch (error) {
    console.error('Error creating dataset:', error);
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
    console.log('Deleting dataset:', datasetName, 'for processor:', processorType);
    const response = await fetch(`${API_BASE_URL}/datasets/delete?dataset=${encodeURIComponent(datasetName)}&app=${encodeURIComponent(processorType)}`, {
      method: 'DELETE',
      headers: {
        'Content-Type': 'application/json',
      },
    });

    console.log('Delete dataset response status:', response.status);
    console.log('Delete dataset response ok:', response.ok);

    if (!response.ok) {
      throw new Error(`Failed to delete dataset: ${response.status}`);
    }

    const data = await response.json();
    console.log('Delete dataset response data:', data);
    
    if (data.success) {
      return { success: true, message: data.message };
    } else {
      throw new Error(data.message || 'Failed to delete dataset');
    }
  } catch (error) {
    console.error('Error deleting dataset:', error);
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
    console.log('Deleting file:', fileKey, 'for processor:', processorType);
    const response = await fetch(`${API_BASE_URL}/files/delete?key=${encodeURIComponent(fileKey)}&app=${encodeURIComponent(processorType)}`, {
      method: 'DELETE',
      headers: {
        'Content-Type': 'application/json',
      },
    });

    console.log('Delete file response status:', response.status);
    console.log('Delete file response ok:', response.ok);

    if (!response.ok) {
      throw new Error(`Failed to delete file: ${response.status}`);
    }

    const data = await response.json();
    console.log('Delete file response data:', data);
    
    if (data.success) {
      return { success: true, message: data.message };
    } else {
      throw new Error(data.message || 'Failed to delete file');
    }
  } catch (error) {
    console.error('Error deleting file:', error);
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
  console.log('Fetching files for dataset:', datasetName, 'processor:', processorType);
  const response = await fetch(`${API_BASE_URL}/datasets/files?dataset=${encodeURIComponent(datasetName)}&app=${encodeURIComponent(processorType)}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
  });

  console.log('Dataset files response status:', response.status);
  console.log('Dataset files response ok:', response.ok);

  if (!response.ok) {
    throw new Error(`Failed to fetch dataset files: ${response.status}`);
  }

  const data = await response.json();
  console.log('Dataset files response data:', data);
  
  if (data.success) {
    console.log('Returning dataset files:', data.files || []);
    return data.files || [];
  } else {
    throw new Error(data.message || 'Failed to fetch dataset files');
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
  console.log('Fetching output files for dataset:', datasetName, 'processor:', processorType);
  const response = await fetch(`${API_BASE_URL}/datasets/output-files?dataset=${encodeURIComponent(datasetName)}&app=${encodeURIComponent(processorType)}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
  });

  console.log('Dataset output files response status:', response.status);
  console.log('Dataset output files response ok:', response.ok);

  if (!response.ok) {
    throw new Error(`Failed to fetch dataset output files: ${response.status}`);
  }

  const data = await response.json();
  console.log('Dataset output files response data:', data);
  
  if (data.success) {
    console.log('Returning dataset output files:', data.files || []);
    return data.files || [];
  } else {
    throw new Error(data.message || 'Failed to fetch dataset output files');
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
  
  console.log(`📄 Fetching paginated output files for dataset: ${datasetName}, processor: ${processorType}, limit: ${limit}, offset: ${offset}`);
  
  try {
    const response = await fetch(`${API_BASE_URL}/datasets/output-files-paginated?dataset=${encodeURIComponent(datasetName)}&app=${encodeURIComponent(processorType)}&limit=${limit}&offset=${offset}`);
    
    console.log('Dataset paginated output files response status:', response.status);
    console.log('Dataset paginated output files response ok:', response.ok);
    
    if (!response.ok) {
      throw new Error(`Failed to fetch paginated output files: ${response.status}`);
    }
    
    const data = await response.json();
    console.log('Dataset paginated output files response data:', data);
    
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
    
    console.log(`📄 Returning ${pdfFiles.length} PDF files (${offset}-${offset + pdfFiles.length - 1} of ${data.total} total)`);
    
    return {
      files: pdfFiles,
      total: data.total,
      hasMore: data.hasMore,
      offset: offset,
      limit: limit
    };
  } catch (error) {
    console.error('❌ Failed to fetch paginated output files:', error);
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
        console.log('Progress endpoint not found or no data available');
        return {};
      }
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }
    
    const data = await response.json();
    
    if (data.success) {
      return data.progress || {};
    } else {
      // If API returns success: false, treat as "no data available"
      console.log('No pipeline progress data available');
      return {};
    }
  } catch (error) {
    // Handle network errors or other fetch failures
    if (error.message.includes('fetch') || error.name === 'TypeError') {
      console.log('Pipeline progress API not available, using fallback');
      return {}; // Return empty object for graceful fallback
    }
    console.error('Error fetching pipeline progress:', error);
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
        console.log('Dataset progress not found or no data available');
        return null;
      }
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }
    
    const data = await response.json();
    
    if (data.success) {
      return data.progress;
    } else {
      // If API returns success: false, treat as "no data available"
      console.log('No dataset progress data available');
      return null;
    }
  } catch (error) {
    // Handle network errors or other fetch failures
    if (error.message.includes('fetch') || error.name === 'TypeError') {
      console.log('Dataset progress API not available, using fallback');
      return null; // Return null for graceful fallback
    }
    console.error('Error fetching dataset progress:', error);
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
    console.error('Error updating progress:', error);
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
    console.error('Error starting processing:', error);
    throw error;
  }
}; 
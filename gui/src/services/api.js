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
export const uploadFile = async (file, dataset, onProgress = null) => {
  const formData = new FormData();
  formData.append('file', file);
  formData.append('dataset', dataset);

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
export const uploadFiles = async (files, dataset, onFileProgress = null, onOverallProgress = null) => {
  const results = [];
  const totalFiles = files.length;
  
  for (let i = 0; i < totalFiles; i++) {
    const file = files[i];
    
    try {
      const result = await uploadFile(file, dataset, (progress) => {
        if (onFileProgress) {
          onFileProgress(file, progress);
        }
      });
      
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
export const getDatasets = async () => {
  console.log('Fetching datasets from:', `${API_BASE_URL}/datasets`);
  const response = await fetch(`${API_BASE_URL}/datasets`, {
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
export const createDataset = async (datasetName) => {
  try {
    console.log('Creating dataset:', datasetName);
    const response = await fetch(`${API_BASE_URL}/datasets/create`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        datasetName: datasetName
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
export const deleteDataset = async (datasetName, appType = 'starlight') => {
  try {
    console.log('Deleting dataset:', datasetName, 'for app:', appType);
    const response = await fetch(`${API_BASE_URL}/datasets/delete?dataset=${encodeURIComponent(datasetName)}&app=${encodeURIComponent(appType)}`, {
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
export const deleteFile = async (fileKey, appType = 'starlight') => {
  try {
    console.log('Deleting file:', fileKey, 'for app:', appType);
    const response = await fetch(`${API_BASE_URL}/files/delete?key=${encodeURIComponent(fileKey)}&app=${encodeURIComponent(appType)}`, {
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
export const getDatasetFiles = async (datasetName) => {
  console.log('Fetching files for dataset:', datasetName);
  const response = await fetch(`${API_BASE_URL}/datasets/files?dataset=${encodeURIComponent(datasetName)}`, {
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
export const getDatasetOutputFiles = async (datasetName) => {
  console.log('Fetching output files for dataset:', datasetName);
  const response = await fetch(`${API_BASE_URL}/datasets/output-files?dataset=${encodeURIComponent(datasetName)}`, {
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
import React, { useState, useEffect, useCallback, useRef } from 'react';
import PropTypes from 'prop-types';
import FileUpload from './FileUpload';
import PipelineProgress from './PipelineProgress';

import FilesPane from './FilesPane';
import DatasetsPane from './DatasetsPane';
import { useDatasetOperations } from '../../hooks/data/useDatasetOperations';
import { usePaginatedFiles } from '../../hooks/data/usePaginatedFiles';
import { useAutoRefresh } from '../../hooks/data/useAutoRefresh';


function DatasetsList({ processorType }) {
  // Extract data management into hooks
  const datasetOps = useDatasetOperations(processorType);
  const inputFilesData = usePaginatedFiles(datasetOps.selectedDataset, 'input', processorType);
  const outputFilesData = usePaginatedFiles(datasetOps.selectedDataset, 'output', processorType);
  
  // Helper functions for localStorage persistence
  const getStoredCollapseState = (key, defaultValue = false) => {
    try {
      const stored = localStorage.getItem(`pipeline-${key}-collapsed`);
      return stored !== null ? JSON.parse(stored) : defaultValue;
    } catch (error) {
      console.error(`Error reading ${key} collapse state:`, error);
      return defaultValue;
    }
  };

  const setStoredCollapseState = (key, value) => {
    try {
      localStorage.setItem(`pipeline-${key}-collapsed`, JSON.stringify(value));
    } catch (error) {
      console.error(`Error storing ${key} collapse state:`, error);
    }
  };

  // Initialize collapsed states from localStorage, defaulting to false (not collapsed)
  const [isUploadCollapsed, setIsUploadCollapsed] = useState(() => 
    getStoredCollapseState('upload', false)
  );
  const [isDatasetsCollapsed, setIsDatasetsCollapsed] = useState(() => 
    getStoredCollapseState('datasets', false)
  );
  const [isPipelineProgressCollapsed, setIsPipelineProgressCollapsed] = useState(() => 
    getStoredCollapseState('progress', false)
  );

  // Enhanced setters that also save to localStorage
  const toggleUploadCollapsed = () => {
    const newState = !isUploadCollapsed;
    setIsUploadCollapsed(newState);
    setStoredCollapseState('upload', newState);
  };

  const toggleDatasetsCollapsed = () => {
    const newState = !isDatasetsCollapsed;
    setIsDatasetsCollapsed(newState);
    setStoredCollapseState('datasets', newState);
  };

  const togglePipelineProgressCollapsed = () => {
    const newState = !isPipelineProgressCollapsed;
    setIsPipelineProgressCollapsed(newState);
    setStoredCollapseState('progress', newState);
  };

  // Auto-refresh for file counts
  useAutoRefresh(datasetOps.selectedDataset, [
    inputFilesData.refreshFilesCount,
    outputFilesData.refreshFilesCount
  ]);

  const fileUploadRef = useRef(null);



  // Clear file upload queue when processor type changes
  useEffect(() => {
    if (fileUploadRef.current) {
      fileUploadRef.current.clearAll();
    }
  }, [processorType]);

  const selectedDatasetInfo = datasetOps.selectedDatasetInfo;
  const datasetName = datasetOps.datasetName;

  // Start processing function (wrapper for hook function)
  const handleStartProcessing = async (datasetName) => {
    const result = await datasetOps.startProcessing(datasetName);
    
    if (!result.success && !result.cancelled) {
      alert(`❌ Failed to start processing: ${result.error}`);
    }
  };

  // Process individual file function (wrapper for hook function)
  const handleProcessFile = async (fileName) => {
    const result = await datasetOps.startSingleFileProcessing(fileName);
    
    if (result.success) {
      alert(`✅ Processing started for file "${fileName}"`);
    } else if (!result.cancelled) {
      alert(`❌ Failed to start processing: ${result.error}`);
    }
  };

  // Delete file function (wrapper for hook function)
  const handleDeleteFile = async (fileKey, fileName, isInputFile = true) => {
    const result = isInputFile 
      ? await inputFilesData.deleteFile(fileKey, fileName)
      : await outputFilesData.deleteFile(fileKey, fileName);
    
    if (!result.success && !result.cancelled) {
      alert(`❌ Failed to delete file: ${result.error}`);
    }
  };

  return (
    <div className="pipeline-wrapper">
      {/* File Upload Container */}
      <div style={{ marginBottom: '0.75rem' }}>
        <FileUpload 
          ref={fileUploadRef}
          isCollapsed={isUploadCollapsed} 
          onToggleCollapse={toggleUploadCollapsed} 
          processorType={processorType}
          onDatasetCreated={datasetOps.handleDatasetCreated}
        />
      </div>
      
      {/* Three-pane layout Container */}
      <div className="pipeline-container" style={{ marginBottom: '0.75rem' }}>
        {/* Parent Header for 3-pane section */}
        <div className="pane-header">
          <div className="pane-header-left">
            <button 
              className="collapse-toggle"
              onClick={toggleDatasetsCollapsed}
              title={isDatasetsCollapsed ? "Expand Dataset Management" : "Collapse Dataset Management"}
            >
              <span className={`toggle-icon ${isDatasetsCollapsed ? 'collapsed' : ''}`}>
                {isDatasetsCollapsed ? '▲' : '▼'}
              </span>
            </button>
            <h3>Dataset Management</h3>
          </div>
        </div>
        
        {!isDatasetsCollapsed && (
          <div className="pipeline-panes">
            
            {/* Left Pane - Dataset Selection */}
            <DatasetsPane
              datasetOps={datasetOps}
              inputFiles={inputFilesData.files}
              outputFiles={outputFilesData.files}
              processorType={processorType}
              onDeleteFile={handleDeleteFile}
            />

            {/* Middle Pane - Input Files */}
            <FilesPane
              title={`Input Files - ${datasetName}`}
              filesData={inputFilesData}
              selectedDataset={datasetOps.selectedDataset}
              onDeleteFile={handleDeleteFile}
              onProcessFile={datasetOps.startSingleFileProcessing}
              processorType={processorType}
            />

            {/* Right Pane - Output Files */}
            <FilesPane
              title={`Output Files - ${datasetName}`}
              filesData={outputFilesData}
              selectedDataset={datasetOps.selectedDataset}
              onDeleteFile={handleDeleteFile}
              processorType={processorType}
            />

          </div>
        )}
      </div>
      
      {/* Pipeline Progress Monitor */}
      <PipelineProgress 
        datasets={datasetOps.datasets} 
        inputFiles={inputFilesData.files}
        inputFilesTotalCount={inputFilesData.pagination.total}
        outputFiles={outputFilesData.files}
        outputFilesTotalCount={outputFilesData.pagination.total}
        processorType={processorType}
        isCollapsed={isPipelineProgressCollapsed}
        onToggleCollapse={togglePipelineProgressCollapsed}
      />
      
    </div>
  );
}

DatasetsList.propTypes = {
  processorType: PropTypes.string.isRequired,
};

export default DatasetsList; 
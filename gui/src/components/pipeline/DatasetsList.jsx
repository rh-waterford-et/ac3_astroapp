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
  
  // Consolidated collapse state - simple and clean
  const [collapseState, setCollapseState] = useState({
    upload: false,
    datasets: false,
    progress: false
  });

  // Single parameterized toggle function
  const toggleSection = useCallback((section) => {
    setCollapseState(prev => ({ ...prev, [section]: !prev[section] }));
  }, []);

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
          isCollapsed={collapseState.upload} 
          onToggleCollapse={() => toggleSection('upload')} 
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
              onClick={() => toggleSection('datasets')}
              title={collapseState.datasets ? "Expand Dataset Management" : "Collapse Dataset Management"}
            >
              <span className={`toggle-icon ${collapseState.datasets ? 'collapsed' : ''}`}>
                {collapseState.datasets ? '▲' : '▼'}
              </span>
            </button>
            <h3>Dataset Management</h3>
          </div>
        </div>
        
        {!collapseState.datasets && (
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
        isCollapsed={collapseState.progress}
        onToggleCollapse={() => toggleSection('progress')}
      />
      
    </div>
  );
}

DatasetsList.propTypes = {
  processorType: PropTypes.string.isRequired,
};

export default DatasetsList; 
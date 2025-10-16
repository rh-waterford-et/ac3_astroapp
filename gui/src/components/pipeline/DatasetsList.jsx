import React, { useState, useEffect, useCallback, useRef } from 'react';
import PropTypes from 'prop-types';
import FileUpload from './FileUpload';
import PipelineProgress from './PipelineProgress';

import FilesPane from './FilesPane';
import DatasetsPane from './DatasetsPane';
import { useDatasetOperations } from '../../hooks/data/useDatasetOperations';
import { usePaginatedFiles } from '../../hooks/data/usePaginatedFiles';
import { useAutoRefresh } from '../../hooks/data/useAutoRefresh';
import { useDatasetFileCounts } from '../../hooks/data/useDatasetFileCounts';


function DatasetsList({ processorType }) {
  // Extract data management into hooks
  const datasetOps = useDatasetOperations(processorType);
  const inputFilesData = usePaginatedFiles(datasetOps.selectedDataset, 'input', processorType);
  
  // Output files loading: Load immediately if no inputs exist, otherwise sequence after inputs
  // This allows viewing outputs even when all input files have been processed/deleted
  const shouldLoadOutputs = datasetOps.selectedDataset && (
    // Load immediately if input files loaded and there are none
    (inputFilesData.filesLoaded && 
     inputFilesData.loadedDataset === datasetOps.selectedDataset &&
     inputFilesData.files.length === 0) ||
    // Or load after inputs are loaded and ready (with inputs present)
    (inputFilesData.filesLoaded && 
     inputFilesData.loadedDataset === datasetOps.selectedDataset &&
     inputFilesData.files.length > 0 && 
     !inputFilesData.loading)
  );

  const outputFilesData = usePaginatedFiles(
    shouldLoadOutputs ? datasetOps.selectedDataset : null,
    'output',
    processorType
  );
  
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

  // Get file counts for all datasets (for progress calculation)
  const datasetFileCounts = useDatasetFileCounts(
    datasetOps.datasets,
    datasetOps.selectedDataset,
    processorType
  );

  // Auto-refresh for file counts (respect input→output sequencing)
  const refreshCallbacks = [inputFilesData.refreshFilesCount];
  if (inputFilesData.filesLoaded) {
    refreshCallbacks.push(outputFilesData.refreshFilesCount);
  }
  // Add refresh for dataset counts
  if (datasetFileCounts.refresh) {
    refreshCallbacks.push(datasetFileCounts.refresh);
  }
  
  useAutoRefresh(datasetOps.selectedDataset, refreshCallbacks, 60000); // 60 seconds - standard for file management

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
              processorType={processorType}
            />

            {/* Middle Pane - Input Files */}
            <FilesPane
              title={`Input - ${datasetName}`}
              filesData={inputFilesData}
              selectedDataset={datasetOps.selectedDataset}
              onDeleteFile={handleDeleteFile}
              onProcessFile={datasetOps.startSingleFileProcessing}
              processorType={processorType}
            />

            {/* Right Pane - Output Files */}
            <FilesPane
              title={`Output - ${datasetName}`}
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
        selectedDataset={datasetOps.selectedDataset}
        selectedInputFiles={inputFilesData.files}
        selectedInputCount={inputFilesData.pagination.total}
        selectedOutputFiles={outputFilesData.files}
        selectedOutputCount={outputFilesData.pagination.total}
        datasetFileCounts={datasetFileCounts.counts}
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
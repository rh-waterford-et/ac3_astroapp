import React, { useEffect, useImperativeHandle, forwardRef, useState, useRef } from 'react';
import PropTypes from 'prop-types';
import UploadZone from './upload/UploadZone';
import UploadQueueList from './upload/UploadQueueList';
import DatasetSelector from './upload/DatasetSelector';
import NewDatasetForm from './upload/NewDatasetForm';
import useDatasets from '../hooks/useDatasets';
import useUploadQueue from '../hooks/useUploadQueue';

const FileUpload = forwardRef(({ isCollapsed = false, onToggleCollapse, processorType, onDatasetCreated }, ref) => {
  const {
    availableDatasets,
    currentDataset,
    setCurrentDataset,
    loading: loadingDatasets,
    error: datasetError,
    refresh: refreshDatasets,
    createDataset,
  } = useDatasets(processorType);

  const {
    queue: uploadQueue,
    addFiles,
    remove: removeFile,
    clear: clearAll,
    clearCompleted,
    formatSize,
    totalSize,
    uploadAll,
  } = useUploadQueue(processorType);

  const [showCreateForm, setShowCreateForm] = useState(false);
  const dropdownOpenRef = useRef(false);
  const [ppxfConfig, setPpxfConfig] = React.useState({
    redshift: 0.016571,
    velocityDisp: 200.0,
    waveRangeStart: 5200,
    waveRangeEnd: 6150,
    spsName: 'emiles'
  });
  const [newDatasetName, setNewDatasetName] = React.useState('');

  useEffect(() => {
    const tick = () => {
      if (!dropdownOpenRef.current) refreshDatasets(false, false);
    };
    const id = setInterval(tick, 5000);
    return () => clearInterval(id);
  }, [refreshDatasets]);

  useImperativeHandle(ref, () => ({ clearAll }));

  const handleNewDatasetCreate = async (newName, cfg, _ignored, resetName) => {
    if (!newName?.trim()) return;
    const sanitizedName = newName.trim().replace(/[^a-zA-Z0-9_-]/g, '');
    if (!sanitizedName) return;

    const configToSend = processorType.toLowerCase() === 'ppxf' ? cfg : null;
    const result = await createDataset(sanitizedName, configToSend);
    if (result.success) {
      setCurrentDataset(sanitizedName);
      setShowCreateForm(false);
      resetName('');
      await refreshDatasets(true, false);
      if (onDatasetCreated) onDatasetCreated(sanitizedName);
    }
  };

  const getStatusColor = (status) => {
    switch (status) {
      case 'ready': return '#A0AEC0';
      case 'uploading': return '#4FD1C5';
      case 'completed': return '#68D391';
      case 'error': return '#FC8181';
      default: return '#A0AEC0';
    }
  };

  return (
    <div className="file-upload">
      <div className="pane-header">
        <div className="pane-header-left">
          {onToggleCollapse && (
            <button 
              className="collapse-toggle"
              onClick={onToggleCollapse}
              title={isCollapsed ? 'Expand File Upload' : 'Collapse File Upload'}
            >
              <span className={`toggle-icon ${isCollapsed ? 'collapsed' : ''}`}>
                {isCollapsed ? '▲' : '▼'}
              </span>
            </button>
          )}
          <h3>File Upload</h3>
        </div>
        <div className="upload-actions">
          {!isCollapsed && uploadQueue.length > 0 && (
            <>
              <button 
                className="upload-btn"
                onClick={() => uploadAll(currentDataset)}
                disabled={uploadQueue.every(f => f.status !== 'ready') || !currentDataset}
              >
                Upload All ({uploadQueue.filter(f => f.status === 'ready').length})
              </button>
              <button 
                className="clear-btn"
                onClick={clearCompleted}
                disabled={!uploadQueue.some(f => f.status === 'completed')}
              >
                Clear Completed
              </button>
              <button 
                className="clear-btn clear-all-btn"
                onClick={clearAll}
              >
                Clear All
              </button>
            </>
          )}
        </div>
      </div>
      
      {!isCollapsed && (
        <div className="upload-content">
          <DatasetSelector
            availableDatasets={availableDatasets}
            currentDataset={currentDataset}
            onSelectDataset={setCurrentDataset}
            loadingDatasets={loadingDatasets && availableDatasets.length === 0}
            datasetError={datasetError}
            isCreatingNewDataset={showCreateForm}
            onToggleCreateNew={() => setShowCreateForm(v => !v)}
            onSelectFocus={() => { dropdownOpenRef.current = true; }}
            onSelectBlur={() => { dropdownOpenRef.current = false; }}
          >
            {showCreateForm && (
              <NewDatasetForm
                processorType={processorType}
                ppxfConfig={ppxfConfig}
                setPpxfConfig={setPpxfConfig}
                newDatasetName={newDatasetName}
                setNewDatasetName={setNewDatasetName}
                onCreate={() => handleNewDatasetCreate(newDatasetName, ppxfConfig, () => {}, setNewDatasetName)}
                onCancel={() => { setShowCreateForm(false); setNewDatasetName(''); }}
                loadingDatasets={loadingDatasets}
              />
            )}
          </DatasetSelector>

          <div className="upload-section upload-files-section">
            <div className="section-header">
              <h4>Upload Files</h4>
            </div>
            <UploadZone onFilesSelected={addFiles} />
          </div>

          <div className="upload-section files-list-section">
            <div className="section-header">
              <h4>Files List</h4>
              {uploadQueue.length > 0 && (
                <div className="queue-summary">{uploadQueue.length} files • {formatSize(totalSize())}</div>
              )}
            </div>
            <UploadQueueList 
              uploadQueue={uploadQueue}
              onRemove={removeFile}
              getStatusColor={getStatusColor}
            />
          </div>
        </div>
      )}
    </div>
  );
});

FileUpload.propTypes = {
  isCollapsed: PropTypes.bool,
  onToggleCollapse: PropTypes.func,
  processorType: PropTypes.string.isRequired,
  onDatasetCreated: PropTypes.func,
};

export default FileUpload; 
import React from 'react';
import PropTypes from 'prop-types';
import DatasetItem from './DatasetItem';

const DatasetsPane = ({
  datasetOps,
  inputFiles,
  outputFiles,
  processorType,
  onDeleteFile
}) => {
  const { datasets, loading, error, refresh, selectedDataset, setSelectedDataset, startProcessing, deleteDataset } = datasetOps;

  return (
    <div className="pipeline-pane datasets-pane">
      <div className="pane-header">
        <div className="pane-header-left">
          <h3>Datasets</h3>
        </div>
        <div className="pane-header-right">
          <button 
            className="refresh-btn" 
            onClick={refresh}
            disabled={loading}
            title="Refresh datasets"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M23 4v6h-6"/>
              <path d="M1 20v-6h6"/>
              <path d="m3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
            </svg>
          </button>
          <div className="pane-count">{datasets.length}</div>
        </div>
      </div>
      <div className="pane-content">
        {loading ? (
          <div className="astro-loading-compact" style={{ minHeight: '150px' }}>
            <div className="astro-loader-galaxy" style={{ width: '20px', height: '20px' }}></div>
            <div className="astro-loading-text" style={{ fontSize: '12px' }}>Loading datasets...</div>
          </div>
        ) : error ? (
          <div className="empty-pane">
            <div className="empty-icon">❌</div>
            <p>Error loading datasets</p>
            <p style={{ fontSize: '12px', color: '#FF6B6B' }}>{error}</p>
          </div>
        ) : datasets.length > 0 ? (
          datasets.map(dataset => (
            <DatasetItem
              key={dataset.id}
              dataset={dataset}
              isSelected={selectedDataset === dataset.id}
              onSelect={setSelectedDataset}
              onStartProcessing={startProcessing}
              onDelete={deleteDataset}
              inputFiles={inputFiles}
              outputFiles={outputFiles}
              processorType={processorType}
            />
          ))
        ) : (
          <div className="empty-pane">
            <div className="empty-icon">📊</div>
            <p>No datasets found</p>
          </div>
        )}
      </div>
    </div>
  );
};

DatasetsPane.propTypes = {
  datasetOps: PropTypes.object.isRequired,
  inputFiles: PropTypes.array.isRequired,
  outputFiles: PropTypes.array.isRequired,
  processorType: PropTypes.string.isRequired,
  onDeleteFile: PropTypes.func.isRequired
};

export default DatasetsPane; 
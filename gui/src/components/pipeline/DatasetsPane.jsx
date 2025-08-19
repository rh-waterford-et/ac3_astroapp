import React from 'react';
import PropTypes from 'prop-types';
import VirtualizedFileList from './VirtualizedFileList';
import RefreshIcon from '../ui/RefreshIcon';

const DatasetsPane = ({
  datasetOps,
  processorType
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
            <RefreshIcon />
          </button>
          <div className="pane-count">{datasets.length}</div>
        </div>
      </div>
      <div className="pane-content">
        <VirtualizedFileList
          items={datasets}
          isDatasetMode={true}
          selectedDatasetId={selectedDataset}
          onSelectDataset={setSelectedDataset}
          onStartProcessing={startProcessing}
          onDelete={deleteDataset}
          itemHeight={48}
          emptyMessage="No datasets found"
          emptyIcon="📊"
          isLoading={loading}
          error={error}
          hasNextPage={false}
          loadingMessage="Loading datasets..."
          processorType={processorType}
        />
      </div>
    </div>
  );
};

DatasetsPane.propTypes = {
  datasetOps: PropTypes.object.isRequired,
  processorType: PropTypes.string.isRequired
};

export default DatasetsPane; 
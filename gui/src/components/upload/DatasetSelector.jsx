import React from 'react';
import PropTypes from 'prop-types';

function DatasetSelector({ 
  availableDatasets = [],
  currentDataset = '',
  onSelectDataset,
  loadingDatasets = false,
  datasetError = null,
  isCreatingNewDataset = false,
  onToggleCreateNew,
  onSelectFocus,
  onSelectBlur,
  children
}) {
  return (
    <div className="upload-section dataset-section">
      <div className="section-header">
        <h4>Select Dataset</h4>
        {loadingDatasets && (
          <div className="astro-loading-compact">
            <div className="astro-loader-galaxy" style={{ width: '18px', height: '18px' }}></div>
            <div className="astro-loading-text" style={{ fontSize: '11px' }}>Loading datasets...</div>
          </div>
        )}
        {datasetError && (
          <div className="astro-loading-compact">
            <div className="astro-loader-galaxy" style={{ width: '18px', height: '18px' }}></div>
            <div className="astro-loading-text" style={{ fontSize: '11px' }}>Loading datasets...</div>
          </div>
        )}
      </div>

      <div className="dataset-selection">
        <div className="dataset-select-wrapper">
          <select 
            className="dataset-select"
            value={currentDataset}
            onChange={(e) => onSelectDataset?.(e.target.value)}
            onFocus={onSelectFocus}
            onBlur={onSelectBlur}
            disabled={loadingDatasets}
          >
            <option value="">-- Select Dataset --</option>
            {availableDatasets.map(dataset => (
              <option key={dataset} value={dataset}>{dataset}</option>
            ))}
          </select>
        </div>

        {!isCreatingNewDataset && (
          <div className="create-dataset-section">
            <button 
              className="create-dataset-toggle-btn"
              onClick={onToggleCreateNew}
              disabled={loadingDatasets}
            >
              Create Dataset
            </button>
          </div>
        )}

        {isCreatingNewDataset && (
          <>
            <div className="dataset-form-divider"></div>
            {children}
          </>
        )}
      </div>
    </div>
  );
}

DatasetSelector.propTypes = {
  availableDatasets: PropTypes.array,
  currentDataset: PropTypes.string,
  onSelectDataset: PropTypes.func,
  loadingDatasets: PropTypes.bool,
  datasetError: PropTypes.string,
  isCreatingNewDataset: PropTypes.bool,
  onToggleCreateNew: PropTypes.func,
  onSelectFocus: PropTypes.func,
  onSelectBlur: PropTypes.func,
  children: PropTypes.node,
};

export default DatasetSelector; 
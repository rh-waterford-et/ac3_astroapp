import React, { useCallback } from 'react';
import PropTypes from 'prop-types';
import PPXFConfigForm from './PPXFConfigForm';
import VoronoiConfigForm from './VoronoiConfigForm';

function NewDatasetForm({
  processorType,
  ppxfConfig,
  setPpxfConfig,
  voronoiConfig,
  setVoronoiConfig,
  newDatasetName,
  setNewDatasetName,
  onCreate,
  onCancel,
  loadingDatasets = false,
}) {
  const isPPXF = (processorType || '').toLowerCase() === 'ppxf';
  const isVoronoi = (processorType || '').toLowerCase() === 'voronoi';

  const handleKeyDown = useCallback((e) => {
    if (e.key === 'Enter') {
      onCreate?.();
    } else if (e.key === 'Escape') {
      onCancel?.();
    }
  }, [onCreate, onCancel]);

  return (
    <div className="new-dataset-form">
      <div className="new-dataset-input-group">
        <input
          type="text"
          className="new-dataset-input"
          placeholder="Dataset Name (eg: NGC7025)"
          value={newDatasetName}
          onChange={(e) => setNewDatasetName(e.target.value)}
          onKeyDown={handleKeyDown}
        />
      </div>

      {isPPXF && (
        <PPXFConfigForm 
          config={ppxfConfig} 
          setConfig={setPpxfConfig} 
        />
      )}

      {isVoronoi && (
        <VoronoiConfigForm 
          config={voronoiConfig} 
          setConfig={setVoronoiConfig} 
        />
      )}

      <div className="dataset-form-actions">
        <button 
          className="create-dataset-btn"
          onClick={onCreate}
          disabled={!newDatasetName.trim() || loadingDatasets}
        >
          Create
        </button>
        <button 
          className="create-dataset-toggle-btn cancel-variant"
          onClick={onCancel}
          disabled={loadingDatasets}
        >
          Cancel
        </button>
      </div>
    </div>
  );
}

NewDatasetForm.propTypes = {
  processorType: PropTypes.string,
  ppxfConfig: PropTypes.shape({
    redshift: PropTypes.number.isRequired,
    velocityDisp: PropTypes.number.isRequired,
    waveRangeStart: PropTypes.number.isRequired,
    waveRangeEnd: PropTypes.number.isRequired,
    spsName: PropTypes.string.isRequired,
  }),
  setPpxfConfig: PropTypes.func,
  voronoiConfig: PropTypes.shape({
    instrument: PropTypes.string.isRequired,
    targetSN: PropTypes.number.isRequired,
    redshift: PropTypes.number.isRequired,
    wavelengthStart: PropTypes.number.isRequired,
    wavelengthEnd: PropTypes.number.isRequired,
    snMethod: PropTypes.string.isRequired,
    knotsNumber: PropTypes.number.isRequired,
    minSN: PropTypes.number.isRequired,
    generateIndividualSpectra: PropTypes.bool.isRequired,
  }),
  setVoronoiConfig: PropTypes.func,
  newDatasetName: PropTypes.string.isRequired,
  setNewDatasetName: PropTypes.func.isRequired,
  onCreate: PropTypes.func.isRequired,
  onCancel: PropTypes.func.isRequired,
  loadingDatasets: PropTypes.bool,
};

export default NewDatasetForm; 
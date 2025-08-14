import React from 'react';
import PropTypes from 'prop-types';

function NewDatasetForm({
  processorType,
  ppxfConfig,
  setPpxfConfig,
  newDatasetName,
  setNewDatasetName,
  onCreate,
  onCancel,
  loadingDatasets = false,
}) {
  const isPPXF = (processorType || '').toLowerCase() === 'ppxf';

  return (
    <div className="new-dataset-form">
      <div className="new-dataset-input-group">
        <input
          type="text"
          className="new-dataset-input"
          placeholder="Dataset Name (eg: NGC7025)"
          value={newDatasetName}
          onChange={(e) => setNewDatasetName(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              onCreate?.();
            } else if (e.key === 'Escape') {
              onCancel?.();
            }
          }}
        />
      </div>

      {isPPXF && (
        <div className="ppxf-config-section">
          <div className="ppxf-config-header">
            <h5>pPXF Configuration</h5>
          </div>
          <div className="ppxf-config-form">
            <div className="ppxf-config-row new-dataset-input-group">
              <div className="ppxf-config-field" style={{ flex: '1 1 0', minWidth: 0 }}>
                <label>Redshift:</label>
                <input
                  type="number"
                  step="0.000001"
                  className="new-dataset-input"
                  value={ppxfConfig.redshift}
                  onChange={(e) => setPpxfConfig({ ...ppxfConfig, redshift: parseFloat(e.target.value) || 0 })}
                />
              </div>
              <div className="ppxf-config-field" style={{ flex: '1 1 0', minWidth: 0 }}>
                <label>Velocity Dispersion (km/s):</label>
                <input
                  type="number"
                  step="0.1"
                  className="new-dataset-input"
                  value={ppxfConfig.velocityDisp}
                  onChange={(e) => setPpxfConfig({ ...ppxfConfig, velocityDisp: parseFloat(e.target.value) || 0 })}
                />
              </div>
            </div>
            <div className="ppxf-config-row new-dataset-input-group">
              <div className="ppxf-config-field" style={{ flex: '1 1 0', minWidth: 0 }}>
                <label>Wave Start (Å):</label>
                <input
                  type="number"
                  className="new-dataset-input"
                  value={ppxfConfig.waveRangeStart}
                  onChange={(e) => setPpxfConfig({ ...ppxfConfig, waveRangeStart: parseInt(e.target.value) || 0 })}
                />
              </div>
              <div className="ppxf-config-field" style={{ flex: '1 1 0', minWidth: 0 }}>
                <label>Wave End (Å):</label>
                <input
                  type="number"
                  className="new-dataset-input"
                  value={ppxfConfig.waveRangeEnd}
                  onChange={(e) => setPpxfConfig({ ...ppxfConfig, waveRangeEnd: parseInt(e.target.value) || 0 })}
                />
              </div>
            </div>
            <div className="ppxf-config-row new-dataset-input-group">
              <div className="ppxf-config-field full-width" style={{ flex: '1 1 100%', minWidth: 0 }}>
                <label>SPS Model:</label>
                <select
                  className="new-dataset-input"
                  value={ppxfConfig.spsName}
                  onChange={(e) => setPpxfConfig({ ...ppxfConfig, spsName: e.target.value })}
                >
                  <option value="emiles">EMILES</option>
                  <option value="fsps">FSPS</option>
                  <option value="galaxev">GALAXEV</option>
                  <option value="coelho">Coelho</option>
                  <option value="coelho_mini">Coelho Mini</option>
                </select>
              </div>
            </div>
          </div>
        </div>
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
  }).isRequired,
  setPpxfConfig: PropTypes.func.isRequired,
  newDatasetName: PropTypes.string.isRequired,
  setNewDatasetName: PropTypes.func.isRequired,
  onCreate: PropTypes.func.isRequired,
  onCancel: PropTypes.func.isRequired,
  loadingDatasets: PropTypes.bool,
};

export default NewDatasetForm; 
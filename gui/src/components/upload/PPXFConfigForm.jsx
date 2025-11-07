import React, { useCallback } from 'react';
import PropTypes from 'prop-types';

function PPXFConfigForm({ config, setConfig }) {
  // Memoized handlers to prevent object recreation on every keystroke
  const handleRedshiftChange = useCallback((e) => {
    const value = parseFloat(e.target.value) || 0;
    setConfig(prev => ({ ...prev, redshift: value }));
  }, [setConfig]);

  const handleVelocityDispChange = useCallback((e) => {
    const value = parseFloat(e.target.value) || 0;
    setConfig(prev => ({ ...prev, velocityDisp: value }));
  }, [setConfig]);

  const handleWaveStartChange = useCallback((e) => {
    const value = parseFloat(e.target.value) || 0;
    setConfig(prev => ({ ...prev, waveRangeStart: value }));
  }, [setConfig]);

  const handleWaveEndChange = useCallback((e) => {
    const value = parseFloat(e.target.value) || 0;
    setConfig(prev => ({ ...prev, waveRangeEnd: value }));
  }, [setConfig]);

  const handleSpsNameChange = useCallback((e) => {
    setConfig(prev => ({ ...prev, spsName: e.target.value }));
  }, [setConfig]);

  return (
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
              value={config.redshift}
              onChange={handleRedshiftChange}
            />
          </div>
          <div className="ppxf-config-field" style={{ flex: '1 1 0', minWidth: 0 }}>
            <label>Velocity Dispersion (km/s):</label>
            <input
              type="number"
              step="0.1"
              className="new-dataset-input"
              value={config.velocityDisp}
              onChange={handleVelocityDispChange}
            />
          </div>
        </div>
        <div className="ppxf-config-row new-dataset-input-group">
          <div className="ppxf-config-field" style={{ flex: '1 1 0', minWidth: 0 }}>
            <label>Wave Start (Å):</label>
            <input
              type="number"
              className="new-dataset-input"
              value={config.waveRangeStart}
              onChange={handleWaveStartChange}
            />
          </div>
          <div className="ppxf-config-field" style={{ flex: '1 1 0', minWidth: 0 }}>
            <label>Wave End (Å):</label>
            <input
              type="number"
              className="new-dataset-input"
              value={config.waveRangeEnd}
              onChange={handleWaveEndChange}
            />
          </div>
        </div>
        <div className="ppxf-config-row new-dataset-input-group">
          <div className="ppxf-config-field full-width" style={{ flex: '1 1 100%', minWidth: 0 }}>
            <label>SPS Model:</label>
            <select
              className="new-dataset-input"
              value={config.spsName}
              onChange={handleSpsNameChange}
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
  );
}

PPXFConfigForm.propTypes = {
  config: PropTypes.shape({
    redshift: PropTypes.number.isRequired,
    velocityDisp: PropTypes.number.isRequired,
    waveRangeStart: PropTypes.number.isRequired,
    waveRangeEnd: PropTypes.number.isRequired,
    spsName: PropTypes.string.isRequired,
  }).isRequired,
  setConfig: PropTypes.func.isRequired,
};

export default PPXFConfigForm;


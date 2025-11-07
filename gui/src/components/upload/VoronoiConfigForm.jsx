import React, { useCallback } from 'react';
import PropTypes from 'prop-types';

function VoronoiConfigForm({ config, setConfig }) {
  // Memoized handlers to prevent object recreation on every keystroke
  const handleInstrumentChange = useCallback((e) => {
    setConfig(prev => ({ ...prev, instrument: e.target.value }));
  }, [setConfig]);

  const handleTargetSNChange = useCallback((e) => {
    const value = parseFloat(e.target.value) || 0;
    setConfig(prev => ({ ...prev, targetSN: value }));
  }, [setConfig]);

  const handleRedshiftChange = useCallback((e) => {
    const value = parseFloat(e.target.value) || 0;
    setConfig(prev => ({ ...prev, redshift: value }));
  }, [setConfig]);

  const handleWavelengthStartChange = useCallback((e) => {
    const value = parseFloat(e.target.value) || 0;
    setConfig(prev => ({ ...prev, wavelengthStart: value }));
  }, [setConfig]);

  const handleWavelengthEndChange = useCallback((e) => {
    const value = parseFloat(e.target.value) || 0;
    setConfig(prev => ({ ...prev, wavelengthEnd: value }));
  }, [setConfig]);

  const handleSNMethodChange = useCallback((e) => {
    setConfig(prev => ({ ...prev, snMethod: e.target.value }));
  }, [setConfig]);

  const handleKnotsNumberChange = useCallback((e) => {
    const value = parseInt(e.target.value) || 0;
    setConfig(prev => ({ ...prev, knotsNumber: value }));
  }, [setConfig]);

  const handleMinSNChange = useCallback((e) => {
    const value = parseFloat(e.target.value) || 0;
    setConfig(prev => ({ ...prev, minSN: value }));
  }, [setConfig]);

  const handleGenerateSpectraChange = useCallback((e) => {
    setConfig(prev => ({ ...prev, generateIndividualSpectra: e.target.checked }));
  }, [setConfig]);

  return (
    <div className="ppxf-config-section">
      <div className="ppxf-config-header">
        <h5>Voronoi Binning Configuration</h5>
      </div>
      <div className="ppxf-config-form">
        {/* Row 1: Instrument */}
        <div className="ppxf-config-row new-dataset-input-group">
          <div className="ppxf-config-field full-width" style={{ flex: '1 1 100%', minWidth: 0 }}>
            <label>Instrument:</label>
            <select
              className="new-dataset-input"
              value={config.instrument}
              onChange={handleInstrumentChange}
              required
            >
              <option value="megara">MEGARA</option>
              <option value="manga">MaNGA</option>
              <option value="muse">MUSE</option>
            </select>
          </div>
        </div>

        {/* Row 2: Target S/N and Redshift */}
        <div className="ppxf-config-row new-dataset-input-group">
          <div className="ppxf-config-field" style={{ flex: '1 1 0', minWidth: 0 }}>
            <label>Target S/N:</label>
            <input
              type="number"
              step="1"
              className="new-dataset-input"
              value={config.targetSN}
              onChange={handleTargetSNChange}
              placeholder="30"
              required
            />
          </div>
          <div className="ppxf-config-field" style={{ flex: '1 1 0', minWidth: 0 }}>
            <label>Redshift:</label>
            <input
              type="number"
              step="0.00001"
              className="new-dataset-input"
              value={config.redshift}
              onChange={handleRedshiftChange}
              placeholder="0.01657"
              required
            />
          </div>
        </div>

        {/* Row 3: Wavelength Range */}
        <div className="ppxf-config-row new-dataset-input-group">
          <div className="ppxf-config-field" style={{ flex: '1 1 0', minWidth: 0 }}>
            <label>Wavelength Start (Å):</label>
            <input
              type="number"
              className="new-dataset-input"
              value={config.wavelengthStart}
              onChange={handleWavelengthStartChange}
              placeholder="5600"
              required
            />
          </div>
          <div className="ppxf-config-field" style={{ flex: '1 1 0', minWidth: 0 }}>
            <label>Wavelength End (Å):</label>
            <input
              type="number"
              className="new-dataset-input"
              value={config.wavelengthEnd}
              onChange={handleWavelengthEndChange}
              placeholder="5800"
              required
            />
          </div>
        </div>

        {/* Row 4: S/N Method */}
        <div className="ppxf-config-row new-dataset-input-group">
          <div className="ppxf-config-field full-width" style={{ flex: '1 1 100%', minWidth: 0 }}>
            <label>S/N Method:</label>
            <select
              className="new-dataset-input"
              value={config.snMethod}
              onChange={handleSNMethodChange}
              required
            >
              <option value="spline">Spline</option>
              <option value="brightest_spaxel">Brightest Spaxel</option>
              <option value="signal_square_root">Signal Square Root</option>
            </select>
          </div>
        </div>

        {/* Row 5: Knots Number (when spline) and Minimum S/N on same row */}
        <div className="ppxf-config-row new-dataset-input-group">
          {config.snMethod === 'spline' && (
            <div className="ppxf-config-field" style={{ flex: '1 1 0', minWidth: 0 }}>
              <label>Knots Number:</label>
              <input
                type="number"
                step="1"
                className="new-dataset-input"
                value={config.knotsNumber}
                onChange={handleKnotsNumberChange}
                placeholder="40"
              />
            </div>
          )}
          <div className="ppxf-config-field" style={{ flex: '1 1 0', minWidth: 0 }}>
            <label>Minimum S/N:</label>
            <input
              type="number"
              step="0.1"
              className="new-dataset-input"
              value={config.minSN}
              onChange={handleMinSNChange}
              placeholder="1"
              required
            />
          </div>
        </div>

        {/* Row 6: Generate Individual Spectra Checkbox */}
        <div className="ppxf-config-row new-dataset-input-group">
          <div className="ppxf-config-field full-width" style={{ flex: '1 1 100%', minWidth: 0 }}>
            <label className="voronoi-checkbox-label">
              <input
                type="checkbox"
                checked={config.generateIndividualSpectra}
                onChange={handleGenerateSpectraChange}
                className="voronoi-checkbox-input"
              />
              <span className="voronoi-checkmark"></span>
              <span className="voronoi-checkbox-text">Generate Individual Spectra</span>
            </label>
          </div>
        </div>
      </div>
    </div>
  );
}

VoronoiConfigForm.propTypes = {
  config: PropTypes.shape({
    instrument: PropTypes.string.isRequired,
    targetSN: PropTypes.number.isRequired,
    redshift: PropTypes.number.isRequired,
    wavelengthStart: PropTypes.number.isRequired,
    wavelengthEnd: PropTypes.number.isRequired,
    snMethod: PropTypes.string.isRequired,
    knotsNumber: PropTypes.number.isRequired,
    minSN: PropTypes.number.isRequired,
    generateIndividualSpectra: PropTypes.bool.isRequired,
  }).isRequired,
  setConfig: PropTypes.func.isRequired,
};

export default VoronoiConfigForm;


import React, { useEffect } from 'react';
import PropTypes from 'prop-types';

const Sidebar = ({ aladinInstance, checkboxStates, onCheckboxChange }) => {
  useEffect(() => {
    if (!aladinInstance) return;
    
    setupSidebarControls(aladinInstance, checkboxStates, onCheckboxChange);
  }, [aladinInstance, checkboxStates, onCheckboxChange]);

  return (
    <div className="right-sidebar">

      
      <div className="sidebar-content">
        {/* Available Maps/Kinematics */}
        <div className="control-section">
          <h4>Available Maps</h4>
          <div className="subsection">
            <h5>Kinematics</h5>
            <div className="checkbox-list">
              <label className="checkbox-item">
                <input 
                  type="checkbox" 
                  id="map-stellar-velocity" 
                  checked={checkboxStates['map-stellar-velocity']}
                  onChange={(e) => onCheckboxChange('map-stellar-velocity', e.target.checked)}
                />
                <span className="checkmark"></span>
                <span> </span>Stellar velocity
              </label>
              <label className="checkbox-item">
                <input 
                  type="checkbox" 
                  id="map-stellar-velocity-error" 
                  checked={checkboxStates['map-stellar-velocity-error']}
                  onChange={(e) => onCheckboxChange('map-stellar-velocity-error', e.target.checked)}
                />
                <span className="checkmark"></span>
                <span> </span>Stellar velocity Error
              </label>
              <label className="checkbox-item">
                <input 
                  type="checkbox" 
                  id="map-velocity-dispersion" 
                  checked={checkboxStates['map-velocity-dispersion']}
                  onChange={(e) => onCheckboxChange('map-velocity-dispersion', e.target.checked)}
                />
                <span className="checkmark"></span>
                <span> </span>Velocity dispersion
              </label>
              <label className="checkbox-item">
                <input 
                  type="checkbox" 
                  id="map-velocity-dispersion-error" 
                  checked={checkboxStates['map-velocity-dispersion-error']}
                  onChange={(e) => onCheckboxChange('map-velocity-dispersion-error', e.target.checked)}
                />
                <span className="checkmark"></span>
                <span> </span>Velocity dispersion Error
              </label>
              <label className="checkbox-item">
                <input 
                  type="checkbox" 
                  id="map-h3" 
                  checked={checkboxStates['map-h3']}
                  onChange={(e) => onCheckboxChange('map-h3', e.target.checked)}
                />
                <span className="checkmark"></span>
                <span> </span>h3
              </label>
              <label className="checkbox-item">
                <input 
                  type="checkbox" 
                  id="map-h4" 
                  checked={checkboxStates['map-h4']}
                  onChange={(e) => onCheckboxChange('map-h4', e.target.checked)}
                />
                <span className="checkmark"></span>
                <span> </span>h4
              </label>
            </div>
          </div>
          
          <div className="subsection">
            <h5>Stellar Populations</h5>
            <div className="checkbox-list">
              <label className="checkbox-item">
                <input 
                  type="checkbox" 
                  id="map-age-weighted" 
                  checked={checkboxStates['map-age-weighted']}
                  onChange={(e) => onCheckboxChange('map-age-weighted', e.target.checked)}
                />
                <span className="checkmark"></span>
                <span> </span>Age (lum. weighted)
              </label>
              <label className="checkbox-item">
                <input 
                  type="checkbox" 
                  id="map-age-mass-weighted" 
                  checked={checkboxStates['map-age-mass-weighted']}
                  onChange={(e) => onCheckboxChange('map-age-mass-weighted', e.target.checked)}
                />
                <span className="checkmark"></span>
                <span> </span>Age (Mass Weighted)
              </label>
              <label className="checkbox-item">
                <input 
                  type="checkbox" 
                  id="map-metallicity" 
                  checked={checkboxStates['map-metallicity']}
                  onChange={(e) => onCheckboxChange('map-metallicity', e.target.checked)}
                />
                <span className="checkmark"></span>
                <span> </span>Metallicity
              </label>
            </div>
          </div>
        </div>

        {/* Display Options */}
        <div className="control-section">
          <h4>Display Options</h4>
          <div className="checkbox-list">
            <label className="checkbox-item">
              <input 
                type="checkbox" 
                id="display-grid" 
                checked={checkboxStates['display-grid']}
                onChange={(e) => onCheckboxChange('display-grid', e.target.checked)}
              />
              <span className="checkmark"></span>
              <span> </span>Coordinate Grid
            </label>
            <label className="checkbox-item">
              <input 
                type="checkbox" 
                id="display-reticle" 
                checked={checkboxStates['display-reticle']}
                onChange={(e) => onCheckboxChange('display-reticle', e.target.checked)}
              />
              <span className="checkmark"></span>
              <span> </span>Center Reticle
            </label>
            <label className="checkbox-item">
              <input 
                type="checkbox" 
                id="display-labels" 
                checked={checkboxStates['display-labels']}
                onChange={(e) => onCheckboxChange('display-labels', e.target.checked)}
              />
              <span className="checkmark"></span>
              <span> </span>Object Labels
            </label>
            <label className="checkbox-item">
              <input 
                type="checkbox" 
                id="display-healpix" 
                checked={checkboxStates['display-healpix']}
                onChange={(e) => onCheckboxChange('display-healpix', e.target.checked)}
              />
              <span className="checkmark"></span>
              <span> </span>HEALPix Grid
            </label>
          </div>
        </div>


      </div>
    </div>
  );
};

/**
 * Set up sidebar controls functionality
 * @param {Object} aladin - The Aladin Lite instance
 * @param {Object} checkboxStates - Current checkbox states
 * @param {Function} onCheckboxChange - Callback for checkbox changes
 */
const setupSidebarControls = (aladin, checkboxStates, onCheckboxChange) => {
  // Apply current display states to Aladin when component mounts
  if (checkboxStates['display-grid']) {
    try {
      aladin.showCooGrid();
      console.log('Coordinate grid enabled (on mount)');
    } catch (error) {
      console.error('Grid control error:', error);
    }
  }
  
  if (checkboxStates['display-reticle']) {
    try {
      aladin.showReticle(true);
      console.log('Reticle enabled (on mount)');
    } catch (error) {
      console.error('Reticle control error:', error);
    }
  }
  
  if (checkboxStates['display-labels']) {
    try {
      const catalogs = aladin.getCatalogs();
      catalogs.forEach(catalog => {
        if (catalog.setShowLabels) {
          catalog.setShowLabels(true);
        }
      });
      console.log('Object labels enabled (on mount)');
    } catch (error) {
      console.log('Label display control not available:', error);
    }
  }

  if (checkboxStates['display-healpix']) {
    try {
      aladin.showHealpixGrid(true);
      console.log('HEALPix grid enabled (on mount)');
    } catch (error) {
      console.error('HEALPix grid control error:', error);
    }
  }

  // Note: Gallery manipulation is handled by App component's handleCheckboxChange
  // to prevent duplicate images when switching tabs
  console.log('Sidebar controls set up with preserved state');
};

Sidebar.propTypes = {
  aladinInstance: PropTypes.object,
  checkboxStates: PropTypes.object.isRequired,
  onCheckboxChange: PropTypes.func.isRequired,
};

export default Sidebar; 
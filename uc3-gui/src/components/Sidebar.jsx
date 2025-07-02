import React, { useEffect } from 'react';

const Sidebar = ({ aladinInstance }) => {
  useEffect(() => {
    if (!aladinInstance) return;
    
    setupSidebarControls(aladinInstance);
  }, [aladinInstance]);

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
                <input type="checkbox" id="map-stellar-velocity" />
                <span className="checkmark"></span>
                Stellar velocity
              </label>
              <label className="checkbox-item">
                <input type="checkbox" id="map-stellar-velocity-error" />
                <span className="checkmark"></span>
                Stellar velocity Error
              </label>
              <label className="checkbox-item">
                <input type="checkbox" id="map-velocity-dispersion" />
                <span className="checkmark"></span>
                Velocity dispersion
              </label>
              <label className="checkbox-item">
                <input type="checkbox" id="map-velocity-dispersion-error" />
                <span className="checkmark"></span>
                Velocity dispersion Error
              </label>
              <label className="checkbox-item">
                <input type="checkbox" id="map-h3" />
                <span className="checkmark"></span>
                h3
              </label>
              <label className="checkbox-item">
                <input type="checkbox" id="map-h4" />
                <span className="checkmark"></span>
                h4
              </label>
            </div>
          </div>
          
          <div className="subsection">
            <h5>Stellar Populations</h5>
            <div className="checkbox-list">
              <label className="checkbox-item">
                <input type="checkbox" id="map-age-weighted" />
                <span className="checkmark"></span>
                Age (lum. weighted)
              </label>
              <label className="checkbox-item">
                <input type="checkbox" id="map-age-mass-weighted" />
                <span className="checkmark"></span>
                Age (Mass Weighted)
              </label>
              <label className="checkbox-item">
                <input type="checkbox" id="map-metallicity" />
                <span className="checkmark"></span>
                Metallicity
              </label>
            </div>
          </div>
        </div>

        {/* Display Options */}
        <div className="control-section">
          <h4>Display Options</h4>
          <div className="checkbox-list">
            <label className="checkbox-item">
              <input type="checkbox" id="display-grid" />
              <span className="checkmark"></span>
              Coordinate Grid
            </label>
            <label className="checkbox-item">
              <input type="checkbox" id="display-reticle" defaultChecked />
              <span className="checkmark"></span>
              Center Reticle
            </label>
            <label className="checkbox-item">
              <input type="checkbox" id="display-labels" />
              <span className="checkmark"></span>
              Object Labels
            </label>
            <label className="checkbox-item">
              <input type="checkbox" id="display-healpix" />
              <span className="checkmark"></span>
              HEALPix Grid
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
 */
const setupSidebarControls = (aladin) => {

  // Display options - using correct Aladin v3 API methods
  const displayControls = {
    'display-grid': () => {
      const checkbox = document.getElementById('display-grid');
      const isChecked = checkbox.checked;
      
      try {
        if (isChecked) {
          aladin.showCooGrid();
          console.log('Coordinate grid enabled');
        } else {
          aladin.hideCooGrid();
          console.log('Coordinate grid disabled');
        }
      } catch (error) {
        console.error('Grid control error:', error);
      }
    },
    
    'display-reticle': () => {
      const checkbox = document.getElementById('display-reticle');
      const isChecked = checkbox.checked;
      
      try {
        aladin.showReticle(isChecked);
        console.log(`Reticle ${isChecked ? 'enabled' : 'disabled'}`);
      } catch (error) {
        console.error('Reticle control error:', error);
      }
    },
    'display-labels': () => {
      const checkbox = document.getElementById('display-labels');
      console.log(`Object labels ${checkbox.checked ? 'enabled' : 'disabled'}`);
      // This would require catalog management - for now just log
      try {
        // Try to toggle catalog labels if any catalogs are loaded
        const catalogs = aladin.getCatalogs();
        catalogs.forEach(catalog => {
          if (catalog.setShowLabels) {
            catalog.setShowLabels(checkbox.checked);
          }
        });
      } catch (error) {
        console.log('Label display control not available:', error);
      }
    },

    'display-healpix': () => {
      const checkbox = document.getElementById('display-healpix');
      const isChecked = checkbox.checked;
      
      try {
        aladin.showHealpixGrid(isChecked);
        console.log(`HEALPix grid ${isChecked ? 'enabled' : 'disabled'}`);
      } catch (error) {
        console.error('HEALPix grid control error:', error);
      }
    }
  };

  Object.entries(displayControls).forEach(([checkboxId, handler]) => {
    const checkbox = document.getElementById(checkboxId);
    if (checkbox) {
      checkbox.addEventListener('change', handler);
    }
  });

  // Available Maps controls - connect to gallery
  const mapControls = {
    'map-stellar-velocity': { label: 'Stellar velocity', icon: '🌀' },
    'map-velocity-dispersion': { label: 'Velocity dispersion', icon: '📊' },
    'map-stellar-velocity-error': { label: 'Stellar velocity Error', icon: '⚠️' },
    'map-velocity-dispersion-error': { label: 'Velocity dispersion Error', icon: '📈' },
    'map-h3': { label: 'h3', icon: 'H₃' },
    'map-h4': { label: 'h4', icon: 'H₄' },
    'map-age-weighted': { label: 'Age (lum. weighted)', icon: '⏳' },
    'map-age-mass-weighted': { label: 'Age (Mass Weighted)', icon: '⚖️' },
    'map-metallicity': { label: 'Metallicity', icon: '⚛️' }
  };

  Object.entries(mapControls).forEach(([checkboxId, config]) => {
    const checkbox = document.getElementById(checkboxId);
    if (checkbox) {
      checkbox.addEventListener('change', (event) => {
        // Check if we have a current object loaded
        const currentObject = window.currentLoadedObject;
        
        console.log(`📋 Checkbox ${checkboxId} changed to ${event.target.checked}, currentObject: ${currentObject}`);
        
        if (currentObject) {
          // If an object is loaded, reload its images based on current selections
          console.log(`🔄 Reloading images for ${currentObject} due to ${config.label} change`);
          if (window.loadObjectImages) {
            window.loadObjectImages(currentObject);
          }
        } else {
          // No object loaded, use placeholder system
          const mapType = checkboxId.replace('map-', '');
          
          if (event.target.checked) {
            // Add to gallery
            if (window.addMapToGallery) {
              window.addMapToGallery(mapType, config.label, config.icon);
            }
            console.log(`Added ${config.label} to gallery`);
          } else {
            // Remove from gallery
            if (window.removeMapFromGallery) {
              window.removeMapFromGallery(mapType);
            }
            console.log(`Removed ${config.label} from gallery`);
          }
        }
      });
    }
  });

  console.log('Sidebar controls set up');
};

export default Sidebar; 
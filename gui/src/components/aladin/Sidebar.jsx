import React, { useEffect } from 'react';
import PropTypes from 'prop-types';
import { MAP_CHECKBOX_IDS, KINEMATICS_CHECKBOXES, POPULATION_CHECKBOXES, DISPLAY_CHECKBOXES } from '../../utils/constants/constants';
import { useGallery } from '../../contexts/GalleryContext';

const areAllMapsSelected = (checkboxStates) => {
  return MAP_CHECKBOX_IDS.every(id => checkboxStates[id]);
};

const handleSelectAllMaps = (checkboxStates, onCheckboxChange, gallery) => {
  const allSelected = MAP_CHECKBOX_IDS.every(id => checkboxStates[id]);
  const newState = !allSelected;
  MAP_CHECKBOX_IDS.forEach(id => {
    onCheckboxChange(id, newState);
  });
  setTimeout(() => {
    const currentObject = window.currentLoadedObject;
    if (currentObject && gallery) {
      gallery.loadObjectImages(currentObject);
    }
  }, 100);
};

const CheckboxItem = ({ id, label, checked, onChange }) => (
  <label className="checkbox-item">
    <input
      type="checkbox"
      id={id}
      checked={checked}
      onChange={(e) => onChange(id, e.target.checked)}
      aria-label={label}
    />
    <span className="checkmark"></span>
    <span> </span>{label}
  </label>
);

const Sidebar = ({ aladinInstance, checkboxStates, onCheckboxChange }) => {
  const gallery = useGallery();

  useEffect(() => {
    if (!aladinInstance) return;
    setupSidebarControls(aladinInstance, checkboxStates, onCheckboxChange);
  }, [aladinInstance, checkboxStates, onCheckboxChange]);

  return (
    <div className="sidebar">
      <div className="sidebar-content">
        <div className="control-section">
          <div className="section-header-with-checkbox">
            <label className="select-all-checkbox">
              <input
                type="checkbox"
                id="select-all-maps"
                checked={areAllMapsSelected(checkboxStates)}
                onChange={() => handleSelectAllMaps(checkboxStates, onCheckboxChange, gallery)}
                aria-label="Select all maps"
              />
              <span className="checkmark"></span>
            </label>
            <h4>Available Maps</h4>
          </div>

          <fieldset className="subsection">
            <legend><h5>Kinematics</h5></legend>
            <div className="checkbox-list">
              {KINEMATICS_CHECKBOXES.map(({ id, label }) => (
                <CheckboxItem
                  key={id}
                  id={id}
                  label={label}
                  checked={!!checkboxStates[id]}
                  onChange={onCheckboxChange}
                />
              ))}
            </div>
          </fieldset>

          <fieldset className="subsection">
            <legend><h5>Stellar Populations</h5></legend>
            <div className="checkbox-list">
              {POPULATION_CHECKBOXES.map(({ id, label }) => (
                <CheckboxItem
                  key={id}
                  id={id}
                  label={label}
                  checked={!!checkboxStates[id]}
                  onChange={onCheckboxChange}
                />
              ))}
            </div>
          </fieldset>
        </div>

        <div className="control-section">
          <h4>Display Options</h4>
          <fieldset>
            <div className="checkbox-list">
              {DISPLAY_CHECKBOXES.map(({ id, label }) => (
                <CheckboxItem
                  key={id}
                  id={id}
                  label={label}
                  checked={!!checkboxStates[id]}
                  onChange={onCheckboxChange}
                />
              ))}
            </div>
          </fieldset>
        </div>
      </div>
    </div>
  );
};

const setupSidebarControls = (aladin, checkboxStates, onCheckboxChange) => {
  const controls = [
    {
      id: 'display-grid',
      apply: () => aladin.showCooGrid(),
      onError: (error) => console.error('Grid control error:', error)
    },
    {
      id: 'display-reticle',
      apply: () => aladin.showReticle(true),
      onError: (error) => console.error('Reticle control error:', error)
    },
    {
      id: 'display-labels',
      apply: () => {
        const catalogs = aladin.getCatalogs();
        catalogs.forEach(catalog => {
          if (catalog.setShowLabels) {
            catalog.setShowLabels(true);
          }
        });
      },
      onError: (error) => console.log('Label display control not available:', error)
    },
    {
      id: 'display-healpix',
      apply: () => aladin.showHealpixGrid(true),
      onError: (error) => console.error('HEALPix grid control error:', error)
    }
  ];

  controls.forEach(({ id, apply, onError }) => {
    if (checkboxStates[id]) {
      try {
        apply();
      } catch (error) {
        onError(error);
      }
    }
  });
};

Sidebar.propTypes = {
  aladinInstance: PropTypes.object,
  checkboxStates: PropTypes.object.isRequired,
  onCheckboxChange: PropTypes.func.isRequired,
};

export default Sidebar; 
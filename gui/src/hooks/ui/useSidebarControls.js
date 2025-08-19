import { useState } from 'react';
import { MAP_CONTROLS } from '../../utils/constants/constants';
import { useAppState } from '../../contexts/AppStateContext';

export const useSidebarControls = (aladinInstance, gallery) => {
  // Import App state to compare with window global
  const { currentLoadedObject: contextObject } = useAppState();
  
  // Sidebar checkbox states
  const [sidebarState, setSidebarState] = useState({
    'map-stellar-velocity': false,
    'map-stellar-velocity-error': false,
    'map-velocity-dispersion': false,
    'map-velocity-dispersion-error': false,
    'map-h3': false,
    'map-h4': false,
    'map-age-weighted': false,
    'map-age-mass-weighted': false,
    'map-metallicity': false,
    'display-grid': false,
    'display-reticle': true, // Default checked
    'display-labels': false,
    'display-healpix': false
  });

  const handleCheckboxChange = (checkboxId, isChecked) => {
    if (checkboxId === 'map-h4') {
      console.log(`📋 H4 checkbox changed to: ${isChecked}, currentObject: ${window.currentLoadedObject}`);
    }
    
    setSidebarState(prev => ({ ...prev, [checkboxId]: isChecked }));
    
    // Handle Aladin display controls
    if (aladinInstance) {
      if (checkboxId === 'display-grid') {
        try { isChecked ? aladinInstance.showCooGrid() : aladinInstance.hideCooGrid(); } catch {}
      } else if (checkboxId === 'display-reticle') {
        try { aladinInstance.showReticle(isChecked); } catch {}
      } else if (checkboxId === 'display-labels') {
        try { aladinInstance.getCatalogs().forEach(c => c.setShowLabels && c.setShowLabels(isChecked)); } catch {}
      } else if (checkboxId === 'display-healpix') {
        try { aladinInstance.showHealpixGrid(isChecked); } catch {}
      }
    }
    
    // Handle map controls with gallery integration
    const mapControls = MAP_CONTROLS;
    if (mapControls[checkboxId]) {
      const config = mapControls[checkboxId];
      const currentObject = window.currentLoadedObject;
      if (!currentObject) {
        // Only handle placeholder logic when no object is loaded
        // Gallery state management handles loading when object exists
        const mapType = checkboxId.replace('map-', '');
        if (isChecked) {
          gallery.addMapToGallery(mapType, config.label, config.icon);
        } else {
          gallery.removeMapFromGallery(mapType);
        }
      }
    }
  };

  return {
    sidebarState,
    handleCheckboxChange
  };
}; 
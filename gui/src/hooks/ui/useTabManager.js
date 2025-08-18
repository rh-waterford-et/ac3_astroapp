import { useState, useEffect, useRef } from 'react';
import { MAP_CONTROLS, TIMEOUTS } from '../../utils/constants/constants';

export const useTabManager = (aladinInstance, sidebarState, gallery) => {
  const [activeTab, setActiveTab] = useState('maps');
  const tabTimeoutRef = useRef(null);

  // Handle tab switching with gallery restoration for maps tab
  useEffect(() => {
    if (activeTab === 'maps') {
      // Clear any existing timeout
      if (tabTimeoutRef.current) clearTimeout(tabTimeoutRef.current);
      
      // Set timeout to restore gallery state after tab switch
      tabTimeoutRef.current = setTimeout(() => {
        if (aladinInstance) {
          // React handlers now; no setupControls needed
        }
        
        // Get current object state
        const currentObject = window.currentLoadedObject;
        const currentCoords = window.currentObjectCoords;
        
        // Clear existing gallery items via DOM manipulation
        const galleryItems = document.getElementById('gallery-items');
        if (galleryItems) { 
          galleryItems.querySelectorAll('.gallery-item').forEach(item => item.remove()); 
        }
        
        // Restore object state if it existed
        if (currentObject) { 
          window.currentLoadedObject = currentObject; 
          window.currentObjectCoords = currentCoords; 
        }
        
        // Restore map gallery items based on sidebar state (only if no current object)
        if (!currentObject) {
          Object.entries(MAP_CONTROLS).forEach(([checkboxId, config]) => {
            const isChecked = sidebarState[checkboxId];
            if (isChecked) {
              const mapType = checkboxId.replace('map-', '');
              gallery.addMapToGallery(mapType, config.label, config.icon);
            }
          });
        }
      }, TIMEOUTS.restoreGalleryMs);
    }
    
    // Cleanup timeout on unmount or tab change
    return () => { 
      if (tabTimeoutRef.current) clearTimeout(tabTimeoutRef.current); 
    };
  }, [activeTab, aladinInstance, sidebarState, gallery]);

  return {
    activeTab,
    setActiveTab
  };
}; 
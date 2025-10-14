import { useState, useEffect, useRef } from 'react';
import { MAP_CONTROLS, TIMEOUTS } from '../../utils/constants/constants';

export const useTabManager = (aladinInstance, sidebarState, gallery) => {
  const [activeTab, setActiveTab] = useState('maps');
  const tabTimeoutRef = useRef(null);
  const galleryRef = useRef(gallery);
  
  // Update gallery ref when it changes
  galleryRef.current = gallery;

  // Handle tab switching with gallery restoration for maps tab
  useEffect(() => {
    if (activeTab === 'maps') {
      // Clear any existing timeout
      if (tabTimeoutRef.current) clearTimeout(tabTimeoutRef.current);
      
      // Set timeout to restore gallery state after tab switch
      tabTimeoutRef.current = setTimeout(() => {
        // Get current object state
        const currentObject = window.currentLoadedObject;
        const currentCoords = window.currentObjectCoords;
        
        // Clear gallery and reload based on context
        if (galleryRef.current?.clearGallery) {
          galleryRef.current.clearGallery();
        }
        
        // Restore object state if it existed
        if (currentObject) { 
          window.currentLoadedObject = currentObject; 
          window.currentObjectCoords = currentCoords;
          
          // Reload object images if we have a current object
          if (galleryRef.current?.loadObjectImages) {
            galleryRef.current.loadObjectImages(currentObject);
          }
        } else {
          // Restore map gallery items based on sidebar state (only if no current object)
          if (galleryRef.current?.addMapToGallery) {
            Object.entries(MAP_CONTROLS).forEach(([checkboxId, config]) => {
              const isChecked = sidebarState[checkboxId];
              if (isChecked) {
                const mapType = checkboxId.replace('map-', '');
                galleryRef.current.addMapToGallery(mapType, config.label, config.icon);
              }
            });
          }
        }
      }, TIMEOUTS.restoreGalleryMs);
    }
    
    // Cleanup timeout on unmount or tab change
    return () => { 
      if (tabTimeoutRef.current) clearTimeout(tabTimeoutRef.current); 
    };
  }, [activeTab]);

  return {
    activeTab,
    setActiveTab
  };
}; 
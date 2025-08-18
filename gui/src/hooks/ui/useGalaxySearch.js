import { useCallback } from 'react';
import { TIMEOUTS } from '../../utils/constants/constants';

export const useGalaxySearch = (aladinInstance, gallery) => {
  const handleSearch = useCallback((value) => {
    const input = value?.trim();
    if (!input || !aladinInstance) return;
    
    const currentPosition = aladinInstance.getRaDec();
    try {
      const beforeRa = currentPosition[0];
      const beforeDec = currentPosition[1];
      aladinInstance.gotoObject(input);
      
      setTimeout(() => {
        const afterPosition = aladinInstance.getRaDec();
        const raDiff = Math.abs(afterPosition[0] - beforeRa);
        const decDiff = Math.abs(afterPosition[1] - beforeDec);
        
        const statusElement = document.getElementById('current-status');
        
        if (raDiff < 0.01 && decDiff < 0.01) {
          // Object not found - position didn't change significantly
          if (statusElement) {
            statusElement.textContent = `Galaxy "${input}" not found. Try a different name or coordinates.`;
          }
        } else {
          // Object found - position changed
          if (statusElement) {
            statusElement.textContent = `Viewing: ${input}`;
          }
          
          // Update global state for backward compatibility
          window.currentLoadedObject = input;
          window.currentObjectCoords = afterPosition;
          
          // Load images for the found object
          gallery.loadObjectImages(input);
          
          // Clear the search input
          const searchElement = document.getElementById('galaxy-search');
          if (searchElement) searchElement.value = '';
        }
      }, TIMEOUTS.objectResolutionMs);
    } catch (error) {
      // Silent error handling like the original
    }
  }, [aladinInstance, gallery]);

  return {
    handleSearch
  };
}; 
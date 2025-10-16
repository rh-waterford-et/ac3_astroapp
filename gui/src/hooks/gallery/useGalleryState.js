import { useState, useCallback, useMemo, useRef } from 'react';
import { isAtObjectCoordinates } from '../../utils/gallery/galleryUtils';
import { MAP_CHECKBOX_IDS, KINEMATICS_CHECKBOXES, POPULATION_CHECKBOXES, PPXF_CHECKBOXES } from '../../utils/constants/constants';

// Build map of checkbox IDs to their labels and order for sorting
const MAP_TYPE_CONFIG = {};
let orderIndex = 0;

KINEMATICS_CHECKBOXES.forEach(({ id, label }) => {
  MAP_TYPE_CONFIG[id] = { label, order: orderIndex++ };
});

POPULATION_CHECKBOXES.forEach(({ id, label }) => {
  MAP_TYPE_CONFIG[id] = { label, order: orderIndex++ };
});

PPXF_CHECKBOXES.forEach(({ id, label }) => {
  MAP_TYPE_CONFIG[id] = { label, order: orderIndex++ };
});

const ITEMS_PER_PAGE = 50; // Legacy - not used in grouped mode

export const useGalleryState = (checkboxStates, onStatusUpdate) => {
  const [galleryGroups, setGalleryGroups] = useState([]);
  const [galleryState, setGalleryStateInternal] = useState('empty');
  const [currentObjectName, setCurrentObjectName] = useState(null);
  const savedPageStatesRef = useRef({}); // Store page states by mapType

  // Wrapper to log state changes  
  const setGalleryState = useCallback((newState) => {
    setGalleryStateInternal(newState);
  }, []);

  // Helper to get or create a group for a map type
  const getOrCreateGroup = useCallback((mapType, label) => {
    return (prevGroups) => {
      const existingGroup = prevGroups.find(g => g.mapType === mapType);
      
      if (existingGroup) {
        return prevGroups;
      }
      
      // Get config for this map type (extract key from mapType)
      const checkboxId = `map-${mapType}`;
      const config = MAP_TYPE_CONFIG[checkboxId] || { label, order: 999 };
      
      // Restore saved page state if it exists
      const savedPage = savedPageStatesRef.current[mapType];
      
      const newGroup = {
        mapType,
        label: config.label || label,
        order: config.order,
        items: [],
        currentPage: savedPage !== undefined ? savedPage : 0,
      };
      
      // Sort while preserving all properties (including currentPage) of existing groups
      const newGroups = [...prevGroups, newGroup].sort((a, b) => a.order - b.order);
      return newGroups;
    };
  }, []);

  // Calculate total items across all groups
  const totalItemsCount = useMemo(() => {
    return galleryGroups.reduce((sum, group) => sum + group.items.length, 0);
  }, [galleryGroups]);

  // Set loading state for an object
  const setLoadingState = useCallback((objectName) => {
    // Save current page states before clearing
    setGalleryGroups(prev => {
      const pageStates = {};
      prev.forEach(group => {
        pageStates[group.mapType] = group.currentPage;
      });
      savedPageStatesRef.current = pageStates;
      return []; // Clear existing groups
    });
    
    setCurrentObjectName(objectName);
    setGalleryState('loading');
  }, []);

  // Set navigate to object state
  const setNavigateToObjectState = useCallback((objectName) => {
    setCurrentObjectName(objectName);
    setGalleryState('navigate-to-object');
  }, []);

  // Add image item to gallery
  const addImageItem = useCallback((imageSrc, mapType, objectName) => {
    const newItem = {
      id: `image-${mapType.key}-${Date.now()}`,
      type: 'image',
      imageSrc,
      mapType,
      objectName
    };
    
    setGalleryGroups(prev => {
      // Ensure group exists
      let groups = getOrCreateGroup(mapType.key, mapType.label)(prev);
      
      // Add item to the group (preserve currentPage explicitly)
      groups = groups.map(group => 
        group.mapType === mapType.key
          ? { 
              ...group, 
              items: [...group.items, newItem],
              currentPage: group.currentPage || 0  // Preserve current page
            }
          : group
      );
      
      return groups;
    });
    
    setGalleryState('loaded');
  }, [getOrCreateGroup]);

  // Add PDF item to gallery (map to pPXF Fitting group for processing results)
  const addPdfItem = useCallback((pdfItem) => {
    const ppxfMapType = 'ppxf-fitting';
    const ppxfLabel = 'pPXF Fitting';
    
    setGalleryGroups(prev => {
      // Check if this PDF already exists in pPXF Fitting group
      const ppxfGroup = prev.find(g => g.mapType === ppxfMapType);
      if (ppxfGroup?.items.some(item => item.type === 'pdf' && item.pdfFile.key === pdfItem.pdfFile.key)) {
        return prev; // Already exists
      }
      
      // Ensure pPXF Fitting group exists
      let groups = getOrCreateGroup(ppxfMapType, ppxfLabel)(prev);
      
      // Add PDF to pPXF Fitting group (preserve currentPage explicitly)
      groups = groups.map(group => 
        group.mapType === ppxfMapType
          ? { 
              ...group, 
              items: [...group.items, pdfItem],
              currentPage: group.currentPage || 0  // Preserve current page
            }
          : group
      );
      
      return groups;
    });
    
    setGalleryState('loaded');
    return true;
  }, [getOrCreateGroup]);

  // Clear PDF items from pPXF Fitting group
  const clearPdfItems = useCallback(() => {
    setGalleryGroups(prev => 
      prev.map(group => 
        group.mapType === 'ppxf-fitting'
          ? { ...group, items: group.items.filter(item => item.type !== 'pdf') }
          : group
      )
    );
  }, []);

  // Add placeholder item to gallery
  const addPlaceholderItem = useCallback((mapType, label, icon) => {
    // Check if at least 1 checkbox is selected using props
    const mapCheckboxKeys = Object.keys(checkboxStates).filter(key => key.startsWith('map-'));
    const checkedCount = mapCheckboxKeys.filter(key => checkboxStates[key]).length;
    
    if (checkedCount < 1) {
      return;
    }
    
    // Check if we're at object coordinates
    if (!window.currentLoadedObject || !isAtObjectCoordinates(window.currentLoadedObject, window.aladinInstance)) {
      return;
    }
    
    const newItem = {
      id: `placeholder-${mapType}-${Date.now()}`,
      type: 'placeholder',
      mapType,
      label,
      icon
    };
    
    setGalleryGroups(prev => {
      // Ensure group exists
      let groups = getOrCreateGroup(mapType, label)(prev);
      
      // Check if placeholder already exists in group
      const group = groups.find(g => g.mapType === mapType);
      if (group?.items.some(item => item.type === 'placeholder' && item.mapType === mapType)) {
        return prev; // Already exists
      }
      
      // Add placeholder to the group (preserve currentPage explicitly)
      groups = groups.map(g => 
        g.mapType === mapType
          ? { 
              ...g, 
              items: [...g.items, newItem],
              currentPage: g.currentPage || 0  // Preserve current page
            }
          : g
      );
      
      return groups;
    });
    
    if (galleryState === 'empty') {
      setGalleryState('loaded');
    }
  }, [checkboxStates, galleryState, getOrCreateGroup]);

  // Remove entire group by map type
  const removeItemByMapType = useCallback((mapType) => {
    setGalleryGroups(prev => prev.filter(group => group.mapType !== mapType));
    
    // Check if we still meet the conditions for showing placeholders using props
    const mapCheckboxKeys = Object.keys(checkboxStates).filter(key => key.startsWith('map-'));
    const checkedCount = mapCheckboxKeys.filter(key => checkboxStates[key]).length;
    
    if (checkedCount < 1) {
      // Clear all groups since no map types are selected
      setGalleryGroups([]);
    }
    
    // Show empty message if no groups left
    setGalleryGroups(prev => {
      if (prev.length === 0 && galleryState !== 'loading') {
        setGalleryState('empty');
      }
      return prev;
    });
  }, [checkboxStates, galleryState]);

  // Clear all gallery groups and state
  const clearGallery = useCallback(() => {
    setGalleryGroups([]);
    setCurrentObjectName(null);
    setGalleryState('empty');
    
    // NOTE: We don't clear window.currentLoadedObject here anymore
    // This allows the view monitoring to reload images if user returns to the object
    
    // Update status using callback
    if (onStatusUpdate) {
      onStatusUpdate('Gallery cleared');
    }
  }, [onStatusUpdate]);

  // Handle loading status after images are processed
  const updateLoadingStatus = useCallback((imagesLoaded, mapTypes, objectName) => {
    // Use both current items AND newly loaded items count to prevent flash
    const currentTotal = totalItemsCount;
    const totalItems = currentTotal + imagesLoaded;
    
    if (totalItems === 0) {
      const anyChecked = mapTypes.some(mapType => checkboxStates[mapType.checkboxId] || false);
      
      if (!anyChecked) {
        setGalleryState('no-options');
      } else {
        setGalleryState('no-images');
      }
    } else {
      setGalleryState('loaded');
      
      if (onStatusUpdate) {
        onStatusUpdate(`Loaded ${totalItems} maps for ${objectName} - click on an image to select`);
      }
    }
  }, [totalItemsCount, checkboxStates, onStatusUpdate]);

  // Status update handler
  const updateStatus = useCallback((message) => {
    if (onStatusUpdate) {
      onStatusUpdate(message);
    }
  }, [onStatusUpdate]);

  // Change page for a specific group
  const changeGroupPage = useCallback((mapType, newPage) => {
    // Save to ref immediately
    savedPageStatesRef.current[mapType] = newPage;
    
    setGalleryGroups(prev => 
      prev.map(group => 
        group.mapType === mapType
          ? { ...group, currentPage: newPage }
          : group
      )
    );
  }, []);

  return {
    // State
    galleryGroups,
    galleryState,
    currentObjectName,
    totalItemsCount,
    
    // Loading actions
    setLoadingState,
    setNavigateToObjectState,
    updateLoadingStatus,
    
    // Item management actions
    addImageItem,
    addPdfItem,
    clearPdfItems,
    addPlaceholderItem,
    removeItemByMapType,
    clearGallery,
    
    // Pagination (per-group)
    changeGroupPage,
    
    // Utility actions
    updateStatus
  };
}; 
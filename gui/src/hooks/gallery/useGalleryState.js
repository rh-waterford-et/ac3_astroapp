import { useState, useCallback } from 'react';
import { isAtObjectCoordinates } from '../../utils/gallery/galleryUtils';

export const useGalleryState = (checkboxStates = {}, onStatusUpdate) => {
  // Core gallery state
  const [galleryItems, setGalleryItems] = useState([]);
  const [galleryState, setGalleryState] = useState('empty'); // 'empty', 'loading', 'loaded', 'no-images', 'no-options', 'navigate-to-object'
  const [currentObjectName, setCurrentObjectName] = useState(null);

  // Set loading state for an object
  const setLoadingState = useCallback((objectName) => {
    setCurrentObjectName(objectName);
    setGalleryState('loading');
    setGalleryItems([]); // Clear existing items
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
    
    setGalleryItems(prev => [...prev, newItem]);
    setGalleryState('loaded');
  }, []);

  // Add PDF item to gallery (called by PDF loader hook)
  const addPdfItem = useCallback((pdfItem) => {
    // Check if this PDF already exists
    const existingPdf = galleryItems.find(item => 
      item.type === 'pdf' && item.pdfFile.key === pdfItem.pdfFile.key
    );
    
    if (existingPdf) {
      return false; // Already exists, don't add
    }
    
    setGalleryItems(prev => [...prev, pdfItem]);
    setGalleryState('loaded');
    return true; // Successfully added
  }, [galleryItems]);

  // Clear PDF items from gallery
  const clearPdfItems = useCallback(() => {
    setGalleryItems(prev => prev.filter(item => item.type !== 'pdf'));
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
    
    
    // Check if this map type already exists
    const existingItem = galleryItems.find(item => item.mapType === mapType);
    if (existingItem) {
      return; // Already exists, don't add duplicate
    }
    
    const newItem = {
      id: `placeholder-${mapType}-${Date.now()}`,
      type: 'placeholder',
      mapType,
      label,
      icon
    };
    
    setGalleryItems(prev => [...prev, newItem]);
    if (galleryState === 'empty') {
      setGalleryState('loaded');
    }
  }, [checkboxStates, galleryItems, galleryState]);

  // Remove item by map type
  const removeItemByMapType = useCallback((mapType) => {
    setGalleryItems(prev => prev.filter(item => item.mapType !== mapType));
    
    // Check if we still meet the conditions for showing placeholders using props
    const mapCheckboxKeys = Object.keys(checkboxStates).filter(key => key.startsWith('map-'));
    const checkedCount = mapCheckboxKeys.filter(key => checkboxStates[key]).length;
    
    if (checkedCount < 1) {
      // Remove all remaining placeholders since we don't meet the minimum requirement
      setGalleryItems(prev => prev.filter(item => item.type !== 'placeholder'));
    }
    
    // Show empty message if no items left
    setGalleryItems(prev => {
      if (prev.length === 0) {
        setGalleryState('empty');
      }
      return prev;
    });
  }, [checkboxStates]);

  // Clear all gallery items and state
  const clearGallery = useCallback(() => {
    setGalleryItems([]);
    setCurrentObjectName(null);
    setGalleryState('empty');
    
    // Clear the current loaded object
    window.currentLoadedObject = null;
    
    // Update status using callback
    if (onStatusUpdate) {
      onStatusUpdate('Gallery cleared');
    }
    
  }, [onStatusUpdate]);

  // Handle loading status after images are processed
  const updateLoadingStatus = useCallback((imagesLoaded, mapTypes, objectName) => {
    // Check total items including those already in state
    const totalItems = galleryItems.length;
    
    if (totalItems === 0) {
      const anyChecked = mapTypes.some(mapType => {
        return checkboxStates[mapType.checkboxId] || false;
      });
      
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
  }, [galleryItems, checkboxStates, onStatusUpdate]);

  // Status update handler
  const updateStatus = useCallback((message) => {
    if (onStatusUpdate) {
      onStatusUpdate(message);
    }
  }, [onStatusUpdate]);

  return {
    // State
    galleryItems,
    galleryState,
    currentObjectName,
    
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
    
    // Utility actions
    updateStatus
  };
}; 
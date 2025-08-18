import { useCallback } from 'react';
import { 
  normalizeObjectName,
  isAtObjectCoordinates,
  tryGetObjectImage
} from '../../utils/gallery/galleryUtils';
import { IMAGE_MAP, MAP_TYPES } from '../../utils/constants/galleryConstants';
import { useAppState } from '../../contexts/AppStateContext';

export const useImageLoader = ({
  checkboxStates = {},
  aladinInstance,
  setNavigateToObjectState,
  setLoadingState,
  updateLoadingStatus,
  addImageItem,
  tryLoadPpxfPdfFiles
}) => {
  // App state hook for shared values
  const { aladinInstance: contextAladinInstance } = useAppState();

  // Main image loading orchestration
  const loadObjectImages = useCallback(async (objectName) => {
    console.log(`🔄 loadObjectImages called for: ${objectName}`);
    
    // Use context aladinInstance with fallback to prop and window (for backward compatibility)
    const currentAladinInstance = contextAladinInstance || aladinInstance || window.aladinInstance;
    
    // Check if we're at the correct coordinates for this object
    if (!isAtObjectCoordinates(objectName, currentAladinInstance)) {
      setNavigateToObjectState(objectName);
      return;
    }
    
    // Set loading state
    setLoadingState(objectName);
    
    // Normalize object name for file naming (remove spaces, make lowercase)
    const normalizedName = normalizeObjectName(objectName);
    
    // Process image loading for checked map types
    const imagesLoaded = await processImageLoading(MAP_TYPES, normalizedName, objectName, IMAGE_MAP);
    
    // Handle final status
    updateLoadingStatus(imagesLoaded, MAP_TYPES, objectName);
  }, [
    contextAladinInstance,
    aladinInstance,
    setNavigateToObjectState,
    setLoadingState,
    updateLoadingStatus,
    checkboxStates,
    addImageItem,
    tryLoadPpxfPdfFiles
  ]);

  // Process image loading for all map types
  const processImageLoading = useCallback(async (mapTypes, normalizedName, objectName, imageMap) => {
    let imagesLoaded = 0;
    
    for (const mapType of mapTypes) {
      const isChecked = checkboxStates[mapType.checkboxId] || false;
      
      if (isChecked) {
        // Special handling for H4: load dynamic PDF files from S3
        if (mapType.key === 'h4') {
          const pdfsLoaded = await tryLoadPpxfPdfFiles(objectName);
          if (pdfsLoaded > 0) {
            imagesLoaded += pdfsLoaded;
          }
        } else {
          // Use existing static image logic for other map types
          const imageFound = await tryLoadObjectImage(mapType, objectName, imageMap);
          if (imageFound) {
            imagesLoaded++;
          }
        }
      }
    }
    
    return imagesLoaded;
  }, [checkboxStates, tryLoadPpxfPdfFiles, addImageItem]);

  // Try to load a single object image
  const tryLoadObjectImage = useCallback(async (mapType, objectName, imageMap) => {
    const imageData = tryGetObjectImage(mapType, objectName, imageMap);
    
    if (imageData) {
      addImageItem(imageData.imageSrc, imageData.mapType, imageData.objectName);
      return true;
    } else {
      return false;
    }
  }, [addImageItem]);

  return {
    loadObjectImages,
    processImageLoading,
    tryLoadObjectImage
  };
}; 
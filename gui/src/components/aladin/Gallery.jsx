import React, { useEffect } from 'react';
import PropTypes from 'prop-types';
import GalleryRow from './gallery/GalleryRow';
import EmptyGalleryMessage from './gallery/ui/EmptyGalleryMessage';
import GalleryLoader from './gallery/ui/GalleryLoader';
import { 
  initializePdfJs
} from '../../utils/gallery/pdfUtils';
import { isAtObjectCoordinates } from '../../utils/gallery/galleryUtils';
import { findObjectAtCoordinates } from '../../utils/gallery/coordinateRegistry';
import { usePdfLoader } from '../../hooks/gallery/usePdfLoader';
import { useGalleryState } from '../../hooks/gallery/useGalleryState';
import { useImageLoader } from '../../hooks/gallery/useImageLoader';
import { useAppState } from '../../contexts/AppStateContext';

const Gallery = ({ aladinInstance, onGalleryOperationsReady, checkboxStates = {}, onStatusUpdate }) => {
  // App state hook for shared values
  const { currentLoadedObject } = useAppState();

  // Gallery state management hook
  const {
    galleryGroups,
    galleryState,
    currentObjectName,
    totalItemsCount,
    setLoadingState,
    setNavigateToObjectState,
    updateLoadingStatus,
    addImageItem,
    addPdfItem,
    clearPdfItems,
    addPlaceholderItem,
    removeItemByMapType,
    clearGallery,
    changeGroupPage,
    updateStatus,
  } = useGalleryState(checkboxStates, onStatusUpdate);

  // Set up PDF loader hook
  const { tryLoadPpxfPdfFiles } = usePdfLoader(addPdfItem, clearPdfItems);

  // Set up image loader hook
  const { loadObjectImages } = useImageLoader({
    checkboxStates,
    aladinInstance,
    setNavigateToObjectState,
    setLoadingState,
    updateLoadingStatus,
    addImageItem,
    addPdfItem,
    tryLoadPpxfPdfFiles
  });

  useEffect(() => {
    if (!aladinInstance) return;
    
    const cleanup = setupViewChangeMonitoring(aladinInstance);
    
    // Return cleanup function to remove event listener
    return () => {
      if (cleanup) cleanup();
    };
  }, [aladinInstance, totalItemsCount, loadObjectImages, clearGallery]);

  // Note: Scroll effect removed - pagination is now per-row, not global

  // Set up view change monitoring inside the component
  const setupViewChangeMonitoring = (aladin) => {
    if (!aladin) return null;
    let viewChangeTimeout;
    
    // Define the position change handler
    const positionChangeHandler = () => {
      // Debounce the view change to avoid too many updates
      clearTimeout(viewChangeTimeout);
      viewChangeTimeout = setTimeout(() => {
        // Use window.currentLoadedObject directly to avoid stale closure issues
        const loadedObject = window.currentLoadedObject;
        
        if (loadedObject) {
          // We have a loaded object - check if still at its coordinates
          const stillAtObject = isAtObjectCoordinates(loadedObject, aladin);
          
          if (stillAtObject) {
            // Still at object - but only reload if gallery is empty
            // This prevents unnecessary reloads when just panning within object radius
            if (totalItemsCount === 0) {
              loadObjectImages(loadedObject);
            }
            // If gallery already has items, do nothing (keep current state)
          } else {
            // User moved away from object - clear gallery but KEEP currentLoadedObject
            // so we can reload if they come back
            clearGallery();
          }
        } else if (totalItemsCount === 0) {
          // No loaded object and gallery is empty - check if we're at a registered object's coordinates
          try {
            const currentPos = aladin.getRaDec();
            if (currentPos && currentPos.length >= 2) {
              const matchedObject = findObjectAtCoordinates(currentPos[0], currentPos[1]);
              
              if (matchedObject) {
                // Found an object at these coordinates - load it
                window.currentLoadedObject = matchedObject;
                window.currentObjectCoords = currentPos;
                loadObjectImages(matchedObject, null, true);
              }
            }
          } catch (error) {
            // Silent error handling for coordinate lookup
          }
        }
      }, 200); // Wait 200ms after view stops changing
    };
    
    // Register the event listener
    aladin.on('positionChanged', positionChangeHandler);
    
    // Return cleanup function
    return () => {
      clearTimeout(viewChangeTimeout);
      // Note: Aladin Lite doesn't have an 'off' method, so we can't remove the listener
      // The event system will be cleaned up when the component unmounts
    };
  };

  // Render gallery content based on state
  const renderGalleryContent = () => {
    switch (galleryState) {
      case 'loading':
        return <GalleryLoader objectName={currentObjectName || 'object'} />;
      case 'empty':
        return <EmptyGalleryMessage type="empty" />;
      case 'no-images':
        return <EmptyGalleryMessage type="no-images" objectName={currentObjectName} />;
      case 'no-options':
        return <EmptyGalleryMessage type="no-options" objectName={currentObjectName} />;
      case 'navigate-to-object':
        return <EmptyGalleryMessage type="navigate-to-object" objectName={currentObjectName} />;
      case 'loaded':
        const rowCount = galleryGroups.length;
        const containerClass = rowCount === 1 
          ? 'gallery-rows-container single-row' 
          : 'gallery-rows-container multi-row';
        
        return (
          <div className={containerClass}>
            {galleryGroups.map(group => (
              <GalleryRow
                key={group.mapType}
                group={group}
                onPageChange={changeGroupPage}
                onStatusUpdate={updateStatus}
              />
            ))}
          </div>
        );
      default:
        return <EmptyGalleryMessage type="empty" />;
    }
  };

  // Expose gallery operations to parent component for Context
  useEffect(() => {
    if (onGalleryOperationsReady) {
      const operations = {
        addMapToGallery: addPlaceholderItem,
        removeMapFromGallery: removeItemByMapType,
        clearGallery: clearGallery,
        loadObjectImages: loadObjectImages,
        // New grouped structure
        galleryGroups,
        totalItemsCount,
      };
      onGalleryOperationsReady(operations);
    }
  }, [onGalleryOperationsReady, addPlaceholderItem, removeItemByMapType, clearGallery, loadObjectImages, galleryGroups, totalItemsCount]);

  // Determine content class based on number of rows
  const contentClass = galleryGroups.length === 1 
    ? 'gallery-content single-row-content' 
    : 'gallery-content multi-row-content';
  
  const galleryClass = galleryGroups.length === 1
    ? 'bottom-gallery single-row-gallery'
    : 'bottom-gallery multi-row-gallery';
  
  return (
    <div className={galleryClass}>
      <div className={contentClass}>
        {renderGalleryContent()}
      </div>
    </div>
  );
};
// Initialize PDF.js when the module loads
initializePdfJs();

Gallery.propTypes = {
  aladinInstance: PropTypes.object, // Can be null during initialization
  onGalleryOperationsReady: PropTypes.func, // Callback to provide gallery operations to parent
  checkboxStates: PropTypes.object, // Current checkbox states from parent
  onStatusUpdate: PropTypes.func, // Callback to update status display
};

export default Gallery; 
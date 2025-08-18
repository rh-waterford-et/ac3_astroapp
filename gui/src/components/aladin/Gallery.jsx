import React, { useEffect } from 'react';
import PropTypes from 'prop-types';
import ImageGalleryItem from './gallery/items/ImageGalleryItem';
import PDFGalleryItem from './gallery/items/PDFGalleryItem';
import PlaceholderGalleryItem from './gallery/items/PlaceholderGalleryItem';
import EmptyGalleryMessage from './gallery/ui/EmptyGalleryMessage';
import GalleryLoader from './gallery/ui/GalleryLoader';
import { 
  initializePdfJs
} from '../../utils/gallery/pdfUtils';
import { usePdfLoader } from '../../hooks/gallery/usePdfLoader';
import { useGalleryState } from '../../hooks/gallery/useGalleryState';
import { useImageLoader } from '../../hooks/gallery/useImageLoader';
import { useAppState } from '../../contexts/AppStateContext';

const Gallery = ({ aladinInstance, onGalleryOperationsReady, checkboxStates = {}, onStatusUpdate }) => {
  // App state hook for shared values
  const { currentLoadedObject } = useAppState();

  // Gallery state management hook
  const {
    galleryItems,
    galleryState,
    currentObjectName,
    setLoadingState,
    setNavigateToObjectState,
    updateLoadingStatus,
    addImageItem,
    addPdfItem,
    clearPdfItems,
    addPlaceholderItem,
    removeItemByMapType,
    clearGallery,
    updateStatus
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
    tryLoadPpxfPdfFiles
  });

  useEffect(() => {
    if (!aladinInstance) return;
    
    setupViewChangeMonitoring(aladinInstance);
  }, [aladinInstance]);

  // Set up view change monitoring inside the component
  const setupViewChangeMonitoring = (aladin) => {
    if (!aladin) return;
    let viewChangeTimeout;
    
    // Monitor view changes
    aladin.on('positionChanged', () => {
      // Debounce the view change to avoid too many updates
      clearTimeout(viewChangeTimeout);
      viewChangeTimeout = setTimeout(() => {
        // Check if we have a current object and need to update gallery
        if (currentLoadedObject) {
          loadObjectImages(currentLoadedObject);
        }
      }, 500); // Wait 500ms after view stops changing
    });
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
        return galleryItems.map(item => {
          switch (item.type) {
            case 'image':
              return (
                <ImageGalleryItem
                  key={item.id}
                  imageSrc={item.imageSrc}
                  mapType={item.mapType}
                  objectName={item.objectName}
                  onStatusUpdate={updateStatus}
                />
              );
            case 'pdf':
              return (
                <PDFGalleryItem
                  key={item.id}
                  pdfFile={item.pdfFile}
                  objectName={item.objectName}
                  onStatusUpdate={updateStatus}
                />
              );
            case 'placeholder':
              return (
                <PlaceholderGalleryItem
                  key={item.id}
                  mapType={item.mapType}
                  label={item.label}
                  icon={item.icon}
                  onStatusUpdate={updateStatus}
                />
              );
            default:
              return null;
          }
        });
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
        loadObjectImages: loadObjectImages
      };
      onGalleryOperationsReady(operations);
    }
  }, [onGalleryOperationsReady, addPlaceholderItem, removeItemByMapType, clearGallery, loadObjectImages]);

  return (
    <div className="bottom-gallery">
      <div className="gallery-content">
        <div className="gallery-items" id="gallery-items">
          {renderGalleryContent()}
        </div>
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
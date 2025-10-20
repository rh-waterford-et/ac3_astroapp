// Gallery modal - main modal component that combines navigation and image viewer
import React, { useEffect, useState, useCallback, useRef } from 'react';
import PropTypes from 'prop-types';
import { Rnd } from 'react-rnd';
import ModalNavigation from './ModalNavigation';
import ModalImageViewer from './ModalImageViewer';
import { useModalNavigation } from '../../../../hooks/modal/useModalNavigation';
import { useGallery } from '../../../../contexts/GalleryContext';
import { MODAL_DIMENSIONS } from '../../../../utils/constants/constants';

const GalleryModal = ({ modalRndRef, onStatusUpdate }) => {
  const [transparency, setTransparency] = useState(95);
  const galleryContainerRef = useRef(null);
  const gallery = useGallery();
  
  const {
    isModalOpen,
    currentImage,
    currentImageIndex,
    hasMultipleImages,
    totalItems,  // Now from the hook (row-specific count)
    openModal,
    closeModal,
    navigateToPrevious,
    navigateToNext,
    setCurrentImage
  } = useModalNavigation();

  // Update gallery container ref to find gallery items (now in rows container)
  useEffect(() => {
    galleryContainerRef.current = document.querySelector('.gallery-rows-container') || document.querySelector('.gallery-content');
  }, []);

  // Handle modal close
  const handleClose = useCallback(() => {
    closeModal();
    setTransparency(95); // Reset transparency
  }, [closeModal]);

  // Handle transparency change
  const handleTransparencyChange = useCallback((e) => {
    const value = parseInt(e.target.value);
    setTransparency(value);
  }, []);

  // Center modal function (exposed globally for compatibility)
  const centerModal = useCallback(() => {
    if (modalRndRef.current) {
      const modalWidth = MODAL_DIMENSIONS.widthPx;
      const modalHeight = MODAL_DIMENSIONS.heightPx;
      const centerX = Math.max(0, (window.innerWidth - modalWidth) / 2);
      const centerY = Math.max(0, (window.innerHeight - modalHeight) / 2);
      modalRndRef.current.updatePosition({ x: centerX, y: centerY });
    }
  }, [modalRndRef]);

  // Expose modal functions globally for backward compatibility
  useEffect(() => {
    window.centerModal = centerModal;
    
    // Expose openImageModal for backward compatibility
    window.openImageModal = (imageSrc, title, objectName, clickedItem = null, isPdf = false) => {
      // Re-query the gallery container to ensure we have the latest reference
      // This is important because the gallery may remount during tab switching
      const freshGalleryContainer = document.querySelector('.gallery-rows-container') || document.querySelector('.gallery-content');
      
      // Update the ref
      galleryContainerRef.current = freshGalleryContainer;
      
      const imageData = {
        src: imageSrc,
        title: title,
        objectName: objectName,
        isPdf: isPdf
      };
      
      openModal(imageData, clickedItem, freshGalleryContainer);
      
      // Center the modal
      setTimeout(centerModal, 50);
    };
    
    return () => {
      window.centerModal = undefined;
      window.openImageModal = undefined;
    };
  }, [openModal, centerModal]);

  // Handle backdrop click
  const handleBackdropClick = useCallback(() => {
    handleClose();
  }, [handleClose]);

  if (!isModalOpen) {
    return null;
  }

  return (
    <div className="image-modal active" id="image-modal">
      <div 
        className="modal-backdrop" 
        id="modal-backdrop"
        onClick={handleBackdropClick}
      />
      
      <Rnd
        ref={modalRndRef}
        default={{
          x: 0,
          y: 0,
          width: MODAL_DIMENSIONS.widthPx,
          height: MODAL_DIMENSIONS.heightPx,
        }}
        minWidth={480}
        minHeight={320}
        maxWidth="90vw"
        maxHeight="85vh"
        bounds="window"
        dragHandleClassName="modal-header"
        cancel=".modal-close, .transparency-control, .modal-nav-buttons"
        className="modal-content-rnd"
        onDragStart={() => {
          const modalContent = document.querySelector('.modal-content');
          if (modalContent) {
            modalContent.classList.add('dragging');
          }
        }}
        onDragStop={() => {
          const modalContent = document.querySelector('.modal-content');
          if (modalContent) {
            modalContent.classList.remove('dragging');
          }
        }}
        style={{ display: 'block' }}
      >
        <div className="modal-content" id="modal-content">
          <div className="modal-header">
            <span className="modal-object-code" id="modal-object">
              {currentImage?.objectName || 'Object'}
            </span>
            
            <h3 className="modal-title" id="modal-title">
              {currentImage?.title || 'Image Title'}
            </h3>
            
            <ModalNavigation
              onPrevious={navigateToPrevious}
              onNext={navigateToNext}
              onClose={handleClose}
              hasMultipleImages={hasMultipleImages}
              currentIndex={currentImageIndex}
              totalImages={totalItems}
              isModalOpen={isModalOpen}
            />
            
            <div className="modal-controls">
              <div className="transparency-control">
                <label htmlFor="transparency-slider" className="transparency-label">
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                    <circle cx="12" cy="12" r="3" stroke="currentColor" strokeWidth="2"/>
                    <path d="M12 1v6m0 6v6m11-7h-6m-6 0H1" stroke="currentColor" strokeWidth="2"/>
                  </svg>
                </label>
                <input 
                  type="range" 
                  id="transparency-slider" 
                  className="transparency-slider"
                  min="20" 
                  max="100" 
                  value={transparency}
                  onChange={handleTransparencyChange}
                  aria-label="Adjust image transparency from 20% to 100%"
                />
              </div>
              
              <button 
                className="modal-close" 
                id="modal-close"
                onClick={handleClose}
              >
                ×
              </button>
            </div>
          </div>
          
          <div 
            className="modal-body" 
            style={{ opacity: transparency / 100 }}
          >
            <ModalImageViewer
              imageData={currentImage}
              isVisible={isModalOpen}
            />
          </div>
        </div>
      </Rnd>
    </div>
  );
};

GalleryModal.propTypes = {
  modalRndRef: PropTypes.object.isRequired,
  onStatusUpdate: PropTypes.func
};

GalleryModal.defaultProps = {
  onStatusUpdate: null
};

export default GalleryModal; 
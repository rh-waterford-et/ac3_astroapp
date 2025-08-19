// Gallery modal - main modal component that combines navigation and image viewer
import React, { useEffect, useState, useCallback, useRef } from 'react';
import PropTypes from 'prop-types';
import { Rnd } from 'react-rnd';
import ModalNavigation from './ModalNavigation';
import ModalImageViewer from './ModalImageViewer';
import { useModalNavigation } from '../../../../hooks/modal/useModalNavigation';
import { MODAL_DIMENSIONS } from '../../../../utils/constants/constants';

const GalleryModal = ({ modalRndRef, onStatusUpdate }) => {
  const [transparency, setTransparency] = useState(95);
  const galleryContainerRef = useRef(null);
  
  const {
    isModalOpen,
    currentImage,
    currentImageIndex,
    galleryItems,
    hasMultipleImages,
    openModal,
    closeModal,
    navigateToPrevious,
    navigateToNext,
    getCurrentItem,
    setCurrentImage
  } = useModalNavigation();

  // Update gallery container ref to find gallery items
  useEffect(() => {
    galleryContainerRef.current = document.getElementById('gallery-items');
  }, []);

  // Handle navigation to update current image when index changes
  useEffect(() => {
    if (currentImageIndex >= 0 && galleryItems.length > 0) {
      const currentItem = getCurrentItem();
      if (currentItem && currentImage) {
        // Extract data from current item
        const isPdfItem = currentItem.classList.contains('pdf-item');
        
        if (isPdfItem) {
          // Handle PDF navigation
          const pdfKey = currentItem.dataset.pdfKey;
          const cellNumber = currentItem.dataset.cellNumber;
          const objectName = currentItem.dataset.objectName || 'Unknown';
          const label = currentItem.querySelector('.gallery-label');
          
          const pdfUrl = `/api/files/download?key=${encodeURIComponent(pdfKey)}#zoom=100&toolbar=0&navpanes=0`;
          const title = label ? label.textContent : `Cell ${cellNumber} H4`;
          
          setCurrentImage({
            src: pdfUrl,
            title: title,
            objectName: objectName,
            isPdf: true
          });
          
          // Update status
          if (onStatusUpdate) {
            onStatusUpdate(`Viewing ${objectName} H4 PDF: ${title} (${currentImageIndex + 1}/${galleryItems.length})`);
          }
        } else {
          // Handle regular image navigation
          const img = currentItem.querySelector('.thumbnail-image');
          if (img) {
            const label = currentItem.querySelector('.gallery-label');
            const objectName = currentItem.dataset.objectName || 'Unknown';
            const title = label ? label.textContent : 'Unknown Map';
            
            setCurrentImage({
              src: img.src,
              title: title,
              objectName: objectName,
              isPdf: false
            });
            
            // Update status
            if (onStatusUpdate) {
              onStatusUpdate(`Viewing ${objectName} map: ${title} (${currentImageIndex + 1}/${galleryItems.length})`);
            }
          }
        }
      }
    }
  }, [currentImageIndex, galleryItems, getCurrentItem, setCurrentImage, onStatusUpdate, currentImage]);

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
      const imageData = {
        src: imageSrc,
        title: title,
        objectName: objectName,
        isPdf: isPdf
      };
      
      openModal(imageData, clickedItem, galleryContainerRef.current);
      
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
              totalImages={galleryItems.length}
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
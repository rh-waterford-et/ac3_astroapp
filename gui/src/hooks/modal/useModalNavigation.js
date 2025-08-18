// Modal navigation hook - manages modal state and navigation
import { useState, useCallback } from 'react';

export const useModalNavigation = () => {
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [currentImageIndex, setCurrentImageIndex] = useState(-1);
  const [galleryItems, setGalleryItems] = useState([]);
  const [currentImage, setCurrentImage] = useState(null);

  /**
   * Set up navigation state when modal opens
   * @param {HTMLElement} clickedItem - The gallery item that was clicked
   * @param {string} imageSrc - The image source to match
   * @param {HTMLElement} galleryContainer - Gallery container to find items
   */
  const setupNavigationState = useCallback((clickedItem, imageSrc, galleryContainer) => {
    if (!galleryContainer) return;

    // Get all gallery items that have actual content (images or PDFs, not placeholders)
    const items = Array.from(galleryContainer.querySelectorAll('.gallery-item')).filter(item => {
      const img = item.querySelector('.thumbnail-image');
      const isPdfItem = item.classList.contains('pdf-item');
      // Include items with images OR PDF items (which don't have .thumbnail-image)
      return (img && img.src && !item.classList.contains('placeholder-item')) || isPdfItem;
    });

    setGalleryItems(items);

    // Find the current image index
    let index = -1;
    if (clickedItem) {
      index = items.indexOf(clickedItem);
    } else {
      // Fallback: find by image source (for regular images) or by PDF key (for PDFs)
      index = items.findIndex(item => {
        const img = item.querySelector('.thumbnail-image');
        if (img && img.src === imageSrc) {
          return true;
        }
        // For PDFs, check if the imageSrc contains the PDF key
        const pdfKey = item.dataset.pdfKey;
        if (pdfKey && imageSrc.includes(encodeURIComponent(pdfKey))) {
          return true;
        }
        return false;
      });
    }

    // Ensure valid index
    if (index === -1 && items.length > 0) {
      index = 0;
    }

    setCurrentImageIndex(index);
    
    return { items, index };
  }, []);

  /**
   * Navigate to previous image (with cycling)
   */
  const navigateToPrevious = useCallback(() => {
    if (galleryItems.length > 1) {
      setCurrentImageIndex(prev => {
        const newIndex = prev - 1;
        return newIndex < 0 ? galleryItems.length - 1 : newIndex; // Cycle to last image
      });
    }
  }, [galleryItems.length]);

  /**
   * Navigate to next image (with cycling)
   */
  const navigateToNext = useCallback(() => {
    if (galleryItems.length > 1) {
      setCurrentImageIndex(prev => {
        const newIndex = prev + 1;
        return newIndex >= galleryItems.length ? 0 : newIndex; // Cycle to first image
      });
    }
  }, [galleryItems.length]);

  /**
   * Open modal with image/PDF
   */
  const openModal = useCallback((imageData, clickedItem, galleryContainer) => {
    setCurrentImage(imageData);
    setIsModalOpen(true);
    
    // Set up navigation if we have gallery context
    if (galleryContainer) {
      setupNavigationState(clickedItem, imageData.src, galleryContainer);
    }
    
    // Prevent background scrolling
    document.body.style.overflow = 'hidden';
    
  }, [setupNavigationState]);

  /**
   * Close modal and clean up
   */
  const closeModal = useCallback(() => {
    setIsModalOpen(false);
    setCurrentImage(null);
    setCurrentImageIndex(-1);
    setGalleryItems([]);
    
    // Restore scrolling
    document.body.style.overflow = '';
    
  }, []);

  /**
   * Get current item from gallery items
   */
  const getCurrentItem = useCallback(() => {
    if (currentImageIndex >= 0 && currentImageIndex < galleryItems.length) {
      return galleryItems[currentImageIndex];
    }
    return null;
  }, [currentImageIndex, galleryItems]);

  /**
   * Check if navigation buttons should be enabled
   */
  const hasMultipleImages = galleryItems.length > 1;

  return {
    // State
    isModalOpen,
    currentImage,
    currentImageIndex,
    galleryItems,
    hasMultipleImages,
    
    // Actions
    openModal,
    closeModal,
    navigateToPrevious,
    navigateToNext,
    setupNavigationState,
    getCurrentItem,
    
    // Internal state setters (for direct updates when navigating)
    setCurrentImage
  };
}; 
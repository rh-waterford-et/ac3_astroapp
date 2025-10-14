// Modal navigation hook - manages modal state and navigation
import { useState, useCallback, useEffect } from 'react';
import { useGallery } from '../../contexts/GalleryContext';
import { createPdfModalUrl } from '../../utils/gallery/pdfUtils';
import { extractCellNumber, generatePdfDisplayName } from '../../utils/gallery/galleryUtils';

export const useModalNavigation = () => {
  const gallery = useGallery();
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [currentImageIndex, setCurrentImageIndex] = useState(-1); // Global index across all pages
  const [currentImage, setCurrentImage] = useState(null);

  // Get total items from gallery context
  const totalItems = gallery.galleryItems?.length || 0;

  /**
   * Set up navigation state when modal opens
   * @param {HTMLElement} clickedItem - The gallery item that was clicked
   * @param {string} imageSrc - The image source to match
   * @param {HTMLElement} galleryContainer - Gallery container to find items
   */
  const setupNavigationState = useCallback((clickedItem, imageSrc, galleryContainer) => {
    if (!galleryContainer) return;

    // Get all gallery items that have actual content (images or PDFs, not placeholders)
    const allGalleryItems = galleryContainer.querySelectorAll('.gallery-item');
    
    const items = Array.from(allGalleryItems).filter(item => {
      const img = item.querySelector('.thumbnail-image');
      const isPdfItem = item.classList.contains('pdf-item');
      const isPlaceholder = item.classList.contains('placeholder-item');
      
      // Include items with images OR PDF items (which don't have .thumbnail-image)
      return (img && img.src && !isPlaceholder) || isPdfItem;
    });

    // Find the current image index IN THE CURRENT PAGE
    let localIndex = -1;
    if (clickedItem) {
      localIndex = items.indexOf(clickedItem);
    } else {
      // Fallback: find by image source (for regular images) or by PDF key (for PDFs)
      localIndex = items.findIndex(item => {
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
    if (localIndex === -1 && items.length > 0) {
      localIndex = 0;
    }

    // Calculate GLOBAL index based on current page
    const globalIndex = (gallery.currentPage * gallery.itemsPerPage) + localIndex;
    setCurrentImageIndex(globalIndex);
    
    return { index: globalIndex };
  }, [gallery]);

  /**
   * Update the displayed image based on current global index
   */
  useEffect(() => {
    if (isModalOpen && currentImageIndex >= 0 && currentImageIndex < totalItems) {
      const item = gallery.galleryItems[currentImageIndex];
      if (item) {
        // Update the current image based on item type
        if (item.type === 'image') {
          setCurrentImage({
            src: item.imageSrc,
            title: item.mapType.label || 'Image',
            objectName: item.objectName,
            isPdf: false
          });
        } else if (item.type === 'pdf') {
          // For PDFs, construct the URL using the same utility as PDFGalleryItem
          const cellNumber = extractCellNumber(item.pdfFile.name);
          const displayName = generatePdfDisplayName(cellNumber);
          const pdfUrl = createPdfModalUrl(item.pdfFile.key);
          
          setCurrentImage({
            src: pdfUrl,
            title: displayName,
            objectName: item.objectName,
            isPdf: true
          });
        }
      }
    }
  }, [currentImageIndex, isModalOpen, totalItems, gallery.galleryItems]);

  /**
   * Navigate to previous image
   */
  const navigateToPrevious = useCallback(() => {
    if (totalItems > 1) {
      setCurrentImageIndex(prev => {
        const newIndex = prev - 1;
        return newIndex < 0 ? totalItems - 1 : newIndex; // Cycle to last image
      });
    }
  }, [totalItems]);

  /**
   * Navigate to next image
   */
  const navigateToNext = useCallback(() => {
    if (totalItems > 1) {
      setCurrentImageIndex(prev => {
        const newIndex = prev + 1;
        return newIndex >= totalItems ? 0 : newIndex; // Cycle to first image
      });
    }
  }, [totalItems]);

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
    
    // Restore scrolling
    document.body.style.overflow = '';
    
  }, []);

  /**
   * Check if navigation buttons should be enabled
   */
  const hasMultipleImages = totalItems > 1;

  return {
    // State
    isModalOpen,
    currentImage,
    currentImageIndex,
    hasMultipleImages,
    
    // Actions
    openModal,
    closeModal,
    navigateToPrevious,
    navigateToNext,
    setupNavigationState,
    
    // Internal state setters (for direct updates when navigating)
    setCurrentImage
  };
}; 

// Modal navigation hook - manages modal state and navigation
import { useState, useCallback, useEffect } from 'react';
import { useGallery } from '../../contexts/GalleryContext';
import { createPdfModalUrl } from '../../utils/gallery/pdfUtils';
import { extractCellNumber, generatePdfDisplayName } from '../../utils/gallery/galleryUtils';

export const useModalNavigation = () => {
  const gallery = useGallery();
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [currentImageIndex, setCurrentImageIndex] = useState(-1); // Index within the current group
  const [currentGroupMapType, setCurrentGroupMapType] = useState(null); // Which row/group is being viewed
  const [currentImage, setCurrentImage] = useState(null);
  const [totalItems, setTotalItems] = useState(0); // Store total items for current group

  // Get items only from the current group (row isolation)
  // Include all item types: images, PDFs, and placeholders
  const currentGroupItems = currentGroupMapType 
    ? (gallery.galleryGroups || [])
        .find(group => group.mapType === currentGroupMapType)
        ?.items || []
    : [];

  /**
   * Set up navigation state when modal opens
   * @param {HTMLElement} clickedItem - The gallery item that was clicked
   * @param {string} imageSrc - The image source to match
   * @param {HTMLElement} galleryContainer - Gallery container to find items
   */
  const setupNavigationState = useCallback((clickedItem, imageSrc, galleryContainer) => {
    if (!galleryContainer) return;

    // Step 1: Find which row this item belongs to
    let clickedRow = clickedItem?.closest('.gallery-row');
    if (!clickedRow) {
      // Fallback: find row by searching for the item
      const allRows = galleryContainer.querySelectorAll('.gallery-row');
      for (const row of allRows) {
        const items = row.querySelectorAll('.gallery-item');
        if (Array.from(items).includes(clickedItem)) {
          clickedRow = row;
          break;
        }
      }
    }
    
    if (!clickedRow) {
      console.warn('Could not find row for clicked item');
      return;
    }

    // Step 2: Get the mapType from the row (extract from header or items)
    const rowItems = clickedRow.querySelectorAll('.gallery-item');
    let mapType = null;
    
    if (clickedItem) {
      // Try to get mapType from the clicked item
      mapType = clickedItem.dataset.mapType || 
                clickedItem.querySelector('[data-map-type]')?.dataset.mapType;
    }
    
    // Fallback: get from first item in row
    if (!mapType && rowItems.length > 0) {
      mapType = rowItems[0].dataset.mapType || 
                rowItems[0].querySelector('[data-map-type]')?.dataset.mapType;
    }
    
    if (!mapType) {
      console.warn('Could not determine mapType for row');
      return;
    }

    // Step 3: Find the group in gallery state
    const group = (gallery.galleryGroups || []).find(g => g.mapType === mapType);
    if (!group) {
      console.warn('Could not find group for mapType:', mapType);
      return;
    }

    // Step 4: Find the index within this group's items (include all types: images, PDFs, placeholders)
    const groupItems = group.items;
    let itemIndex = -1;
    
    // Match by imageSrc
    if (imageSrc) {
      itemIndex = groupItems.findIndex(item => {
        if (item.type === 'image') {
          return item.imageSrc === imageSrc;
        }
        if (item.type === 'pdf') {
          const encodedKey = encodeURIComponent(item.pdfFile?.key || '');
          return imageSrc.includes(encodedKey);
        }
        if (item.type === 'placeholder') {
          // Match placeholder by its mapType pattern: "placeholder:mapType"
          return imageSrc === `placeholder:${item.mapType}`;
        }
        return false;
      });
    }
    
    // Fallback: match by data attributes
    if (itemIndex === -1 && clickedItem) {
      const pdfKey = clickedItem.dataset.pdfKey;
      const cellNumber = clickedItem.dataset.cellNumber;
      
      if (pdfKey) {
        itemIndex = groupItems.findIndex(item => 
          item.type === 'pdf' && item.pdfFile?.key === pdfKey
        );
      }
      
      if (itemIndex === -1 && cellNumber) {
        itemIndex = groupItems.findIndex(item => {
          if (item.type === 'pdf') {
            const itemCellNumber = extractCellNumber(item.pdfFile?.name);
            return itemCellNumber === parseInt(cellNumber);
          }
          return false;
        });
      }
    }
    
    // Fallback to 0 if still not found
    if (itemIndex === -1) {
      console.warn('Could not find item in group, defaulting to index 0');
      itemIndex = 0;
    }
    
    // Set the group, index, and total items count
    setCurrentGroupMapType(mapType);
    setCurrentImageIndex(itemIndex);
    setTotalItems(groupItems.length);
    
    return { index: itemIndex, mapType, totalItems: groupItems.length };
  }, [gallery.galleryGroups]);

  /**
   * Update the displayed image based on current index within the group
   */
  useEffect(() => {
    if (isModalOpen && currentImageIndex >= 0 && currentImageIndex < totalItems) {
      const item = currentGroupItems[currentImageIndex];
      if (item) {
        // Update the current image based on item type
        if (item.type === 'image') {
          setCurrentImage({
            src: item.imageSrc,
            title: item.mapType.label || 'Image',
            objectName: item.objectName,
            isPdf: false,
            isPlaceholder: false
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
            isPdf: true,
            isPlaceholder: false
          });
        } else if (item.type === 'placeholder') {
          // For placeholders, use the placeholder identifier
          setCurrentImage({
            src: `placeholder:${item.mapType}`,
            title: item.label || 'Placeholder',
            objectName: window.currentLoadedObject || 'Object',
            isPdf: false,
            isPlaceholder: true,
            icon: item.icon
          });
        }
      }
    }
  }, [currentImageIndex, isModalOpen, totalItems, currentGroupItems]);

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
    // Set up navigation FIRST if we have gallery context
    if (galleryContainer) {
      setupNavigationState(clickedItem, imageData.src, galleryContainer);
    }
    
    // Then open the modal
    setCurrentImage(imageData);
    setIsModalOpen(true);
    
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
    setCurrentGroupMapType(null); // Reset group context
    setTotalItems(0); // Reset total items count
    
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
    totalItems,  // Added for modal counter display
    
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

import React, { useEffect } from 'react';
import PropTypes from 'prop-types';

// Import NGC7025 images from assets
import NGC7025_stellar_velocity from '../../assets/NGC7025_stellar_velocity.jpg';
import NGC7025_stellar_velocity_error from '../../assets/NGC7025_stellar_velocity_error.jpg';
import NGC7025_velocity_dispersion from '../../assets/NGC7025_velocity_dispersion.jpg';
import NGC7025_velocity_dispersion_error from '../../assets/NGC7025_velocity_dispersion_error.jpg';
import NGC7025_h3 from '../../assets/NGC7025_h3.jpg';
import NGC7025_h4 from '../../assets/NGC7025_h4.jpg';
import NGC7025_age from '../../assets/NGC7025_age.jpeg';
import NGC7025_age_mass_weighted from '../../assets/NGC7025_age_mass_weighted.jpg';
import NGC7025_metallicity from '../../assets/NGC7025_metallicity.jpg';

// Import API functions
import { getDatasetOutputFilesPaginated } from '../../services/api.js';

// Module-level PDF cache for progressive loading
const pdfCache = new Map();

// Cache helper functions
const getCacheKey = (objectName, processorType) => `${objectName}-${processorType}`;

const getCachedPdfs = (objectName, processorType) => {
  const key = getCacheKey(objectName, processorType);
  return pdfCache.get(key) || { batches: [], total: 0, lastUpdated: 0 };
};

const setCachedPdfs = (objectName, processorType, data) => {
  const key = getCacheKey(objectName, processorType);
  pdfCache.set(key, { ...data, lastUpdated: Date.now() });
};

const Gallery = ({ aladinInstance }) => {
  useEffect(() => {
    if (!aladinInstance) return;
    
    setupGalleryControls(aladinInstance);
  }, [aladinInstance]);

  return (
    <div className="bottom-gallery">
      <div className="gallery-content">
        <div className="gallery-items" id="gallery-items">
          {/* Gallery starts empty - images appear when sidebar options are selected */}
          <div className="empty-gallery-message" id="empty-gallery-message">
            <p>Select at least 1 option from the sidebar and navigate to a celestial object to view maps</p>
          </div>
        </div>
      </div>
    </div>
  );
};

/**
 * Set up gallery controls functionality
 * @param {Object} aladin - The Aladin Lite instance
 */
const setupGalleryControls = (aladin) => {
  setupImageModal();
  setupViewChangeMonitoring(aladin);
};

/**
 * Monitor Aladin view changes to update gallery based on coordinates
 * @param {Object} aladin - The Aladin Lite instance
 */
const setupViewChangeMonitoring = (aladin) => {
  if (!aladin) return;
  
  let viewChangeTimeout;
  
  // Monitor view changes
  aladin.on('positionChanged', () => {
    // Debounce the view change to avoid too many updates
    clearTimeout(viewChangeTimeout);
    viewChangeTimeout = setTimeout(() => {
      // Check if we have a current object and need to update gallery
      if (window.currentLoadedObject) {
        loadObjectImages(window.currentLoadedObject);
      }
    }, 500); // Wait 500ms after view stops changing
  });
};

/**
 * Set up image modal functionality
 */
const setupImageModal = () => {
  // Setup modal with react-rnd integration
  setupModalRnd();
};

/**
 * Setup modal for react-rnd integration
 */
const setupModalRnd = () => {
  // The modal is now handled by react-rnd, so we only need to setup basic modal functionality
  const modal = document.getElementById('image-modal');
  const modalClose = document.getElementById('modal-close');
  const modalBackdrop = document.getElementById('modal-backdrop');
  
  // Close modal when clicking close button
  if (modalClose) {
    modalClose.addEventListener('click', closeImageModal);
  }
  
  // Close modal when clicking backdrop
  if (modalBackdrop) {
    modalBackdrop.addEventListener('click', closeImageModal);
  }
  
  // Close modal with Escape key
  document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape' && modal && modal.classList.contains('active')) {
      closeImageModal();
    }
  });
};

/**
 * Global variables for modal navigation
 */
let currentGalleryItems = [];
let currentImageIndex = -1;

/**
 * Set up modal navigation functionality
 */
const setupModalNavigation = () => {
  const prevBtn = document.getElementById('modal-prev');
  const nextBtn = document.getElementById('modal-next');
  
  if (prevBtn && nextBtn) {
    prevBtn.addEventListener('click', navigateToPrevious);
    nextBtn.addEventListener('click', navigateToNext);
    
    // Add keyboard navigation
    document.addEventListener('keydown', handleModalKeydown);
  }
};

/**
 * Handle keyboard navigation in modal
 * @param {KeyboardEvent} e - The keyboard event
 */
const handleModalKeydown = (e) => {
  const modal = document.getElementById('image-modal');
  if (!modal || !modal.classList.contains('active')) return;
  
  switch (e.key) {
    case 'ArrowLeft':
      e.preventDefault();
      navigateToPrevious();
      break;
    case 'ArrowRight':
      e.preventDefault();
      navigateToNext();
      break;
    case 'Escape':
      e.preventDefault();
      closeImageModal();
      break;
  }
};

/**
 * Update navigation button states based on current position
 */
const updateNavigationButtons = () => {
  const prevBtn = document.getElementById('modal-prev');
  const nextBtn = document.getElementById('modal-next');
  
  if (prevBtn && nextBtn) {
    // Enable buttons if we have more than 1 image (cycling mode)
    const hasMultipleImages = currentGalleryItems.length > 1;
    prevBtn.disabled = !hasMultipleImages;
    nextBtn.disabled = !hasMultipleImages;
  }
};

/**
 * Navigate to previous image in gallery (with cycling)
 */
const navigateToPrevious = () => {
  if (currentGalleryItems.length > 1) {
    currentImageIndex--;
    if (currentImageIndex < 0) {
      currentImageIndex = currentGalleryItems.length - 1; // Cycle to last image
    }
    const item = currentGalleryItems[currentImageIndex];
    displayImageFromNavigation(item);
  }
};

/**
 * Navigate to next image in gallery (with cycling)
 */
const navigateToNext = () => {
  if (currentGalleryItems.length > 1) {
    currentImageIndex++;
    if (currentImageIndex >= currentGalleryItems.length) {
      currentImageIndex = 0; // Cycle to first image
    }
    const item = currentGalleryItems[currentImageIndex];
    displayImageFromNavigation(item);
  }
};

/**
 * Display image from navigation (without rebuilding gallery state)
 */
const displayImageFromNavigation = (item) => {
  const modalImage = document.getElementById('modal-image');
  const modalTitle = document.getElementById('modal-title');
  const modalObject = document.getElementById('modal-object');
  const modalBody = document.querySelector('.modal-body');
  const transparencySlider = document.getElementById('transparency-slider');
  
  if (modalImage && modalTitle && modalObject) {
    // Check if this is a PDF item or regular image item
    const isPdfItem = item.classList.contains('pdf-item');
    const img = item.querySelector('.thumbnail-image');
    
    if (isPdfItem) {
      // For PDF items, get the PDF URL and title
      const pdfKey = item.dataset.pdfKey;
      const cellNumber = item.dataset.cellNumber;
      const objectName = item.dataset.objectName || 'Unknown';
      const label = item.querySelector('.gallery-label');
      
      const pdfUrl = `/api/files/download?key=${encodeURIComponent(pdfKey)}#zoom=100&toolbar=0&navpanes=0`;
      const title = label ? label.textContent : `Cell ${cellNumber} H4`;
      
      // Call openImageModal to handle PDF display properly (skip navigation setup)
      openImageModal(pdfUrl, title, objectName, item, true, true);
      
      // Update navigation buttons and status
      updateNavigationButtons();
      
      // Update status
      const statusElement = document.getElementById('current-status');
      if (statusElement) {
        statusElement.textContent = `Viewing ${objectName} H4 PDF: ${title} (${currentImageIndex + 1}/${currentGalleryItems.length})`;
      }
      
      return; // Exit early since openImageModal handles everything
    } else if (img) {
      // For regular images
      modalImage.src = img.src;
      modalImage.alt = img.alt;
      
      // Clean up any existing PDF iframe
      const modalImageContainer = modalImage.parentElement;
      const existingIframe = modalImageContainer.querySelector('.pdf-iframe');
      if (existingIframe) {
        existingIframe.remove();
      }
      
      // Ensure image is visible
      modalImage.style.display = 'block';
      
      // Get title and object from the item
      const label = item.querySelector('.gallery-label');
      const objectName = item.dataset.objectName || 'Unknown';
      
      modalTitle.textContent = label ? label.textContent : 'Unknown Map';
      modalObject.textContent = objectName;
      
      // Reset image transform and interactions
      modalImage.style.transform = 'translate(0, 0) scale(1)';
      modalImage._currentZoom = 1;
      modalImage._currentX = 0;
      modalImage._currentY = 0;
      setupImageInteractions(modalImage);
      
      // Reset transparency
      if (transparencySlider && modalBody) {
        transparencySlider.value = 95;
        modalBody.style.opacity = 0.95;
      }
      
      // Update navigation buttons
      updateNavigationButtons();
      
      // Update status
      const statusElement = document.getElementById('current-status');
      if (statusElement) {
        statusElement.textContent = `Viewing ${objectName} map: ${modalTitle.textContent} (${currentImageIndex + 1}/${currentGalleryItems.length})`;
      }
    } else {
      console.log('⚠️ Could not find image or PDF data for navigation item');
    }
  }
};

/**
 * Open image modal with full-size image or PDF
 * @param {string} imageSrc - The image source or PDF URL
 * @param {string} title - The image title
 * @param {string} objectName - The object name
 * @param {HTMLElement} clickedItem - The gallery item that was clicked (optional)
 * @param {boolean} isPdf - Whether the content is a PDF (optional)
 * @param {boolean} skipNavigationSetup - Whether to skip setting up navigation (for navigation calls)
 */
const openImageModal = (imageSrc, title, objectName, clickedItem = null, isPdf = false, skipNavigationSetup = false) => {
  const modal = document.getElementById('image-modal');
  const modalImage = document.getElementById('modal-image');
  const modalTitle = document.getElementById('modal-title');
  const modalObject = document.getElementById('modal-object');
  const modalRnd = document.querySelector('.modal-content-rnd');
  const modalBody = document.querySelector('.modal-body');
  const transparencySlider = document.getElementById('transparency-slider');
  
  if (modal && modalImage && modalTitle && modalObject && modalRnd) {
    if (isPdf) {
      // For PDFs, create an iframe element and hide the image
      modalImage.style.display = 'none';
      
      // Clean up any existing iframe
      const modalImageContainer = modalImage.parentElement;
      const existingIframe = modalImageContainer.querySelector('.pdf-iframe');
      if (existingIframe) {
        existingIframe.remove();
      }
      
      // Create new iframe for PDF
      const iframe = document.createElement('iframe');
      iframe.className = 'pdf-iframe';
      iframe.src = imageSrc;
      iframe.style.cssText = 'width: 100%; height: 100%; border: none; border-radius: 4px;';
      iframe.title = `${title} for ${objectName}`;
      
      // Add load and error handlers for debugging
      iframe.onload = () => {
        console.log(`✅ PDF iframe loaded successfully: ${title}`);
      };
      iframe.onerror = (error) => {
        console.error(`❌ PDF iframe failed to load: ${title}`, error);
      };
      
      modalImageContainer.appendChild(iframe);
    } else {
      // For images, use the normal img element and clean up any PDF iframes
      const modalImageContainer = modalImage.parentElement;
      const existingIframe = modalImageContainer.querySelector('.pdf-iframe');
      if (existingIframe) {
        existingIframe.remove();
      }
      
      modalImage.style.display = 'block';
      modalImage.src = imageSrc;
      modalImage.alt = `${title} for ${objectName}`;
    }
    
    modalTitle.textContent = title;
    modalObject.textContent = objectName;
    
    // Set up navigation state (skip if already navigating)
    if (!skipNavigationSetup) {
      setupNavigationState(clickedItem, imageSrc);
    }
    
    // Reset transparency to default (only affect body)
    if (transparencySlider && modalBody) {
      transparencySlider.value = 95;
      modalBody.style.opacity = 0.95;
    }
    
    // Reset image position, zoom and set up interactions
    modalImage.style.transform = 'translate(0, 0) scale(1)';
    modalImage._currentZoom = 1;
    modalImage._currentX = 0;
    modalImage._currentY = 0;
    setupImageInteractions(modalImage);
    
    // Set up navigation if not already done
    setupModalNavigation();
    
    // Show the modal and the Rnd component
    modal.classList.add('active');
    modalRnd.style.display = 'block';
    document.body.style.overflow = 'hidden'; // Prevent background scrolling
    
    // Center the modal using react-rnd's positioning API
    if (window.centerModal) {
      window.centerModal();
    }
    
    console.log(`🖼️ Opened modal for ${title} - ${objectName} (${currentImageIndex + 1}/${currentGalleryItems.length})`);
  }
};

/**
 * Set up navigation state for modal
 * @param {HTMLElement} clickedItem - The gallery item that was clicked
 * @param {string} imageSrc - The image source to match
 */
const setupNavigationState = (clickedItem, imageSrc) => {
  // Get all gallery items that have actual content (images or PDFs, not placeholders)
  const galleryItems = document.getElementById('gallery-items');
  if (galleryItems) {
    currentGalleryItems = Array.from(galleryItems.querySelectorAll('.gallery-item')).filter(item => {
      const img = item.querySelector('.thumbnail-image');
      const isPdfItem = item.classList.contains('pdf-item');
      // Include items with images OR PDF items (which don't have .thumbnail-image)
      return (img && img.src && !item.classList.contains('placeholder-item')) || isPdfItem;
    });
    
    // Find the current image index
    if (clickedItem) {
      currentImageIndex = currentGalleryItems.indexOf(clickedItem);
    } else {
      // Fallback: find by image source (for regular images) or by PDF key (for PDFs)
      currentImageIndex = currentGalleryItems.findIndex(item => {
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
    if (currentImageIndex === -1) {
      currentImageIndex = 0;
    }
    
    // Update navigation buttons
    updateNavigationButtons();
  }
};

/**
 * Close image modal
 */
const closeImageModal = () => {
  const modal = document.getElementById('image-modal');
  const modalImage = document.getElementById('modal-image');
  const modalRnd = document.querySelector('.modal-content-rnd');
  const modalBody = document.querySelector('.modal-body');
  
  if (modal) {
    modal.classList.remove('active');
    document.body.style.overflow = ''; // Restore scrolling
    
    // Hide the Rnd component
    if (modalRnd) {
      modalRnd.style.display = 'none';
    }
    
    // Reset modal body opacity
    if (modalBody) {
      modalBody.style.opacity = '';
    }
    
    // Clear image source and reset position/zoom
    if (modalImage) {
      modalImage.src = '';
      modalImage.style.display = 'block'; // Ensure image is visible for next use
      modalImage.style.transform = 'translate(0, 0) scale(1)';
      modalImage._currentZoom = 1;
      modalImage._currentX = 0;
      modalImage._currentY = 0;
      removeInteractionListeners(modalImage);
      
      // Clean up any PDF iframe
      const modalImageContainer = modalImage.parentElement;
      const existingIframe = modalImageContainer.querySelector('.pdf-iframe');
      if (existingIframe) {
        existingIframe.remove();
      }
    }
    
    // Clean up navigation state
    currentGalleryItems = [];
    currentImageIndex = -1;
    
    // Remove keyboard event listener
    document.removeEventListener('keydown', handleModalKeydown);
    
    console.log('🖼️ Closed modal');
  }
};

/**
 * Set up image interactions (dragging and zooming)
 * @param {HTMLImageElement} image - The image element to make interactive
 */
const setupImageInteractions = (image) => {
  let isDragging = false;
  let startX = 0;
  let startY = 0;
  let lastTouchDistance = 0;
  let lastTouchCenter = { x: 0, y: 0 };
  
  // Update transform with current values
  const updateTransform = () => {
    image.style.transform = `translate(${image._currentX}px, ${image._currentY}px) scale(${image._currentZoom})`;
  };
  
  // Mouse wheel zoom
  const handleWheel = (e) => {
    e.preventDefault();
    
    const rect = image.getBoundingClientRect();
    const centerX = rect.left + rect.width / 2;
    const centerY = rect.top + rect.height / 2;
    
    const zoomFactor = e.deltaY > 0 ? 0.9 : 1.1;
    const newZoom = Math.max(0.1, Math.min(5, image._currentZoom * zoomFactor));
    
    // Zoom towards mouse position
    const mouseX = e.clientX - centerX;
    const mouseY = e.clientY - centerY;
    
    image._currentX += mouseX * (1 - zoomFactor);
    image._currentY += mouseY * (1 - zoomFactor);
    image._currentZoom = newZoom;
    
    updateTransform();
  };
  
  // Mouse drag events
  const handleMouseDown = (e) => {
    isDragging = true;
    startX = e.clientX - image._currentX;
    startY = e.clientY - image._currentY;
    image.classList.add('dragging');
    e.preventDefault();
  };
  
  const handleMouseMove = (e) => {
    if (!isDragging) return;
    
    image._currentX = e.clientX - startX;
    image._currentY = e.clientY - startY;
    
    updateTransform();
    e.preventDefault();
  };
  
  const handleMouseUp = () => {
    isDragging = false;
    image.classList.remove('dragging');
  };
  
  // Touch events
  const getTouchDistance = (touches) => {
    const dx = touches[0].clientX - touches[1].clientX;
    const dy = touches[0].clientY - touches[1].clientY;
    return Math.sqrt(dx * dx + dy * dy);
  };
  
  const getTouchCenter = (touches) => {
    return {
      x: (touches[0].clientX + touches[1].clientX) / 2,
      y: (touches[0].clientY + touches[1].clientY) / 2
    };
  };
  
  const handleTouchStart = (e) => {
    e.preventDefault();
    
    if (e.touches.length === 1) {
      // Single touch - dragging
      isDragging = true;
      const touch = e.touches[0];
      startX = touch.clientX - image._currentX;
      startY = touch.clientY - image._currentY;
      image.classList.add('dragging');
    } else if (e.touches.length === 2) {
      // Two touches - pinch zoom
      isDragging = false;
      image.classList.remove('dragging');
      lastTouchDistance = getTouchDistance(e.touches);
      lastTouchCenter = getTouchCenter(e.touches);
    }
  };
  
  const handleTouchMove = (e) => {
    e.preventDefault();
    
    if (e.touches.length === 1 && isDragging) {
      // Single touch drag
      const touch = e.touches[0];
      image._currentX = touch.clientX - startX;
      image._currentY = touch.clientY - startY;
      updateTransform();
    } else if (e.touches.length === 2) {
      // Pinch zoom
      const newDistance = getTouchDistance(e.touches);
      const newCenter = getTouchCenter(e.touches);
      
      if (lastTouchDistance > 0) {
        const zoomFactor = newDistance / lastTouchDistance;
        const newZoom = Math.max(0.1, Math.min(5, image._currentZoom * zoomFactor));
        
        // Zoom towards touch center
        const rect = image.getBoundingClientRect();
        const imageCenterX = rect.left + rect.width / 2;
        const imageCenterY = rect.top + rect.height / 2;
        
        const touchX = newCenter.x - imageCenterX;
        const touchY = newCenter.y - imageCenterY;
        
        image._currentX += touchX * (1 - zoomFactor);
        image._currentY += touchY * (1 - zoomFactor);
        image._currentZoom = newZoom;
        
        updateTransform();
      }
      
      lastTouchDistance = newDistance;
      lastTouchCenter = newCenter;
    }
  };
  
  const handleTouchEnd = (e) => {
    if (e.touches.length === 0) {
      isDragging = false;
      image.classList.remove('dragging');
      lastTouchDistance = 0;
    } else if (e.touches.length === 1) {
      // Switch back to dragging mode
      const touch = e.touches[0];
      startX = touch.clientX - image._currentX;
      startY = touch.clientY - image._currentY;
      isDragging = true;
      image.classList.add('dragging');
      lastTouchDistance = 0;
    }
  };
  
  // Double-click/tap to reset
  const handleDoubleClick = () => {
    image._currentX = 0;
    image._currentY = 0;
    image._currentZoom = 1;
    updateTransform();
  };
  
  // Add event listeners
  image.addEventListener('wheel', handleWheel, { passive: false });
  image.addEventListener('mousedown', handleMouseDown);
  document.addEventListener('mousemove', handleMouseMove);
  document.addEventListener('mouseup', handleMouseUp);
  image.addEventListener('dblclick', handleDoubleClick);
  
  image.addEventListener('touchstart', handleTouchStart, { passive: false });
  document.addEventListener('touchmove', handleTouchMove, { passive: false });
  document.addEventListener('touchend', handleTouchEnd);
  
  // Store references for cleanup
  image._interactionHandlers = {
    handleWheel,
    handleMouseDown,
    handleMouseMove,
    handleMouseUp,
    handleDoubleClick,
    handleTouchStart,
    handleTouchMove,
    handleTouchEnd
  };
};

/**
 * Remove interaction event listeners
 * @param {HTMLImageElement} image - The image element
 */
const removeInteractionListeners = (image) => {
  if (image._interactionHandlers) {
    const handlers = image._interactionHandlers;
    
    image.removeEventListener('wheel', handlers.handleWheel);
    image.removeEventListener('mousedown', handlers.handleMouseDown);
    document.removeEventListener('mousemove', handlers.handleMouseMove);
    document.removeEventListener('mouseup', handlers.handleMouseUp);
    image.removeEventListener('dblclick', handlers.handleDoubleClick);
    
    image.removeEventListener('touchstart', handlers.handleTouchStart);
    document.removeEventListener('touchmove', handlers.handleTouchMove);
    document.removeEventListener('touchend', handlers.handleTouchEnd);
    
    delete image._interactionHandlers;
  }
};

/**
 * Process image loading for checked map types
 * @param {Array} mapTypes - Array of map type configurations
 * @param {string} normalizedName - Normalized object name
 * @param {string} objectName - Original object name
 * @returns {Promise<number>} - Number of images loaded
 */
const processImageLoading = async (mapTypes, normalizedName, objectName, imageMap) => {
  let imagesLoaded = 0;
  
  for (const mapType of mapTypes) {
    const checkbox = document.getElementById(mapType.checkboxId);
    
    if (checkbox && checkbox.checked) {
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
};

/**
 * Handle the final status after image loading
 * @param {number} imagesLoaded - Number of images loaded
 * @param {Array} mapTypes - Array of map type configurations
 * @param {string} objectName - Original object name
 */
const handleLoadingStatus = (imagesLoaded, mapTypes, objectName) => {
  // Hide loading indicator
  hideGalleryLoader();
  
  // Check if there are already items in the gallery (like PDFs)
  const galleryItems = document.getElementById('gallery-items');
  const existingItems = galleryItems.querySelectorAll('.gallery-item');
  const totalItems = imagesLoaded + existingItems.length;
  
  if (totalItems === 0) {
    const anyChecked = mapTypes.some(mapType => {
      const checkbox = document.getElementById(mapType.checkboxId);
      return checkbox && checkbox.checked;
    });
    
    if (!anyChecked) {
      showNoOptionsSelectedMessage(objectName);
    } else {
      showNoImagesMessage(objectName);
    }
  } else {
    console.log(`Loaded ${totalItems} total items for ${objectName} (${imagesLoaded} static images + ${existingItems.length} existing items)`);
    
    const statusElement = document.getElementById('current-status');
    if (statusElement) {
      statusElement.textContent = `Loaded ${totalItems} maps for ${objectName} - click on an image to select`;
    }
  }
};

/**
 * Load object images when navigating to a celestial object
 * @param {string} objectName - The name of the object (e.g., 'NGC7025')
 */
const loadObjectImages = async (objectName) => {
  console.log(`🔄 loadObjectImages called for: ${objectName}`);
  const galleryItems = document.getElementById('gallery-items');
  const emptyMessage = document.getElementById('empty-gallery-message');
  
  // Check if we're at the correct coordinates for this object
  if (!isAtObjectCoordinates(objectName)) {
    console.log(`📍 Not at ${objectName} coordinates, showing location message`);
    showNavigateToObjectMessage(objectName);
    return;
  }
  
  // Hide empty message
  if (emptyMessage) {
    emptyMessage.style.display = 'none';
  }
  
  // Clear existing items completely
  const existingItems = galleryItems.querySelectorAll('.gallery-item, .no-images-message, .gallery-loader');
  console.log(`🧹 Clearing ${existingItems.length} existing items`);
  existingItems.forEach(item => item.remove());
  
  // Also clear any placeholder items that might have been added
  galleryItems.innerHTML = '';
  console.log(`🧹 Gallery cleared completely`);
  
  // Show loading indicator
  showGalleryLoader(galleryItems, objectName);
  
  // Normalize object name for file naming (remove spaces, make lowercase)
  const normalizedName = objectName.replace(/\s+/g, '').toUpperCase();
  
  // Image mapping for specific objects (can be extended for other objects)
  // To add support for other objects, import their images at the top of the file
  // and add them to this map with the same suffix structure
  const imageMap = {
    NGC7025: {
      stellar_velocity: NGC7025_stellar_velocity,
      stellar_velocity_error: NGC7025_stellar_velocity_error,
      velocity_dispersion: NGC7025_velocity_dispersion,
      velocity_dispersion_error: NGC7025_velocity_dispersion_error,
      h3: NGC7025_h3,
      h4: NGC7025_h4,
      age: NGC7025_age,
      age_mass_weighted: NGC7025_age_mass_weighted,
      metallicity: NGC7025_metallicity
    }
    // Add other objects here as needed, e.g.:
    // NGC6027: {
    //   stellar_velocity: NGC6027_stellar_velocity,
    //   // ... other map types
    // }
  };
  
  // Map types that we expect to find images for
  const mapTypes = [
    { key: 'stellar-velocity', suffix: 'stellar_velocity', label: 'Stellar Velocity', checkboxId: 'map-stellar-velocity' },
    { key: 'stellar-velocity-error', suffix: 'stellar_velocity_error', label: 'Stellar Velocity Error', checkboxId: 'map-stellar-velocity-error' },
    { key: 'velocity-dispersion', suffix: 'velocity_dispersion', label: 'Velocity Dispersion', checkboxId: 'map-velocity-dispersion' },
    { key: 'velocity-dispersion-error', suffix: 'velocity_dispersion_error', label: 'Velocity Dispersion Error', checkboxId: 'map-velocity-dispersion-error' },
    { key: 'h3', suffix: 'h3', label: 'H3', checkboxId: 'map-h3' },
    { key: 'h4', suffix: 'h4', label: 'H4', checkboxId: 'map-h4' },
    { key: 'age-lum-weighted', suffix: 'age', label: 'Age (Lum. Weighted)', checkboxId: 'map-age-weighted' },
    { key: 'age-mass-weighted', suffix: 'age_mass_weighted', label: 'Age (Mass Weighted)', checkboxId: 'map-age-mass-weighted' },
    { key: 'metallicity', suffix: 'metallicity', label: 'Metallicity', checkboxId: 'map-metallicity' }
  ];
  
  // Process image loading for checked map types
  const imagesLoaded = await processImageLoading(mapTypes, normalizedName, objectName, imageMap);
  
  // Handle final status
  handleLoadingStatus(imagesLoaded, mapTypes, objectName);
};

/**
 * Try to load an image for a specific object and map type
 * @param {Object} mapType - Map type configuration object
 * @param {string} objectName - The original object name for display
 * @param {Object} imageMap - Mapping of object names to their image assets
 * @returns {Promise<boolean>} - True if image was found and loaded
 */
const tryLoadObjectImage = async (mapType, objectName, imageMap) => {
  // Normalize object name to match image map keys
  const normalizedName = objectName.replace(/\s+/g, '').toUpperCase();
  
  // Check if we have an image map for this object
  if (!imageMap[normalizedName]) {
    console.log(`No image map found for object: ${normalizedName}`);
    return false;
  }
  
  // Get the image source for this map type
  const imageSrc = imageMap[normalizedName][mapType.suffix];
  
  if (imageSrc) {
    console.log(`✅ Found image for ${objectName} ${mapType.label}: ${mapType.suffix}`);
    addImageToGallery(imageSrc, mapType, objectName);
    return true;
  } else {
    console.log(`❌ No image found for ${objectName} ${mapType.label}: ${mapType.suffix}`);
    return false;
  }
};

/**
 * Add an image to the gallery
 * @param {string} imageSrc - The image source
 * @param {Object} mapType - Map type configuration
 * @param {string} objectName - The object name
 */
const addImageToGallery = (imageSrc, mapType, objectName) => {
  console.log(`🖼️ Adding object image: ${mapType.label} for ${objectName}`);
  const galleryItems = document.getElementById('gallery-items');
  
  const mapItem = document.createElement('div');
  mapItem.className = 'gallery-item object-map-item';
  mapItem.dataset.mapType = mapType.key;
  mapItem.dataset.objectName = objectName;
  
  mapItem.innerHTML = `
    <div class="gallery-thumbnail">
      <img src="${imageSrc}" alt="${mapType.label} for ${objectName}" class="thumbnail-image" />
    </div>
    <div class="gallery-label">${mapType.label}</div>
  `;
  
  // Add click handler to open modal
  mapItem.addEventListener('click', () => {
    console.log(`Clicked on ${objectName} map: ${mapType.key}`);
    
    // Open modal with full-size image, passing the clicked item
    openImageModal(imageSrc, mapType.label, objectName, mapItem);
    
    // Update status
    const statusElement = document.getElementById('current-status');
    if (statusElement) {
      statusElement.textContent = `Viewing ${objectName} map: ${mapType.label}`;
    }
    
    // Toggle selection
    mapItem.classList.toggle('selected');
    
    // Remove selection from other items
    const otherItems = galleryItems.querySelectorAll('.gallery-item:not([data-map-type="' + mapType.key + '"])');
    otherItems.forEach(item => item.classList.remove('selected'));
  });
  
  galleryItems.appendChild(mapItem);
};

/**
 * Check if an image exists at the given path
 * @param {string} imagePath - The image path to check
 * @returns {Promise<boolean>} - True if image exists
 */
const checkImageExists = (imagePath) => {
  return new Promise((resolve) => {
    const img = new Image();
    img.onload = () => resolve(true);
    img.onerror = () => resolve(false);
    img.src = imagePath;
  });
};

/**
 * Show message when no images are found for an object
 * @param {string} objectName - The object name
 */
const showNoImagesMessage = (objectName) => {
  const galleryItems = document.getElementById('gallery-items');
  
  const messageDiv = document.createElement('div');
  messageDiv.className = 'no-images-message';
  messageDiv.innerHTML = `
    <p>No map images available for ${objectName}</p>
    <p class="sub-message">Images will be loaded when available in the format: ${objectName.replace(/\s+/g, '').toUpperCase()}_maptype.jpg</p>
  `;
  
  galleryItems.appendChild(messageDiv);
  
  // Update status
  const statusElement = document.getElementById('current-status');
  if (statusElement) {
    statusElement.textContent = `No images found for ${objectName}`;
  }
};

/**
 * Show message when no sidebar options are selected
 * @param {string} objectName - The object name
 */
const showNoOptionsSelectedMessage = (objectName) => {
  const galleryItems = document.getElementById('gallery-items');
  
  const messageDiv = document.createElement('div');
  messageDiv.className = 'no-images-message';
  messageDiv.innerHTML = `
    <p>Select map options from the sidebar to view ${objectName} images</p>
    <p class="sub-message">Check the boxes in "Available Maps" to load corresponding images</p>
  `;
  
  galleryItems.appendChild(messageDiv);
  
  // Update status
  const statusElement = document.getElementById('current-status');
  if (statusElement) {
    statusElement.textContent = `Viewing ${objectName} - select map options to load images`;
  }
};

/**
 * Show message when not at object coordinates
 * @param {string} objectName - The object name
 */
const showNavigateToObjectMessage = (objectName) => {
  const galleryItems = document.getElementById('gallery-items');
  
  // Clear existing items
  const existingItems = galleryItems.querySelectorAll('.gallery-item, .no-images-message');
  existingItems.forEach(item => item.remove());
  galleryItems.innerHTML = '';
  
  const messageDiv = document.createElement('div');
  messageDiv.className = 'no-images-message';
  messageDiv.innerHTML = `
    <p>Navigate closer to the searched object to view map images</p>
    <p class="sub-message">Use the galaxy search to find and navigate to celestial objects</p>
  `;
  
  galleryItems.appendChild(messageDiv);
  
  // Update status
  const statusElement = document.getElementById('current-status');
  if (statusElement) {
    statusElement.textContent = `Navigate closer to object to view images`;
  }
};

/**
 * Check if current Aladin view is at or near the object coordinates
 * @param {string} objectName - The object name
 * @returns {boolean} - True if at object coordinates
 */
const isAtObjectCoordinates = (objectName) => {
  // If no coordinates stored or no object loaded, don't show images
  if (!window.currentObjectCoords || !window.currentLoadedObject) {
    console.log(`📍 No coordinates stored (coords: ${!!window.currentObjectCoords}, object: ${window.currentLoadedObject})`);
    return false;
  }
  
  // Only check coordinates if this is the currently loaded object
  if (window.currentLoadedObject !== objectName) {
    console.log(`📍 Object mismatch: requested ${objectName}, loaded ${window.currentLoadedObject}`);
    return false;
  }
  
  // Get current Aladin position
  const aladinInstance = window.aladinInstance;
  if (!aladinInstance) {
    console.log(`📍 No Aladin instance available`);
    return false;
  }
  
  try {
    const currentPos = aladinInstance.getRaDec();
    const objectCoords = window.currentObjectCoords;
    
    // Validate coordinates
    if (!currentPos || !objectCoords || currentPos.length < 2 || objectCoords.length < 2) {
      console.log(`📍 Invalid coordinate data: current=${currentPos}, object=${objectCoords}`);
      return false;
    }
    
    // Calculate angular distance between current position and object
    const deltaRA = Math.abs(currentPos[0] - objectCoords[0]);
    const deltaDec = Math.abs(currentPos[1] - objectCoords[1]);
    
    // Allow very small tolerance (within 0.05 degrees = 3 arcminutes)
    const tolerance = 0.05;
    
    const isNear = deltaRA < tolerance && deltaDec < tolerance;
    console.log(`📍 Coordinate check: Current(${currentPos[0].toFixed(3)}, ${currentPos[1].toFixed(3)}) vs Object(${objectCoords[0].toFixed(3)}, ${objectCoords[1].toFixed(3)}) - Distance: RA=${deltaRA.toFixed(3)}°, Dec=${deltaDec.toFixed(3)}° - Near: ${isNear}`);
    
    return isNear;
  } catch (error) {
    console.error('Error checking coordinates:', error);
    return false; // Changed: Don't show images if coordinate check fails
  }
};

/**
 * Add a map item to the gallery
 * @param {string} mapType - The type of map (e.g., 'stellar-velocity')
 * @param {string} label - The display label for the map
 * @param {string} icon - The icon to display
 */
const addMapToGallery = (mapType, label, icon) => {
  console.log(`📦 Checking conditions for placeholder: ${label} (${mapType})`);
  
  // Check if at least 1 checkbox is selected
  const allCheckboxes = document.querySelectorAll('input[type="checkbox"][id^="map-"]');
  const checkedCount = Array.from(allCheckboxes).filter(cb => cb.checked).length;
  
  if (checkedCount < 1) {
    console.log(`❌ Not enough options selected (${checkedCount}/1 minimum required)`);
    return;
  }
  
  // Check if we're at object coordinates
  if (!window.currentLoadedObject || !isAtObjectCoordinates(window.currentLoadedObject)) {
    console.log(`❌ Not at object coordinates or no object loaded`);
    return;
  }
  
  console.log(`✅ Both conditions met - adding placeholder: ${label} (${mapType})`);
  
  const galleryItems = document.getElementById('gallery-items');
  const emptyMessage = document.getElementById('empty-gallery-message');
  
  // Hide empty message if it exists
  if (emptyMessage) {
    emptyMessage.style.display = 'none';
  }
  
  // Check if this map type already exists
  const existingItem = galleryItems.querySelector(`[data-map-type="${mapType}"]`);
  if (existingItem) {
    return; // Already exists, don't add duplicate
  }
  
  // Create new map item
  const mapItem = document.createElement('div');
  mapItem.className = 'gallery-item placeholder-item';
  mapItem.dataset.mapType = mapType;
  
  mapItem.innerHTML = `
    <div class="gallery-thumbnail">
      <div class="thumbnail-placeholder map-placeholder">
        <span class="map-icon">${icon}</span>
      </div>
    </div>
    <div class="gallery-label">${label}</div>
  `;
  
  // Add click handler for selection
  mapItem.addEventListener('click', () => {
    console.log(`Clicked on map: ${mapType}`);
    
    // Update status
    const statusElement = document.getElementById('current-status');
    if (statusElement) {
      statusElement.textContent = `Selected map: ${label} (placeholder - backend integration needed)`;
    }
    
    // Toggle selection
    mapItem.classList.toggle('selected');
  });
  
  galleryItems.appendChild(mapItem);
  console.log(`Added ${label} to gallery`);
};

/**
 * Remove a map item from the gallery
 * @param {string} mapType - The type of map to remove
 */
const removeMapFromGallery = (mapType) => {
  const galleryItems = document.getElementById('gallery-items');
  const mapItem = galleryItems.querySelector(`[data-map-type="${mapType}"]`);
  
  if (mapItem) {
    mapItem.remove();
    console.log(`Removed ${mapType} from gallery`);
  }
  
  // Check if we still meet the conditions for showing placeholders
  const allCheckboxes = document.querySelectorAll('input[type="checkbox"][id^="map-"]');
  const checkedCount = Array.from(allCheckboxes).filter(cb => cb.checked).length;
  
  if (checkedCount < 1) {
    // Remove all remaining placeholders since we don't meet the minimum requirement
    const remainingPlaceholders = galleryItems.querySelectorAll('.placeholder-item');
    remainingPlaceholders.forEach(item => item.remove());
    console.log(`Removed all placeholders - only ${checkedCount} options selected (minimum 1 required)`);
  }
  
  // Show empty message if no items left
  const remainingItems = galleryItems.querySelectorAll('.gallery-item');
  if (remainingItems.length === 0) {
    showEmptyGalleryMessage();
  }
};

/**
 * Clear all items from gallery
 */
const clearGallery = () => {
  const galleryItems = document.getElementById('gallery-items');
  const mapItems = galleryItems.querySelectorAll('.gallery-item');
  
  mapItems.forEach(item => item.remove());
  
  // Clear the current loaded object
  window.currentLoadedObject = null;
  
  showEmptyGalleryMessage();
  
  // Update status
  const statusElement = document.getElementById('current-status');
  if (statusElement) {
    statusElement.textContent = 'Gallery cleared';
  }
  
  console.log('Gallery cleared');
};

/**
 * Show the empty gallery message
 */
const showEmptyGalleryMessage = () => {
  const galleryItems = document.getElementById('gallery-items');
  const emptyMessage = document.getElementById('empty-gallery-message');
  
  if (emptyMessage) {
    emptyMessage.style.display = 'flex';
  } else {
    // Create empty message if it doesn't exist
    const newEmptyMessage = document.createElement('div');
    newEmptyMessage.className = 'empty-gallery-message';
    newEmptyMessage.id = 'empty-gallery-message';
    newEmptyMessage.innerHTML = `
      <p>Select at least 1 option from the sidebar and navigate to a celestial object to view maps</p>
    `;
    galleryItems.appendChild(newEmptyMessage);
  }
};

/**
 * Create and add a single PDF item to the gallery
 * @param {Object} pdfFile - PDF file object with name and key
 * @param {string} objectName - The original object name
 * @returns {number} - 1 if added, 0 if skipped
 */
const addPdfToGallery = (pdfFile, objectName) => {
  const galleryItems = document.getElementById('gallery-items');
  const cellNumber = pdfFile.name.split('/')[0];
  const fileName = pdfFile.name.split('/').pop();
  const displayName = `Cell ${cellNumber} H4`;
  
  // Check if this PDF already exists in gallery
  const existingPdfItem = galleryItems.querySelector(`[data-map-type="h4"][data-cell-number="${cellNumber}"]`);
  if (existingPdfItem) {
    console.log(`⏭️ Skipping ${displayName}: already exists`);
    return 0;
  }
  
  // Create gallery item for PDF
  const mapItem = document.createElement('div');
  mapItem.className = 'gallery-item object-map-item pdf-item';
  mapItem.dataset.mapType = 'h4';
  mapItem.dataset.objectName = objectName;
  mapItem.dataset.cellNumber = cellNumber;
  mapItem.dataset.pdfKey = pdfFile.key; // S3 key for download
  
  mapItem.innerHTML = `
    <div class="gallery-thumbnail">
      <div class="thumbnail-placeholder pdf-placeholder" id="pdf-thumb-${cellNumber}">
        <canvas class="pdf-thumbnail-canvas" width="150" height="200" style="display: none;"></canvas>
        <div class="pdf-loading-indicator">
          <span class="cell-label">Cell ${cellNumber}</span>
          <div class="loading-text">Loading preview...</div>
        </div>
      </div>
    </div>
    <div class="gallery-label">${displayName}</div>
  `;
  
  // Generate PDF thumbnail
  const thumbnailUrl = `/api/files/download?key=${encodeURIComponent(pdfFile.key)}`;
  generatePdfThumbnail(thumbnailUrl, cellNumber);
  
  // Add click handler for PDF viewing in modal
  mapItem.addEventListener('click', () => {
    console.log(`📄 Clicked on ${objectName} H4 PDF: Cell ${cellNumber}`);
    console.log(`📄 PDF Key: ${pdfFile.key}`);
    
    // Create PDF URL for modal display with PDF.js viewer settings
    const pdfUrl = `/api/files/download?key=${encodeURIComponent(pdfFile.key)}#zoom=100&toolbar=0&navpanes=0`;
    
    // Open PDF in modal using existing modal system
    openImageModal(pdfUrl, `${displayName} PDF`, objectName, mapItem, true); // true indicates it's a PDF
    
    // Update status
    const statusElement = document.getElementById('current-status');
    if (statusElement) {
      statusElement.textContent = `Viewing ${objectName} H4 PDF: Cell ${cellNumber}`;
    }
    
    // Toggle selection
    mapItem.classList.toggle('selected');
    
    // Remove selection from other items
    const otherItems = galleryItems.querySelectorAll('.gallery-item:not([data-cell-number="' + cellNumber + '"])');
    otherItems.forEach(item => item.classList.remove('selected'));
  });
  
  galleryItems.appendChild(mapItem);
  console.log(`✅ Added H4 PDF to gallery: Cell ${cellNumber}`);
  return 1;
};

/**
 * Load PDFs progressively with caching (first 50, then background batches)
 * @param {string} normalizedObjectName - Normalized object name for API calls
 * @param {string} objectName - Original object name for display
 * @param {number} totalPdfs - Total number of PDFs available
 * @returns {Promise<number>} - Number of PDFs loaded in first batch
 */
const loadPdfsProgressively = async (normalizedObjectName, objectName, totalPdfs) => {
  console.log(`📄 Progressive loading for ${normalizedObjectName}: ${totalPdfs || 'unknown'} total PDFs`);
  
  // 1. Check cache first - show immediately if available
  const cached = getCachedPdfs(normalizedObjectName, 'ppxf');
  let pdfsLoaded = 0;
  
  if (cached.batches.length > 0) {
    console.log(`💨 Loading ${cached.total} cached PDFs immediately`);
    cached.batches.forEach(batch => {
      batch.files.forEach(pdfFile => {
        pdfsLoaded += addPdfToGallery(pdfFile, objectName);
      });
    });
    
    // If cache is complete and we know the total, return early
    if (totalPdfs && cached.total === totalPdfs) {
      console.log(`✅ Cache complete: ${pdfsLoaded} PDFs loaded from cache`);
      return pdfsLoaded;
    }
  }
  
  // 2. Load first batch (0-49) using paginated API
  try {
    const firstBatch = await getDatasetOutputFilesPaginated(normalizedObjectName, 'ppxf', 50, 0);
    console.log(`📄 First batch loaded: ${firstBatch.files.length} PDFs (0-${firstBatch.files.length - 1})`);
    
    // Add first batch to gallery
    let firstBatchLoaded = 0;
    firstBatch.files.forEach(pdfFile => {
      firstBatchLoaded += addPdfToGallery(pdfFile, objectName);
    });
    pdfsLoaded += firstBatchLoaded;
    
    // 3. Update cache with first batch
    setCachedPdfs(normalizedObjectName, 'ppxf', {
      batches: [{ offset: 0, files: firstBatch.files }],
      total: firstBatch.total
    });
    
    // 4. Start background loading for remaining batches
    if (firstBatch.hasMore) {
      const remainingText = firstBatch.total > 0 ? `${firstBatch.total - firstBatch.files.length} PDFs` : 'additional PDFs';
      console.log(`🔄 Starting background loading for remaining ${remainingText}`);
      loadRemainingBatches(normalizedObjectName, objectName, 50);
    }
    
    const backgroundText = firstBatch.total > 0 ? `${firstBatch.total - firstBatch.files.length}` : 'unknown number of';
    console.log(`🎯 Progressive loading started: ${pdfsLoaded} PDFs loaded immediately, ${backgroundText} loading in background`);
    return pdfsLoaded;
    
  } catch (error) {
    console.error(`❌ Error in progressive PDF loading:`, error);
    return pdfsLoaded; // Return any cached PDFs that were loaded
  }
};

/**
 * Load remaining PDF batches in background
 * @param {string} normalizedObjectName - Normalized object name  
 * @param {string} objectName - Original object name for display
 * @param {number} batchSize - Size of each batch (50)
 */
const loadRemainingBatches = async (normalizedObjectName, objectName, batchSize) => {
  let offset = batchSize; // Start from second batch (first batch already loaded)
  let hasMore = true;
  let totalLoaded = batchSize; // Track how many we've loaded so far
  
  while (hasMore) {
    try {
      const batch = await getDatasetOutputFilesPaginated(normalizedObjectName, 'ppxf', batchSize, offset);
      console.log(`📄 Background batch loaded: ${batch.files.length} PDFs (${offset}-${offset + batch.files.length - 1})`);
      
      // Add to gallery
      batch.files.forEach(pdfFile => {
        addPdfToGallery(pdfFile, objectName);
      });
      
      // Update cache
      const cached = getCachedPdfs(normalizedObjectName, 'ppxf');
      cached.batches.push({ offset, files: batch.files });
      // Update total in cache if we now know it
      if (batch.total > 0) {
        cached.total = batch.total;
      }
      setCachedPdfs(normalizedObjectName, 'ppxf', cached);
      
      totalLoaded += batch.files.length;
      hasMore = batch.hasMore;
      
      const totalText = batch.total > 0 ? `/${batch.total}` : '';
      console.log(`✅ Background batch complete: ${totalLoaded}${totalText} PDFs loaded`);
      
      // Move to next batch
      offset += batchSize;
      
      // Safety break to prevent infinite loops
      if (offset > 10000) {
        console.warn('⚠️ Safety break: stopped loading after 10000 offset');
        break;
      }
      
    } catch (error) {
      console.error(`❌ Error loading background batch at offset ${offset}:`, error);
      // Stop loading on error to prevent infinite retries
      break;
    }
  }
  
  console.log(`🎉 All background loading complete: ${totalLoaded} PDFs loaded for ${normalizedObjectName}`);
};

/**
 * Try to load PPXF PDF files for H4 map type
 * @param {string} objectName - The original object name
 * @returns {Promise<number>} - Number of PDFs loaded
 */
const tryLoadPpxfPdfFiles = async (objectName) => {
  console.log(`🔄 Loading H4 PDF files for: ${objectName}`);
  const galleryItems = document.getElementById('gallery-items');
  const emptyMessage = document.getElementById('empty-gallery-message');
  
  // Hide empty message if it exists
  if (emptyMessage) {
    emptyMessage.style.display = 'none';
  }
  
  // Clear existing H4 items
  const existingH4Items = galleryItems.querySelectorAll('.gallery-item[data-map-type="h4"]');
  existingH4Items.forEach(item => item.remove());
  console.log(`🧹 Cleared existing H4 items for ${objectName}`);
  
  // Normalize object name for S3 folder lookup (case-insensitive)
  // Convert "ngc7025" or "NGC 7025" to "NGC7025" to match S3 structure
  const normalizedObjectName = objectName
    .toUpperCase()
    .replace(/\s+/g, '')  // Remove spaces
    .replace(/^NGC(\d+)$/, 'NGC$1'); // Ensure NGC format
  
  console.log(`📁 Normalized object name: "${objectName}" → "${normalizedObjectName}"`);
  
  try {
    // Skip full file list fetch to avoid 504 timeout - go straight to progressive loading
    console.log(`📄 Starting progressive PDF loading for ${normalizedObjectName} (bypassing full list to avoid timeout)`);
    
    // Progressive loading with cache (let it discover the total count)
    return await loadPdfsProgressively(normalizedObjectName, objectName, null);
    
    // Hide loader if no other loading is happening
    setTimeout(() => hideGalleryLoader(), 100);
    
    return pdfsLoaded;
    
  } catch (error) {
    console.error(`❌ Error loading H4 PDF files for ${normalizedObjectName} (original: ${objectName}):`, error);
    
    // Hide loader on error too
    setTimeout(() => hideGalleryLoader(), 100);
    
    return 0;
  }
};

/**
 * Generate a PDF thumbnail using PDF.js
 * @param {string} pdfUrl - URL of the PDF file
 * @param {string} cellNumber - Cell number for the thumbnail container ID
 */
const generatePdfThumbnail = async (pdfUrl, cellNumber) => {
  try {
    // Check if PDF.js is available
    if (typeof window.pdfjsLib === 'undefined') {
      console.log('PDF.js not available, keeping placeholder for Cell', cellNumber);
      return;
    }

    const pdf = await window.pdfjsLib.getDocument(pdfUrl).promise;
    const page = await pdf.getPage(1); // Get first page
    
    const container = document.getElementById(`pdf-thumb-${cellNumber}`);
    if (!container) return;
    
    const canvas = container.querySelector('.pdf-thumbnail-canvas');
    const loadingIndicator = container.querySelector('.pdf-loading-indicator');
    
    if (!canvas || !loadingIndicator) return;
    
    const context = canvas.getContext('2d');
    const viewport = page.getViewport({ scale: 0.5 }); // Scale down for thumbnail
    
    canvas.width = viewport.width;
    canvas.height = viewport.height;
    
    await page.render({
      canvasContext: context,
      viewport: viewport
    }).promise;
    
    // Show canvas, hide loading indicator
    canvas.style.display = 'block';
    loadingIndicator.style.display = 'none';
    
    console.log(`✅ Generated thumbnail for Cell ${cellNumber}`);
    
  } catch (error) {
    console.log(`⚠️ Could not generate thumbnail for Cell ${cellNumber}:`, error.message);
    
    // Show error state in the placeholder
    const container = document.getElementById(`pdf-thumb-${cellNumber}`);
    if (container) {
      const loadingIndicator = container.querySelector('.pdf-loading-indicator');
      if (loadingIndicator) {
        loadingIndicator.innerHTML = `
          <span class="cell-label">Cell ${cellNumber}</span>
          <div class="loading-text" style="color: #ff6b6b;">Preview failed</div>
        `;
      }
    }
  }
};

/**
 * Load PDF.js library dynamically if not already loaded
 */
const loadPdfJs = () => {
  if (typeof window.pdfjsLib !== 'undefined') {
    return Promise.resolve();
  }
  
  return new Promise((resolve, reject) => {
    const script = document.createElement('script');
    script.src = 'https://cdnjs.cloudflare.com/ajax/libs/pdf.js/3.11.174/pdf.min.js';
    script.onload = () => {
      window.pdfjsLib.GlobalWorkerOptions.workerSrc = 'https://cdnjs.cloudflare.com/ajax/libs/pdf.js/3.11.174/pdf.worker.min.js';
      resolve();
    };
    script.onerror = reject;
    document.head.appendChild(script);
  });
};

// Load PDF.js when the module loads
loadPdfJs().catch(() => {
  console.log('PDF.js could not be loaded, thumbnails will show placeholders');
});

/**
 * Show loading indicator in gallery
 * @param {HTMLElement} galleryItems - Gallery container element
 * @param {string} objectName - Object name being loaded
 */
const showGalleryLoader = (galleryItems, objectName) => {
  // Remove any existing loader
  const existingLoader = galleryItems.querySelector('.gallery-loader');
  if (existingLoader) {
    existingLoader.remove();
  }

  const loaderDiv = document.createElement('div');
  loaderDiv.className = 'gallery-loader';
  loaderDiv.style.cssText = 'display: flex; justify-content: center; align-items: center; width: 100%; height: 200px;';
  loaderDiv.innerHTML = `
    <div class="astro-loading-container" style="display: flex; flex-direction: column; justify-content: center; align-items: center; text-align: center; padding: 2rem;">
      <div class="astro-loader-galaxy" style="width: 32px; height: 32px;"></div>
      <div class="astro-loading-text" style="font-size: 14px; margin-top: 0.5rem;">Loading maps for ${objectName}...</div>
    </div>
  `;
  
  galleryItems.appendChild(loaderDiv);
};

/**
 * Hide loading indicator from gallery
 */
const hideGalleryLoader = () => {
  const galleryItems = document.getElementById('gallery-items');
  if (galleryItems) {
    const loader = galleryItems.querySelector('.gallery-loader');
    if (loader) {
      loader.remove();
    }
  }
};

// Make gallery functions available globally for sidebar integration
window.addMapToGallery = addMapToGallery;
window.removeMapFromGallery = removeMapFromGallery;
window.clearGallery = clearGallery;
window.loadObjectImages = loadObjectImages;

Gallery.propTypes = {
  aladinInstance: PropTypes.object, // Can be null during initialization
};

export default Gallery; 
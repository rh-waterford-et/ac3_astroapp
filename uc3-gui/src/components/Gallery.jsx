import React, { useEffect } from 'react';

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
            <p>Select options from the sidebar and navigate to a celestial object to view maps</p>
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
  console.log('Gallery controls set up');
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
        console.log('📍 View changed, checking if still at object coordinates');
        loadObjectImages(window.currentLoadedObject);
      }
    }, 500); // Wait 500ms after view stops changing
  });
};

/**
 * Set up image modal functionality
 */
const setupImageModal = () => {
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
  
  // Setup modal dragging
  setupModalDrag();
};

/**
 * Setup modal drag functionality
 */
const setupModalDrag = () => {
  const modalHeader = document.querySelector('.modal-header');
  const modalContent = document.querySelector('.modal-content');
  
  if (!modalHeader || !modalContent) return;
  
  let isDragging = false;
  let startX = 0;
  let startY = 0;
  let initialX = 0;
  let initialY = 0;
  
  // Initialize modal position
  modalContent._currentX = 0;
  modalContent._currentY = 0;
  
  const updateModalPosition = () => {
    modalContent.style.transform = `translate(${modalContent._currentX}px, ${modalContent._currentY}px)`;
  };
  
  const handleMouseDown = (e) => {
    // Only allow dragging if clicking on the header (not close button, transparency controls, or nav buttons)
    if (e.target.closest('.modal-close') || e.target.closest('.transparency-control') || e.target.closest('.modal-nav-buttons')) return;
    
    isDragging = true;
    startX = e.clientX;
    startY = e.clientY;
    initialX = modalContent._currentX;
    initialY = modalContent._currentY;
    
    modalContent.classList.add('dragging');
    document.body.style.userSelect = 'none';
    
    e.preventDefault();
  };
  
  const handleMouseMove = (e) => {
    if (!isDragging) return;
    
    const deltaX = e.clientX - startX;
    const deltaY = e.clientY - startY;
    
    modalContent._currentX = initialX + deltaX;
    modalContent._currentY = initialY + deltaY;
    
    // Keep modal within viewport bounds
    const rect = modalContent.getBoundingClientRect();
    const viewportWidth = window.innerWidth;
    const viewportHeight = window.innerHeight;
    
    // Prevent modal from going too far off screen
    const minX = -rect.width + 100; // Keep at least 100px visible
    const maxX = viewportWidth - 100;
    const minY = -rect.height + 100;
    const maxY = viewportHeight - 100;
    
    modalContent._currentX = Math.max(minX, Math.min(maxX, modalContent._currentX));
    modalContent._currentY = Math.max(minY, Math.min(maxY, modalContent._currentY));
    
    updateModalPosition();
    e.preventDefault();
  };
  
  const handleMouseUp = () => {
    if (!isDragging) return;
    
    isDragging = false;
    modalContent.classList.remove('dragging');
    document.body.style.userSelect = '';
  };
  
  // Touch events for mobile
  const handleTouchStart = (e) => {
    if (e.target.closest('.modal-close') || e.target.closest('.transparency-control') || e.target.closest('.modal-nav-buttons')) return;
    
    const touch = e.touches[0];
    isDragging = true;
    startX = touch.clientX;
    startY = touch.clientY;
    initialX = modalContent._currentX;
    initialY = modalContent._currentY;
    
    modalContent.classList.add('dragging');
    
    e.preventDefault();
  };
  
  const handleTouchMove = (e) => {
    if (!isDragging) return;
    
    const touch = e.touches[0];
    const deltaX = touch.clientX - startX;
    const deltaY = touch.clientY - startY;
    
    modalContent._currentX = initialX + deltaX;
    modalContent._currentY = initialY + deltaY;
    
    // Keep modal within viewport bounds
    const rect = modalContent.getBoundingClientRect();
    const viewportWidth = window.innerWidth;
    const viewportHeight = window.innerHeight;
    
    const minX = -rect.width + 100;
    const maxX = viewportWidth - 100;
    const minY = -rect.height + 100;
    const maxY = viewportHeight - 100;
    
    modalContent._currentX = Math.max(minX, Math.min(maxX, modalContent._currentX));
    modalContent._currentY = Math.max(minY, Math.min(maxY, modalContent._currentY));
    
    updateModalPosition();
    e.preventDefault();
  };
  
  const handleTouchEnd = () => {
    isDragging = false;
    modalContent.classList.remove('dragging');
  };
  
  // Add event listeners
  modalHeader.addEventListener('mousedown', handleMouseDown);
  document.addEventListener('mousemove', handleMouseMove);
  document.addEventListener('mouseup', handleMouseUp);
  
  // Touch events
  modalHeader.addEventListener('touchstart', handleTouchStart);
  document.addEventListener('touchmove', handleTouchMove);
  document.addEventListener('touchend', handleTouchEnd);
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
    // Get image source from the item
    const img = item.querySelector('.thumbnail-image');
    if (img) {
      modalImage.src = img.src;
      modalImage.alt = img.alt;
      
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
      
      console.log(`🔄 Navigated to image ${currentImageIndex + 1}/${currentGalleryItems.length}: ${modalTitle.textContent}`);
    }
  }
};

/**
 * Open image modal with full-size image
 * @param {string} imageSrc - The image source
 * @param {string} title - The image title
 * @param {string} objectName - The object name
 * @param {HTMLElement} clickedItem - The gallery item that was clicked (optional)
 */
const openImageModal = (imageSrc, title, objectName, clickedItem = null) => {
  const modal = document.getElementById('image-modal');
  const modalImage = document.getElementById('modal-image');
  const modalTitle = document.getElementById('modal-title');
  const modalObject = document.getElementById('modal-object');
  const modalContent = document.querySelector('.modal-content');
  const modalBody = document.querySelector('.modal-body');
  const transparencySlider = document.getElementById('transparency-slider');
  
  if (modal && modalImage && modalTitle && modalObject && modalContent) {
    modalImage.src = imageSrc;
    modalImage.alt = `${title} for ${objectName}`;
    modalTitle.textContent = title;
    modalObject.textContent = objectName;
    
    // Set up navigation state
    setupNavigationState(clickedItem, imageSrc);
    
    // Reset modal position - center horizontally, raise vertically
    modalContent._currentX = 0;
    modalContent._currentY = -80; // Raise modal 80px above center
    modalContent.style.transform = 'translate(0, -80px)';
    
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
    
    modal.classList.add('active');
    document.body.style.overflow = 'hidden'; // Prevent background scrolling
    
    console.log(`🖼️ Opened modal for ${title} - ${objectName} (${currentImageIndex + 1}/${currentGalleryItems.length})`);
  }
};

/**
 * Set up navigation state for modal
 * @param {HTMLElement} clickedItem - The gallery item that was clicked
 * @param {string} imageSrc - The image source to match
 */
const setupNavigationState = (clickedItem, imageSrc) => {
  // Get all gallery items that have actual images (not placeholders)
  const galleryItems = document.getElementById('gallery-items');
  if (galleryItems) {
    currentGalleryItems = Array.from(galleryItems.querySelectorAll('.gallery-item')).filter(item => {
      const img = item.querySelector('.thumbnail-image');
      return img && img.src && !item.classList.contains('placeholder-item');
    });
    
    // Find the current image index
    if (clickedItem) {
      currentImageIndex = currentGalleryItems.indexOf(clickedItem);
    } else {
      // Fallback: find by image source
      currentImageIndex = currentGalleryItems.findIndex(item => {
        const img = item.querySelector('.thumbnail-image');
        return img && img.src === imageSrc;
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
  const modalContent = document.querySelector('.modal-content');
  const modalBody = document.querySelector('.modal-body');
  
  if (modal) {
    modal.classList.remove('active');
    document.body.style.overflow = ''; // Restore scrolling
    
    // Reset modal position
    if (modalContent) {
      modalContent._currentX = 0;
      modalContent._currentY = -80;
      modalContent.style.transform = 'translate(0, -80px)';
    }
    
    // Reset modal body opacity
    if (modalBody) {
      modalBody.style.opacity = '';
    }
    
    // Clear image source and reset position/zoom
    if (modalImage) {
      modalImage.src = '';
      modalImage.style.transform = 'translate(0, 0) scale(1)';
      modalImage._currentZoom = 1;
      modalImage._currentX = 0;
      modalImage._currentY = 0;
      removeInteractionListeners(modalImage);
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
  const existingItems = galleryItems.querySelectorAll('.gallery-item, .no-images-message');
  console.log(`🧹 Clearing ${existingItems.length} existing items`);
  existingItems.forEach(item => item.remove());
  
  // Also clear any placeholder items that might have been added
  galleryItems.innerHTML = '';
  console.log(`🧹 Gallery cleared completely`);
  
  // Normalize object name for file naming (remove spaces, make lowercase)
  const normalizedName = objectName.replace(/\s+/g, '').toUpperCase();
  
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
  
  let imagesLoaded = 0;
  
  // Try to load each map type, but only if it's checked in the sidebar
  for (const mapType of mapTypes) {
    const checkbox = document.getElementById(mapType.checkboxId);
    
    // Only load image if the corresponding checkbox is checked
    if (checkbox && checkbox.checked) {
      const imageName = `${normalizedName}_${mapType.suffix}`;
      const imageFound = await tryLoadObjectImage(imageName, mapType, objectName);
      if (imageFound) {
        imagesLoaded++;
      }
    }
  }
  
  if (imagesLoaded === 0) {
    // Check if any checkboxes are selected
    const anyChecked = mapTypes.some(mapType => {
      const checkbox = document.getElementById(mapType.checkboxId);
      return checkbox && checkbox.checked;
    });
    
    if (!anyChecked) {
      showNoOptionsSelectedMessage(objectName);
    } else {
      // Checkboxes are selected but no images found
      showNoImagesMessage(objectName);
    }
  } else {
    console.log(`Loaded ${imagesLoaded} images for ${objectName}`);
    
    // Update status
    const statusElement = document.getElementById('current-status');
    if (statusElement) {
      statusElement.textContent = `Loaded ${imagesLoaded} maps for ${objectName} - click on an image to select`;
    }
  }
};

/**
 * Try to load an image for a specific object and map type
 * @param {string} imageName - The expected image name (e.g., 'NGC7025_stellar_velocity')
 * @param {Object} mapType - Map type configuration object
 * @param {string} objectName - The original object name for display
 * @returns {Promise<boolean>} - True if image was found and loaded
 */
const tryLoadObjectImage = async (imageName, mapType, objectName) => {
  // For now, we'll check if the image exists by trying to load it
  // In the future, this will be replaced with an API call
  
  // Try different file extensions
  const extensions = ['jpg', 'jpeg', 'png', 'fits'];
  
  for (const ext of extensions) {
    const imagePath = `/src/assets/${imageName}.${ext}`;
    
    try {
      // Check if image exists
      const imageExists = await checkImageExists(imagePath);
      if (imageExists) {
        addImageToGallery(imagePath, mapType, objectName);
        return true;
      }
    } catch (error) {
      // Continue to next extension
    }
  }
  
     return false;
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
  console.log(`📦 Adding placeholder: ${label} (${mapType})`);
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
    
    // Show empty message if no items left
    const remainingItems = galleryItems.querySelectorAll('.gallery-item');
    if (remainingItems.length === 0) {
      showEmptyGalleryMessage();
    }
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
      <p>Select options from the sidebar and navigate to a celestial object to view maps</p>
    `;
    galleryItems.appendChild(newEmptyMessage);
  }
};

// Make gallery functions available globally for sidebar integration
window.addMapToGallery = addMapToGallery;
window.removeMapFromGallery = removeMapFromGallery;
window.clearGallery = clearGallery;
window.loadObjectImages = loadObjectImages;

export default Gallery; 
// Core Aladin utility functions - NO DOM DEPENDENCIES

// Load external script utility
export const loadScript = (src) => {
  return new Promise((resolve, reject) => {
    if (document.querySelector(`script[src="${src}"]`)) {
      resolve();
      return;
    }

    const script = document.createElement('script');
    script.src = src;
    script.onload = resolve;
    script.onerror = reject;
    document.head.appendChild(script);
  });
};

// Pure Aladin coordinate hiding utilities
export const hideCoordinateElements = () => {
  try {
    // Hide coordinate box and position info
    const coordElements = document.querySelectorAll('.aladin-location-text, .aladin-coord-text, .aladin-statusBar, .aladin-box-coord');
    coordElements.forEach(el => {
      el.style.display = 'none';
    });
    
    // Also try to hide any elements with coordinate-related classes
    const possibleCoordElements = document.querySelectorAll('[class*="coord"], [class*="position"], [class*="location"]');
    possibleCoordElements.forEach(el => {
      if (el.textContent && el.textContent.includes('°')) {
        el.style.display = 'none';
      }
    });
  } catch (error) {
    // Silent handling
  }
};

export const hideCoordinateFrames = (aladin) => {
  try {
    if (aladin.hideCooFrame) aladin.hideCooFrame();
    if (aladin.hideFrame) aladin.hideFrame();
  } catch (error) {
    // Silent handling  
  }
};

// Galaxy search handler - PURE FUNCTION (no DOM queries)
export const handleGalaxySearch = (aladin, input, gallery = null) => {
  if (!input || !aladin) {
    return;
  }

  // Clean input
  const cleanInput = input.trim();
  if (!cleanInput) {
    return;
  }

  // Use Aladin's gotoObject method
  aladin.gotoObject(cleanInput, {
    success: (coords) => {
      // Only block if currentLoadedObject is set to a DIFFERENT object  
      if (window.currentLoadedObject && window.currentLoadedObject !== cleanInput) {
        return;
      }
      
      if (coords && coords.length >= 2) {
        // Store coordinates and object name globally for backward compatibility
        window.currentObjectCoords = coords;
        window.currentLoadedObject = cleanInput;
        
        // Load gallery images if gallery hook available
        if (gallery?.loadObjectImages) {
          gallery.loadObjectImages(cleanInput);
        }
      }
    },
    error: () => {
      // Error handling could be added here if needed
    }
  });
};
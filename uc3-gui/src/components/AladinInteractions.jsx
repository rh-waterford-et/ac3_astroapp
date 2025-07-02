import { useEffect } from 'react';

const AladinInteractions = ({ aladinInstance }) => {
  useEffect(() => {
    if (!aladinInstance) return;
    
    setupMouseInteractions(aladinInstance);
    setupKeyboardShortcuts(aladinInstance);
    
    // Initial status update
    setTimeout(() => updateStatusDisplays(aladinInstance), 1000);
    
    return () => {
      // Cleanup event listeners
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [aladinInstance]);

  // This component doesn't render anything, it just manages interactions
  return null;
};

/**
 * Global key handler for cleanup
 */
let handleKeyDown = null;

/**
 * Setup mouse interactions for the Aladin Lite instance
 * @param {Object} aladin - The Aladin Lite instance
 */
const setupMouseInteractions = (aladin) => {
  console.log('Setting up mouse interactions...');
  
  // Get the Aladin container element for direct mouse event handling
  const aladinContainer = document.getElementById('aladin-lite-div');
  if (!aladinContainer) {
    console.error('Aladin container not found');
    return;
  }

  // Add some sample catalog data for interaction testing
  try {
    // Create a simple catalog with a few objects for interaction testing
    const catalog = window.A.catalog({
      name: 'Sample Objects',
      color: '#ff0000',
      shape: 'circle',
      sourceSize: 12,
      onClick: 'showPopup'
    });
    
    // Add some sample sources around M101
    const sampleSources = [
      window.A.source(202.4, 54.4, {name: 'Test Object 1', type: 'Star'}),
      window.A.source(202.6, 54.2, {name: 'Test Object 2', type: 'Galaxy'}),
      window.A.source(202.2, 54.6, {name: 'Test Object 3', type: 'Nebula'})
    ];
    
    catalog.addSources(sampleSources);
    aladin.addCatalog(catalog);
    
    console.log('Sample catalog added for interaction testing');
  } catch (error) {
    console.log('Could not add sample catalog:', error);
  }

  // Set up the available event listeners for Aladin Lite
  try {
    // Object hover event (works with catalog objects)
    aladin.on('objectHovered', function(object) {
      if (object) {
        console.log('Hovered over object:', object);
        // Update status if possible
        const statusElement = document.getElementById('current-status');
        if (statusElement && object.ra && object.dec) {
          statusElement.textContent = `Object: ${object.data?.name || 'Unknown'} - RA: ${object.ra.toFixed(4)}°, Dec: ${object.dec.toFixed(4)}°`;
        }
      }
    });

    // Object click event (works with catalog objects)
    aladin.on('objectClicked', function(object) {
      if (object) {
        console.log('Clicked on astronomical object:', object);
        // Center on the object and zoom in
        aladin.gotoRaDec(object.ra, object.dec);
        const currentFov = aladin.getFov();
        aladin.setFov(Math.min(currentFov * 0.6, 1)); // Zoom in but limit to max 1 degree
        
        // Update status
        const statusElement = document.getElementById('current-status');
        if (statusElement) {
          statusElement.textContent = `Clicked: ${object.data?.name || 'Object'} - RA: ${object.ra.toFixed(4)}°, Dec: ${object.dec.toFixed(4)}°`;
        }
      }
    });
    
    console.log('Aladin Lite event listeners setup complete');
  } catch (error) {
    console.error('Error setting up Aladin event listeners:', error);
  }

  // Add direct mouse event listeners to the container for custom behavior
  let isMouseDown = false;
  let mouseDownTime = 0;

  aladinContainer.addEventListener('mousedown', function(event) {
    isMouseDown = true;
    mouseDownTime = Date.now();
  });

  aladinContainer.addEventListener('mouseup', function(event) {
    const clickDuration = Date.now() - mouseDownTime;
    isMouseDown = false;
    
    // Only trigger zoom if it was a quick click (not a drag)
    if (clickDuration < 200) {
      setTimeout(() => {
        // Get click position and convert to world coordinates
        const rect = aladinContainer.getBoundingClientRect();
        const x = event.clientX - rect.left;
        const y = event.clientY - rect.top;
        
        try {
          const worldCoords = aladin.pix2world(x, y);
          if (worldCoords) {
            const [ra, dec] = worldCoords;
            console.log(`Clicked at RA: ${ra.toFixed(4)}°, Dec: ${dec.toFixed(4)}°`);
            
            // Center on clicked position
            aladin.gotoRaDec(ra, dec);
            
            // Zoom in
            const currentFov = aladin.getFov();
            aladin.setFov(currentFov * 0.8);
            
            // Update status displays
            updateStatusDisplays(aladin);
          }
        } catch (error) {
          console.log('Could not convert pixel to world coordinates');
          // Just zoom in at current center
          const currentFov = aladin.getFov();
          aladin.setFov(currentFov * 0.8);
          updateStatusDisplays(aladin);
        }
      }, 50); // Small delay to ensure click is processed
    }
  });

  // Mouse move for coordinate display
  aladinContainer.addEventListener('mousemove', function(event) {
    if (isMouseDown) return; // Skip during drag operations
    
    const rect = aladinContainer.getBoundingClientRect();
    const x = event.clientX - rect.left;
    const y = event.clientY - rect.top;
    
    try {
      const worldCoords = aladin.pix2world(x, y);
      if (worldCoords) {
        const [ra, dec] = worldCoords;
        const statusElement = document.getElementById('current-status');
        if (statusElement) {
          statusElement.textContent = `Mouse: RA ${ra.toFixed(4)}°, Dec ${dec.toFixed(4)}°`;
        }
      }
    } catch (error) {
      // Silently fail - this is expected when mouse is outside valid area
    }
  });

  console.log('Mouse interactions setup complete!');
};

/**
 * Setup keyboard shortcuts for navigation and control
 * @param {Object} aladin - The Aladin Lite instance
 */
const setupKeyboardShortcuts = (aladin) => {
  handleKeyDown = function(event) {
    // Only trigger if not typing in an input field
    if (event.target.tagName === 'INPUT' || event.target.tagName === 'TEXTAREA') {
      return;
    }

    switch(event.key) {
      case '+':
      case '=':
        // Zoom in
        const currentFov = aladin.getFov();
        aladin.setFov(currentFov * 0.8);
        updateStatusDisplays(aladin);
        event.preventDefault();
        break;
      case '-':
        // Zoom out  
        const currentFovOut = aladin.getFov();
        aladin.setFov(currentFovOut * 1.25);
        updateStatusDisplays(aladin);
        event.preventDefault();
        break;
      case 'r':
      case 'R':
        // Reset to initial view
        aladin.gotoRaDec(210.8, 54.3); // M101
        aladin.setFov(1.5);
        updateStatusDisplays(aladin);
        event.preventDefault();
        break;
      case 'g':
      case 'G':
        // Go to a specific object
        const target = prompt('Enter object name or coordinates (e.g., "M31", "Orion", "12.5 34.2"):');
        if (target && target.trim()) {
          try {
            aladin.gotoObject(target.trim());
            console.log('Successfully went to:', target);
            updateStatusDisplays(aladin);
          } catch (error) {
            console.error('Could not go to target:', target, error);
            alert('Could not find target: ' + target);
          }
        }
        event.preventDefault();
        break;
      case 'h':
      case 'H':
        // Show help
        showKeyboardHelp();
        event.preventDefault();
        break;
    }
  };

  document.addEventListener('keydown', handleKeyDown);
  console.log('Keyboard shortcuts set up: +/- (zoom), R (reset), G (goto), H (help)');
};

/**
 * Update the status display elements with current Aladin state
 * @param {Object} aladin - The Aladin Lite instance
 */
const updateStatusDisplays = (aladin) => {
  try {
    // Update position and FOV display
    const position = aladin.getRaDec();
    const fov = aladin.getFov();
    
    const statusElement = document.getElementById('current-status');
    if (statusElement && position) {
      statusElement.textContent = `Center: RA ${position[0].toFixed(4)}°, Dec ${position[1].toFixed(4)}° | FOV: ${fov.toFixed(3)}°`;
    }
  } catch (error) {
    console.log('Could not update status displays:', error);
  }
};

/**
 * Show keyboard help dialog
 */
const showKeyboardHelp = () => {
  const helpText = `
🚀 Universe Explorer - Keyboard Shortcuts:

Navigation:
• Click anywhere to center and zoom in
• + or = : Zoom in
• - : Zoom out
• R : Reset to initial view (M101)
• G : Go to object (enter name or coordinates)

Controls:
• F : Toggle image format (FITS/JPEG/PNG)
• S : Cycle through surveys
• H : Show this help

Mouse:
• Click: Center and zoom in
• Drag: Pan around the sky
• Hover: Show coordinates in status bar

Enjoy exploring the universe! 🌌
  `;
  
  alert(helpText);
};

export default AladinInteractions; 
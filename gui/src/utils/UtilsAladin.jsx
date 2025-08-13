export const hideCoordinateElements = () => {
    try {
      const aladinDiv = document.getElementById('aladin-lite-div');
      if (!aladinDiv) return;

      // Hide coordinate box and position info
      const coordElements = aladinDiv.querySelectorAll('.aladin-location-text, .aladin-coord-text, .aladin-statusBar, .aladin-box-coord');
      coordElements.forEach(el => {
        el.style.display = 'none';
      });
      
      // Also try to hide any elements with coordinate-related classes
      const possibleCoordElements = aladinDiv.querySelectorAll('[class*="coord"], [class*="position"], [class*="location"]');
      possibleCoordElements.forEach(el => {
        if (el.textContent && el.textContent.includes('°')) {
          el.style.display = 'none';
        }
      });
    } catch (error) {
      console.log('Could not hide coordinate elements via DOM:', error);
    }
  };

export const hideCoordinateFrames = (aladin) => {
    try {
      if (aladin.hideCooFrame) aladin.hideCooFrame();
      if (aladin.hideFrame) aladin.hideFrame();
    } catch (error) {
      console.log('Could not hide coordinate frame:', error);
    }
  };


export const handleGalaxyNotFound = (input) => {
  const statusElement = document.getElementById('current-status');
  if (statusElement) {
    statusElement.textContent = `Galaxy "${input}" not found. Try a different name or coordinates.`;
  }
};

export const handleGalaxySearch = (aladin, input) => {
  const searchName = input.trim();
  const displayName = searchName;
  const currentPosition = aladin.getRaDec();

  try {
    // Store current position to detect if gotoObject actually moved
    const beforeRa = currentPosition[0];
    const beforeDec = currentPosition[1];

    aladin.gotoObject(searchName);

    // Wait a moment for the navigation to complete, then check if position changed
    setTimeout(() => {
      const afterPosition = aladin.getRaDec();
      const afterRa = afterPosition[0];
      const afterDec = afterPosition[1];

      // If position didn't change significantly, the object wasn't found
      const raDiff = Math.abs(afterRa - beforeRa);
      const decDiff = Math.abs(afterDec - beforeDec);

      if (raDiff < 0.01 && decDiff < 0.01) {
        // Position didn't change - object not found
        handleGalaxyNotFound(input);
      } else {
        // Position changed - Object found, Update status bar
        const statusElement = document.getElementById('current-status');
        if (statusElement) {
          statusElement.textContent = `Viewing: ${displayName}`;
        }

        // Store the current object and coordinates, then load images
        window.currentLoadedObject = displayName;
        window.currentObjectCoords = afterPosition;
        if (window.loadObjectImages) {
          window.loadObjectImages(displayName);
        }

        // Clear the input on successful search
        const galaxySearchInput = document.getElementById('galaxy-search');
        if (galaxySearchInput) {
          galaxySearchInput.value = '';
        }
      }
    }, 2000); // Wait 2 seconds for the object resolution to complete
  } catch (error) {
    handleGalaxyNotFound(input);
  }
};

export const applyImageFormat = (aladin, format) => {
  try {
    const currentSurvey = document.getElementById('survey-select')?.value || 'P/DSS2/color';
    
    // For FITS format, we use the survey as-is
    // For other formats, we might need to modify the survey ID
    let surveyWithFormat = currentSurvey;
    
    switch (format) {
      case 'fits':
        // FITS is the default high-quality format
        break;
      case 'jpeg':
        // Some surveys support format specification
        if (!currentSurvey.includes('?')) {
          surveyWithFormat = `${currentSurvey}?format=jpeg`;
        }
        break;
      case 'png':
        if (!currentSurvey.includes('?')) {
          surveyWithFormat = `${currentSurvey}?format=png`;
        }
        break;
    }
    
    aladin.setImageSurvey(surveyWithFormat);
    
    // Update status
    const statusElement = document.getElementById('current-status');
    if (statusElement) {
      statusElement.textContent = `Format: ${format.toUpperCase()} | Survey: ${currentSurvey}`;
    }
  } catch (error) {
    console.error('Error applying image format:', error);
  }
};

export const setupKeyboardControls = (aladin) => {
  document.addEventListener('keydown', (event) => {
    // Only trigger if not typing in an input field
    if (event.target.tagName === 'INPUT' || event.target.tagName === 'TEXTAREA') {
      return;
    }

    switch (event.key.toLowerCase()) {
      case 'f':
        event.preventDefault();
        toggleImageFormat(aladin);
        break;
      case 's':
        event.preventDefault();
        cycleSurvey(aladin);
        break;
    }
  });
};

export const toggleImageFormat = (aladin) => {
  const formatSelect = document.getElementById('format-select');
  if (!formatSelect) return;

  const formats = ['fits', 'jpeg', 'png'];
  const currentIndex = formats.indexOf(formatSelect.value);
  const nextIndex = (currentIndex + 1) % formats.length;
  
  formatSelect.value = formats[nextIndex];
  applyImageFormat(aladin, formats[nextIndex]);
};

export const cycleSurvey = (aladin) => {
  const surveySelect = document.getElementById('survey-select');
  if (!surveySelect) return;

  const currentIndex = surveySelect.selectedIndex;
  const nextIndex = (currentIndex + 1) % surveySelect.options.length;
  
  surveySelect.selectedIndex = nextIndex;
  
  // Trigger change event
  const changeEvent = new Event('change');
  surveySelect.dispatchEvent(changeEvent);
};


export const setupSearchControls = (aladin) => {
  const searchGalaxyBtn = document.getElementById('search-galaxy-btn');
  const galaxySearchInput = document.getElementById('galaxy-search');
  
  if (searchGalaxyBtn && galaxySearchInput) {
    // Remove existing event listeners to prevent duplicates
    const newSearchBtn = searchGalaxyBtn.cloneNode(true);
    const newSearchInput = galaxySearchInput.cloneNode(true);
    
    searchGalaxyBtn.parentNode.replaceChild(newSearchBtn, searchGalaxyBtn);
    galaxySearchInput.parentNode.replaceChild(newSearchInput, galaxySearchInput);
    
    // Add fresh event listeners
    newSearchBtn.addEventListener('click', () => {
      const input = newSearchInput.value.trim();
      if (input) {
        handleGalaxySearch(aladin, input);
      } else {
        console.log('No input provided for search');
      }
    });

    // Enter key support for galaxy search
    newSearchInput.addEventListener('keypress', (event) => {
      if (event.key === 'Enter') {
        newSearchBtn.click();
      }
    });
  }
};

export const setupControls = (aladin) => {
  // Format selection
  const formatSelect = document.getElementById('format-select');
  if (formatSelect) {
    if (!formatSelect.dataset.listenersAttached) {
      const newSelect = formatSelect.cloneNode(true);
      formatSelect.parentNode.replaceChild(newSelect, formatSelect);
      newSelect.addEventListener('change', (event) => {
        applyImageFormat(aladin, event.target.value);
      });
      newSelect.dataset.listenersAttached = 'true';
    }
  }

  // Survey selection
  const surveySelect = document.getElementById('survey-select');
  if (surveySelect) {
    if (!surveySelect.dataset.listenersAttached) {
      const newSurvey = surveySelect.cloneNode(true);
      surveySelect.parentNode.replaceChild(newSurvey, surveySelect);
      newSurvey.addEventListener('change', (event) => {
        aladin.setImageSurvey(event.target.value);
        const statusElement = document.getElementById('current-status');
        if (statusElement) {
          const format = document.getElementById('format-select')?.value || 'fits';
          statusElement.textContent = `Format: ${format.toUpperCase()} | Survey: ${event.target.value}`;
        }
      });
      newSurvey.dataset.listenersAttached = 'true';
    }
  }

  setupSearchControls(aladin);
};

export const loadScript = (src) => {
  return new Promise((resolve, reject) => {
    const script = document.createElement('script');
    script.src = src;
    script.async = true;
    script.onload = () => resolve();
    script.onerror = (error) => reject(new Error(`Failed to load script: ${src}`));
    document.head.appendChild(script);
  });
};

const UtilsAladin = {
  hideCoordinateElements,
  hideCoordinateFrames,
  setupKeyboardControls,
  setupControls,
  handleGalaxySearch,
  applyImageFormat,
  toggleImageFormat,
  cycleSurvey,
  setupSearchControls,
  loadScript,
};

export default UtilsAladin;
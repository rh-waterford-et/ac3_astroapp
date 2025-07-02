import { useState, useEffect } from 'react';
import './App.css';
import Sidebar from './components/Sidebar';
import Gallery from './components/Gallery';
import StatusBar from './components/StatusBar';
import AladinInteractions from './components/AladinInteractions';
import PipelineMonitor from './components/PipelineMonitor';
import logoImage from './assets/AC3-LogoConFrase.jpg';

function App() {
  const [aladinInstance, setAladinInstance] = useState(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState(null);
  const [scriptLoaded, setScriptLoaded] = useState(false);
  const [activeTab, setActiveTab] = useState('maps'); // 'maps' or 'pipeline'
  const [selectedApp, setSelectedApp] = useState('starlight'); // 'starlight', 'ppxf', 'steckmap'

  useEffect(() => {
    loadAladinScript();
  }, []);

  useEffect(() => {
    if (scriptLoaded) {
      initializeAladin();
    }
  }, [scriptLoaded]);

  useEffect(() => {
    if (!aladinInstance) return;
    
    setupControls(aladinInstance);
    setupKeyboardControls(aladinInstance);
  }, [aladinInstance]);

  /**
   * Load the Aladin Lite v3 script dynamically
   */
  const loadAladinScript = async () => {
    try {
      // Check if Aladin is already loaded
      if (window.A) {
        setScriptLoaded(true);
        return;
      }

      console.log('Loading Aladin Lite v3 script...');

      // Load jQuery first (required for Aladin Lite)
      if (!window.jQuery && !window.$) {
        await loadScript('https://code.jquery.com/jquery-3.6.0.min.js');
        console.log('jQuery loaded successfully');
      }

      // Load Aladin Lite v3
      await loadScript('https://aladin.cds.unistra.fr/AladinLite/api/v3/latest/aladin.js');
      console.log('Aladin Lite v3 script loaded successfully');

      setScriptLoaded(true);
    } catch (error) {
      console.error('Failed to load Aladin Lite script:', error);
      setError('Failed to load Aladin Lite script: ' + error.message);
      setIsLoading(false);
    }
  };

  /**
   * Helper function to load a script dynamically
   */
  const loadScript = (src) => {
    return new Promise((resolve, reject) => {
      const script = document.createElement('script');
      script.src = src;
      script.async = true;
      
      script.onload = () => resolve();
      script.onerror = (error) => reject(new Error(`Failed to load script: ${src}`));
      
      document.head.appendChild(script);
    });
  };

  /**
   * Initialize Aladin Lite v3
   */
  const initializeAladin = async () => {
    try {
      console.log('Initializing Aladin Lite v3...');
      
      // Wait for Aladin to be available
      if (typeof window.A === 'undefined') {
        throw new Error('Aladin Lite script not loaded');
      }

      // Initialize Aladin v3
      await window.A.init.then(() => {
        console.log('Aladin Lite v3 initialized successfully');
        
        // Create Aladin instance
        const aladin = window.A.aladin('#aladin-lite-div', {
          survey: 'P/DSS2/color',
          fov: 1.5,
          projection: 'SIN',
          cooFrame: 'ICRS',
          showReticle: true,
          showZoomControl: false,
          showFullscreenControl: false,
          showLayersControl: false,
          showGotoControl: false,
          showShareControl: false,
          showCatalogControl: false,
          showFrame: false,
          showCooGrid: false,
          showProjectionControl: false,
          showSimbadPointerControl: false,
          showCooGridControl: false,
          fullScreen: false,
          reticleColor: '#ff89ba',
          reticleSize: 22,
          log: true
        });

        console.log('Aladin instance created:', aladin);
        setAladinInstance(aladin);
        
        // Make Aladin instance globally available for coordinate checking
        window.aladinInstance = aladin;
        
        // Hide any coordinate frame displays
        try {
          if (aladin.hideCooFrame) aladin.hideCooFrame();
          if (aladin.hideFrame) aladin.hideFrame();
        } catch (error) {
          console.log('Could not hide coordinate frame:', error);
        }
        
        // Additional coordinate display hiding
        setTimeout(() => {
          try {
            // Hide coordinate display elements via DOM manipulation
            const aladinDiv = document.getElementById('aladin-lite-div');
            if (aladinDiv) {
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
            }
          } catch (error) {
            console.log('Could not hide coordinate elements via DOM:', error);
          }
        }, 1000);
        
        setIsLoading(false);
      });

    } catch (error) {
      console.error('Error initializing Aladin:', error);
      setError(error.message);
      setIsLoading(false);
    }
  };

  // Error handling moved to be inline within the Aladin display area

  return (
    <div className="app">
      <header className="app-header">
        <div className="header-left">
          <div className="app-logo">
            <img src={logoImage} alt="AC³ Logo" className="logo-image" />
          </div>
          {/* Tab Navigation */}
          <div className="tab-navigation">
            <button 
              className={`tab-button ${activeTab === 'maps' ? 'active' : ''}`}
              onClick={() => setActiveTab('maps')}
            >
              Maps
            </button>
            <button 
              className={`tab-button ${activeTab === 'pipeline' ? 'active' : ''}`}
              onClick={() => setActiveTab('pipeline')}
            >
              Pipeline
            </button>
          </div>
          
          {/* App Selection for Pipeline */}
          {activeTab === 'pipeline' && (
            <div className="app-selection">
              <button 
                className={`app-btn ${selectedApp === 'starlight' ? 'active' : ''}`}
                onClick={() => setSelectedApp('starlight')}
              >
                Starlight
              </button>
              <button 
                className={`app-btn ${selectedApp === 'ppxf' ? 'active' : ''} disabled`}
                onClick={() => setSelectedApp('ppxf')}
                disabled
              >
                PPXF
              </button>
              <button 
                className={`app-btn ${selectedApp === 'steckmap' ? 'active' : ''} disabled`}
                onClick={() => setSelectedApp('steckmap')}
                disabled
              >
                SteckMap
              </button>
            </div>
          )}
        </div>
        
        {/* Integrated Controls */}
        {activeTab === 'maps' && (
          <div className="header-controls">
            <div className="header-control-group">
              <div className="select-wrapper">
                <select id="format-select" defaultValue="fits" className="header-select">
                  <option value="fits">FITS</option>
                  <option value="jpeg">JPEG</option>
                  <option value="png">PNG</option>
                </select>
                <div className="select-arrow">
                  <svg width="10" height="6" viewBox="0 0 12 8" fill="none">
                    <path d="M1 1L6 6L11 1" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                  </svg>
                </div>
              </div>
            </div>

            <div className="header-control-group">
              <div className="select-wrapper">
                <select id="survey-select" defaultValue="P/DSS2/color" className="header-select">
                  <option value="P/DSS2/color">DSS2 Color</option>
                  <option value="P/2MASS/color">2MASS</option>
                  <option value="P/allWISE/color">AllWISE</option>
                  <option value="P/SDSS9/color">SDSS9</option>
                  <option value="P/GLIMPSE360">GLIMPSE</option>
                </select>
                <div className="select-arrow">
                  <svg width="10" height="6" viewBox="0 0 12 8" fill="none">
                    <path d="M1 1L6 6L11 1" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                  </svg>
                </div>
              </div>
            </div>

            <div className="header-control-group">
              <div className="header-search-input-wrapper">
                <input
                  type="text"
                  id="galaxy-search"
                  className="header-search-input"
                  placeholder="Enter galaxy name..."
                />
                <div className="header-search-icon">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none">
                    <circle cx="11" cy="11" r="8" stroke="currentColor" strokeWidth="2"/>
                    <path d="m21 21-4.35-4.35" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                  </svg>
                </div>
              </div>
              <button id="search-galaxy-btn" className="header-search-btn">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none">
                  <circle cx="11" cy="11" r="8" stroke="currentColor" strokeWidth="2"/>
                  <path d="m21 21-4.35-4.35" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                </svg>
              </button>
            </div>
          </div>
        )}
      </header>

      {/* Background Aladin Map */}
      <div className="aladin-background">
        {isLoading && !error && (
          <div className="loading-overlay">
            <div className="loading-spinner"></div>
            <p>
              {!scriptLoaded 
                ? 'Loading Aladin Lite v3 script...' 
                : 'Initializing Aladin Lite v3...'
              }
            </p>
          </div>
        )}
        {error && (
          <div className="error-overlay" style={{
            position: 'absolute',
            top: '45%',
            left: 'calc(50% + 7.5rem)',
            transform: 'translate(-50%, -50%)',
            backgroundColor: '#2D3748',
            border: '2px solid #FC8181',
            borderRadius: '8px',
            padding: '20px',
            maxWidth: '400px',
            textAlign: 'center',
            zIndex: 1000,
            boxShadow: '0 4px 12px rgba(0, 0, 0, 0.3)'
          }}>
            <div style={{
              color: '#FC8181',
              fontSize: '18px',
              fontWeight: '600',
              marginBottom: '12px',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              gap: '8px'
            }}>
              <span>⚠️</span>
              Aladin Loading Error
            </div>
            <p style={{
              color: '#E2E8F0',
              fontSize: '14px',
              marginBottom: '12px',
              lineHeight: '1.4'
            }}>
              {error}
            </p>
            <p style={{
              color: '#A0AEC0',
              fontSize: '12px',
              marginBottom: '16px'
            }}>
              Please check your internet connection and WebGL2 support.
            </p>
            <button 
              onClick={() => {
                setError(null);
                setIsLoading(true);
                setScriptLoaded(false);
                loadAladinScript();
              }}
              style={{
                backgroundColor: '#4FD1C5',
                color: '#1A202C',
                border: 'none',
                borderRadius: '4px',
                padding: '8px 16px',
                fontSize: '14px',
                fontWeight: '600',
                cursor: 'pointer',
                transition: 'background-color 0.2s ease'
              }}
              onMouseEnter={(e) => {
                e.target.style.backgroundColor = '#38B2AC';
              }}
              onMouseLeave={(e) => {
                e.target.style.backgroundColor = '#4FD1C5';
              }}
            >
              Retry
            </button>
          </div>
        )}
        <div id="aladin-lite-div"></div>
      </div>

      <main className="app-main">
        <div className="content-layout">
          {activeTab === 'maps' && (
            <>
              {/* Right Sidebar */}
              <Sidebar aladinInstance={aladinInstance} />
            </>
          )}

          {activeTab === 'pipeline' && (
            <div className="pipeline-full-view">
              {/* Pipeline Monitor with full space */}
              <PipelineMonitor selectedApp={selectedApp} />
            </div>
          )}
        </div>
      </main>

      {/* Bottom Overlays */}
      {activeTab === 'maps' && (
        <>
          {/* Bottom Gallery */}
          <div className="bottom-overlay gallery-overlay">
            <Gallery aladinInstance={aladinInstance} />
          </div>

          {/* Status Bar */}
          <div className="bottom-overlay status-overlay">
            <StatusBar />
          </div>
        </>
      )}
      
      {/* Interactions Handler (invisible component) */}
      <AladinInteractions aladinInstance={aladinInstance} />
      
      {/* Image Modal - Positioned at app level for full overlay */}
      <div className="image-modal" id="image-modal">
        <div className="modal-backdrop" id="modal-backdrop"></div>
        <div className="modal-content" id="modal-content">
          <div className="modal-header">
            <span className="modal-object-code" id="modal-object">Object</span>
            <h3 className="modal-title" id="modal-title">Image Title</h3>
            <div className="modal-nav-buttons">
              <button className="modal-nav-btn modal-prev" id="modal-prev" title="Previous image">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
                  <path d="M15 18l-6-6 6-6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                </svg>
              </button>
              <button className="modal-nav-btn modal-next" id="modal-next" title="Next image">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
                  <path d="M9 18l6-6-6-6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                </svg>
              </button>
            </div>
            <div className="modal-controls">
              <div className="transparency-control">
                <label htmlFor="transparency-slider" className="transparency-label">
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
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
                  defaultValue="95"
                  onChange={(e) => {
                    const modalBody = document.querySelector('.modal-body');
                    if (modalBody) {
                      const opacity = e.target.value / 100;
                      modalBody.style.opacity = opacity;
                    }
                  }}
                />
              </div>
              <button className="modal-close" id="modal-close">×</button>
            </div>
          </div>
          <div className="modal-body">
            <img className="modal-image" id="modal-image" alt="" />
          </div>
        </div>
      </div>
    </div>
  );
}

/**
 * Set up controls functionality
 * @param {Object} aladin - The Aladin Lite instance
 */
const setupControls = (aladin) => {
  // Format selection
  const formatSelect = document.getElementById('format-select');
  if (formatSelect) {
    formatSelect.addEventListener('change', (event) => {
      applyImageFormat(aladin, event.target.value);
    });
  }

  // Survey selection
  const surveySelect = document.getElementById('survey-select');
  if (surveySelect) {
    surveySelect.addEventListener('change', (event) => {
      aladin.setImageSurvey(event.target.value);
      
      // Update status
      const statusElement = document.getElementById('current-status');
      if (statusElement) {
        const format = formatSelect?.value || 'fits';
        statusElement.textContent = `Format: ${format.toUpperCase()} | Survey: ${event.target.value}`;
      }
      
      console.log(`Survey changed to: ${event.target.value}`);
    });
  }

  // Galaxy search
  const searchGalaxyBtn = document.getElementById('search-galaxy-btn');
  const galaxySearchInput = document.getElementById('galaxy-search');
  
  if (searchGalaxyBtn && galaxySearchInput) {
    searchGalaxyBtn.addEventListener('click', () => {
      const input = galaxySearchInput.value.trim();
      console.log('Search button clicked, input:', input);
      if (input) {
        handleGalaxySearch(aladin, input);
      } else {
        console.log('No input provided for search');
      }
    });
  }

  // Enter key support for galaxy search
  if (galaxySearchInput) {
    galaxySearchInput.addEventListener('keypress', (event) => {
      if (event.key === 'Enter') {
        console.log('Enter key pressed in search input');
        searchGalaxyBtn?.click();
      }
    });
  }

  console.log('Controls set up');
};

/**
 * Set up keyboard controls
 * @param {Object} aladin - The Aladin Lite instance
 */
const setupKeyboardControls = (aladin) => {
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

  console.log('Keyboard controls set up (F = format toggle, S = survey cycle)');
};

/**
 * Apply image format to current survey
 * @param {Object} aladin - The Aladin Lite instance
 * @param {string} format - The format to apply ('fits', 'jpeg', 'png')
 */
const applyImageFormat = (aladin, format) => {
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
    
    console.log(`Applied ${format} format to survey: ${currentSurvey}`);
  } catch (error) {
    console.error('Error applying image format:', error);
  }
};

/**
 * Toggle between image formats
 * @param {Object} aladin - The Aladin Lite instance
 */
const toggleImageFormat = (aladin) => {
  const formatSelect = document.getElementById('format-select');
  if (!formatSelect) return;

  const formats = ['fits', 'jpeg', 'png'];
  const currentIndex = formats.indexOf(formatSelect.value);
  const nextIndex = (currentIndex + 1) % formats.length;
  
  formatSelect.value = formats[nextIndex];
  applyImageFormat(aladin, formats[nextIndex]);
};

/**
 * Cycle through available surveys
 * @param {Object} aladin - The Aladin Lite instance
 */
const cycleSurvey = (aladin) => {
  const surveySelect = document.getElementById('survey-select');
  if (!surveySelect) return;

  const currentIndex = surveySelect.selectedIndex;
  const nextIndex = (currentIndex + 1) % surveySelect.options.length;
  
  surveySelect.selectedIndex = nextIndex;
  
  // Trigger change event
  const changeEvent = new Event('change');
  surveySelect.dispatchEvent(changeEvent);
};

/**
 * Handle galaxy search input
 * @param {Object} aladin - The Aladin Lite instance
 * @param {string} input - The galaxy name or identifier
 */
const handleGalaxySearch = (aladin, input) => {
  console.log('handleGalaxySearch called with input:', input);
  
  // First try to resolve common galaxy names to their catalog equivalents
  const resolvedName = resolveGalaxyName(input);
  console.log('Resolved name:', resolvedName);
  
  if (resolvedName.coordinates) {
    console.log('Using predefined coordinates for:', resolvedName.displayName);
    // Use predefined coordinates for known galaxies
    aladin.gotoRaDec(resolvedName.coordinates.ra, resolvedName.coordinates.dec);
    console.log(`Navigated to ${resolvedName.displayName} at RA: ${resolvedName.coordinates.ra}°, Dec: ${resolvedName.coordinates.dec}°`);
    
    // Update status
    const statusElement = document.getElementById('current-status');
    if (statusElement) {
      statusElement.textContent = `Viewing: ${resolvedName.displayName}`;
    }
    
    // Store the current object and coordinates, then load images
    window.currentLoadedObject = resolvedName.displayName;
    window.currentObjectCoords = [resolvedName.coordinates.ra, resolvedName.coordinates.dec];
    if (window.loadObjectImages) {
      window.loadObjectImages(resolvedName.displayName);
    }
    
    // Clear the input
    const galaxySearchInput = document.getElementById('galaxy-search');
    if (galaxySearchInput) {
      galaxySearchInput.value = '';
    }
    return;
  }
  
  console.log('Using Aladin object resolution for:', resolvedName.searchName);
  // Try Aladin's object resolution with better error detection
  const currentPosition = aladin.getRaDec();
  console.log('Current position before search:', currentPosition);
  
  try {
    // Store current position to detect if gotoObject actually moved
    const beforeRa = currentPosition[0];
    const beforeDec = currentPosition[1];
    
    aladin.gotoObject(resolvedName.searchName);
    
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
        console.log(`Object "${resolvedName.searchName}" not found - position unchanged`);
        handleGalaxyNotFound(input);
      } else {
        // Position changed - object found
        console.log(`Successfully navigated to galaxy: ${resolvedName.searchName}`);
        
        // Update status
        const statusElement = document.getElementById('current-status');
        if (statusElement) {
          statusElement.textContent = `Viewing: ${resolvedName.displayName}`;
        }
        
        // Store the current object and coordinates, then load images
        window.currentLoadedObject = resolvedName.displayName;
        window.currentObjectCoords = afterPosition;
        if (window.loadObjectImages) {
          window.loadObjectImages(resolvedName.displayName);
        }
        
        // Clear the input on successful search
        const galaxySearchInput = document.getElementById('galaxy-search');
        if (galaxySearchInput) {
          galaxySearchInput.value = '';
        }
      }
    }, 2000); // Wait 2 seconds for the object resolution to complete
  } catch (error) {
    console.error('Error during galaxy search:', error);
    handleGalaxyNotFound(input);
  }
};

/**
 * Parse coordinate input (supports multiple formats)
 * @param {string} input - The coordinate string
 * @returns {Object|null} - {ra, dec} or null if parsing failed
 */
const parseCoordinates = (input) => {
  // Remove extra whitespace and normalize
  const cleaned = input.trim().replace(/\s+/g, ' ');
  
  // Try decimal degrees format: "ra dec" or "ra, dec"
  const decimalMatch = cleaned.match(/^(-?\d+\.?\d*)[,\s]+(-?\d+\.?\d*)$/);
  if (decimalMatch) {
    const ra = parseFloat(decimalMatch[1]);
    const dec = parseFloat(decimalMatch[2]);
    if (ra >= 0 && ra <= 360 && dec >= -90 && dec <= 90) {
      return { ra, dec };
    }
  }

  // Try HMS DMS format: "HH:MM:SS DD:MM:SS" or "HH MM SS DD MM SS"
  const hmsMatch = cleaned.match(/^(\d{1,2}):(\d{2}):(\d{2}\.?\d*)\s+([+-]?\d{1,2}):(\d{2}):(\d{2}\.?\d*)$/);
  if (hmsMatch) {
    const h = parseInt(hmsMatch[1]);
    const m = parseInt(hmsMatch[2]);
    const s = parseFloat(hmsMatch[3]);
    const d = parseInt(hmsMatch[4]);
    const arcm = parseInt(hmsMatch[5]);
    const arcs = parseFloat(hmsMatch[6]);
    
    if (h <= 23 && m <= 59 && s < 60 && Math.abs(d) <= 90 && arcm <= 59 && arcs < 60) {
      const ra = (h + m/60 + s/3600) * 15; // Convert hours to degrees
      const dec = Math.sign(d) * (Math.abs(d) + arcm/60 + arcs/3600);
      return { ra, dec };
    }
  }

  return null;
};

/**
 * Resolve galaxy name to search terms and coordinates
 * @param {string} input - The galaxy name
 * @returns {Object} - {searchName, displayName, coordinates}
 */
const resolveGalaxyName = (input) => {
  // Convert to uppercase for comparison
  const upperInput = input.toUpperCase().trim();
  
  // Common galaxy name mappings with coordinates
  const galaxyDatabase = {
    'M31': { searchName: 'M31', displayName: 'M31 (Andromeda Galaxy)', coordinates: { ra: 10.684, dec: 41.269 } },
    'M33': { searchName: 'M33', displayName: 'M33 (Triangulum Galaxy)', coordinates: { ra: 23.462, dec: 30.660 } },
    'M51': { searchName: 'M51', displayName: 'M51 (Whirlpool Galaxy)', coordinates: { ra: 202.469, dec: 47.195 } },
    'M81': { searchName: 'M81', displayName: 'M81 (Bode\'s Galaxy)', coordinates: { ra: 148.888, dec: 69.065 } },
    'M82': { searchName: 'M82', displayName: 'M82 (Cigar Galaxy)', coordinates: { ra: 148.969, dec: 69.679 } },
    'M87': { searchName: 'M87', displayName: 'M87 (Virgo A)', coordinates: { ra: 187.706, dec: 12.391 } },
    'M101': { searchName: 'M101', displayName: 'M101 (Pinwheel Galaxy)', coordinates: { ra: 210.802, dec: 54.349 } },
    'M104': { searchName: 'M104', displayName: 'M104 (Sombrero Galaxy)', coordinates: { ra: 189.998, dec: -11.623 } },
    'NGC 2903': { searchName: 'NGC 2903', displayName: 'NGC 2903', coordinates: { ra: 143.042, dec: 21.501 } },
    'NGC 4258': { searchName: 'NGC 4258', displayName: 'NGC 4258', coordinates: { ra: 184.740, dec: 47.304 } },
    'NGC 4472': { searchName: 'NGC 4472', displayName: 'NGC 4472', coordinates: { ra: 187.445, dec: 8.000 } },
    'NGC 4594': { searchName: 'NGC 4594', displayName: 'NGC 4594 (Sombrero Galaxy)', coordinates: { ra: 189.998, dec: -11.623 } },
    'NGC 5194': { searchName: 'NGC 5194', displayName: 'NGC 5194 (Whirlpool Galaxy)', coordinates: { ra: 202.469, dec: 47.195 } },
    'NGC 7025': { searchName: 'NGC 7025', displayName: 'NGC 7025', coordinates: { ra: 315.707, dec: 16.085 } },
    'NGC 6027': { searchName: 'NGC 6027', displayName: 'NGC 6027', coordinates: { ra: 239.732, dec: 20.756 } },
    'NGC 2906': { searchName: 'NGC 2906', displayName: 'NGC 2906', coordinates: { ra: 142.875, dec: 8.458 } },
    'IC 1683': { searchName: 'IC 1683', displayName: 'IC 1683', coordinates: { ra: 23.021, dec: 5.168 } },
    'IC1683': { searchName: 'IC 1683', displayName: 'IC 1683', coordinates: { ra: 23.021, dec: 5.168 } },
    'ANDROMEDA': { searchName: 'M31', displayName: 'M31 (Andromeda Galaxy)', coordinates: { ra: 10.684, dec: 41.269 } },
    'WHIRLPOOL': { searchName: 'M51', displayName: 'M51 (Whirlpool Galaxy)', coordinates: { ra: 202.469, dec: 47.195 } },
    'SOMBRERO': { searchName: 'M104', displayName: 'M104 (Sombrero Galaxy)', coordinates: { ra: 189.998, dec: -11.623 } },
    'PINWHEEL': { searchName: 'M101', displayName: 'M101 (Pinwheel Galaxy)', coordinates: { ra: 210.802, dec: 54.349 } },
    'CIGAR': { searchName: 'M82', displayName: 'M82 (Cigar Galaxy)', coordinates: { ra: 148.969, dec: 69.679 } },
    'TRIANGULUM': { searchName: 'M33', displayName: 'M33 (Triangulum Galaxy)', coordinates: { ra: 23.462, dec: 30.660 } }
  };
  
  // Check if input matches any known galaxy
  if (galaxyDatabase[upperInput]) {
    return galaxyDatabase[upperInput];
  }
  
  // Try to match partial names
  for (const [key, value] of Object.entries(galaxyDatabase)) {
    if (key.includes(upperInput) || upperInput.includes(key)) {
      return value;
    }
  }
  
  // If no match found, return the input as searchName
  return {
    searchName: input,
    displayName: input,
    coordinates: null
  };
};

/**
 * Handle case when galaxy is not found
 * @param {string} input - The original search input
 */
const handleGalaxyNotFound = (input) => {
  console.log('Galaxy not found:', input);
  
  const statusElement = document.getElementById('current-status');
  if (statusElement) {
    statusElement.textContent = `Galaxy "${input}" not found. Try a different name or coordinates.`;
  }
  
  // You could also show suggestions here
  // const suggestions = getSuggestions(input);
  // if (suggestions.length > 0) {
  //   console.log('Suggestions:', suggestions);
  // }
};

export default App; 
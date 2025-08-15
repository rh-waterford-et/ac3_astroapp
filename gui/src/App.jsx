import React from 'react';
import { useState, useEffect, useRef } from 'react';
import Sidebar from './components/maps/Sidebar';
import Gallery from './components/maps/Gallery';
import StatusBar from './components/maps/StatusBar';
import ErrorOverlay from './components/maps/ErrorOverlay';
import ImageModal from './components/ui/ImageModal';
import TabNavigation from './components/ui/TabNavigation';
import PipelineAppSelector from './components/pipeline/PipelineAppSelector';
import HeaderControls from './components/maps/HeaderControls';
import { loadScript, hideCoordinateElements, hideCoordinateFrames, setupKeyboardControls } from './utils/UtilsAladin';
import AladinInteractions from './components/maps/AladinInteractions';
import { MAP_CONTROLS, MODAL_DIMENSIONS, TIMEOUTS, RETICLE, DEFAULTS } from './utils/constants';
import DatasetsList from './components/pipeline/DatasetsList';
import logoImage from './assets/AC3-LogoConFrase.jpg';

function App() {
  const [aladinInstance, setAladinInstance] = useState(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState(null);
  const [scriptLoaded, setScriptLoaded] = useState(false);
  const [activeTab, setActiveTab] = useState('maps'); 
  const [selectedApp, setSelectedApp] = useState('starlight'); 
  const modalRndRef = useRef(null);
  const keyboardInitRef = useRef(false);
  const tabTimeoutRef = useRef(null);


  // Make centerModal globally available
  useEffect(() => {
    window.centerModal = centerModal;
  }, []);

  // Add beforeunload event listener to warn about uploads in progress
  useEffect(() => {
    const handleBeforeUnload = (e) => {
      // Check if any uploads are in progress
      const uploadElements = document.querySelectorAll('[data-upload-status="uploading"]');
      if (uploadElements.length > 0) {
        e.preventDefault();
        e.returnValue = 'File uploads are in progress. Are you sure you want to leave?';
        return 'File uploads are in progress. Are you sure you want to leave?';
      }
    };

    window.addEventListener('beforeunload', handleBeforeUnload);
    
    return () => {
      window.removeEventListener('beforeunload', handleBeforeUnload);
    };
  }, []);

  // Sidebar checkbox states
  const [sidebarState, setSidebarState] = useState({
    'map-stellar-velocity': false,
    'map-stellar-velocity-error': false,
    'map-velocity-dispersion': false,
    'map-velocity-dispersion-error': false,
    'map-h3': false,
    'map-h4': false,
    'map-age-weighted': false,
    'map-age-mass-weighted': false,
    'map-metallicity': false,
    'display-grid': false,
    'display-reticle': true, // Default checked
    'display-labels': false,
    'display-healpix': false
  });

  const handleFormatChange = (format) => {
    if (!aladinInstance) return;
    try {
      const currentSurvey = document.getElementById('survey-select')?.value || DEFAULTS.survey;
      let surveyWithFormat = currentSurvey;
      if (format === 'jpeg' && !currentSurvey.includes('?')) surveyWithFormat = `${currentSurvey}?format=jpeg`;
      if (format === 'png' && !currentSurvey.includes('?')) surveyWithFormat = `${currentSurvey}?format=png`;
      aladinInstance.setImageSurvey(surveyWithFormat);
      const statusElement = document.getElementById('current-status');
      if (statusElement) statusElement.textContent = `Format: ${format.toUpperCase()} | Survey: ${currentSurvey}`;
    } catch {}
  };

  const handleSurveyChange = (survey) => {
    if (!aladinInstance) return;
    try {
      aladinInstance.setImageSurvey(survey);
      const statusElement = document.getElementById('current-status');
      if (statusElement) {
        const fmt = document.getElementById('format-select')?.value || 'fits';
        statusElement.textContent = `Format: ${fmt.toUpperCase()} | Survey: ${survey}`;
      }
    } catch {}
  };

  const handleSearch = (value) => {
    const input = value?.trim();
    if (!input || !aladinInstance) return;
    const currentPosition = aladinInstance.getRaDec();
    try {
      const beforeRa = currentPosition[0];
      const beforeDec = currentPosition[1];
      aladinInstance.gotoObject(input);
      setTimeout(() => {
        const afterPosition = aladinInstance.getRaDec();
        const raDiff = Math.abs(afterPosition[0] - beforeRa);
        const decDiff = Math.abs(afterPosition[1] - beforeDec);
        if (raDiff < 0.01 && decDiff < 0.01) {
          const statusElement = document.getElementById('current-status');
          if (statusElement) statusElement.textContent = `Galaxy "${input}" not found. Try a different name or coordinates.`;
        } else {
          const statusElement = document.getElementById('current-status');
          if (statusElement) statusElement.textContent = `Viewing: ${input}`;
          window.currentLoadedObject = input;
          window.currentObjectCoords = afterPosition;
          if (window.loadObjectImages) window.loadObjectImages(input);
          const el = document.getElementById('galaxy-search');
          if (el) el.value = '';
        }
      }, TIMEOUTS.objectResolutionMs);
    } catch {}
  };

  const handleCheckboxChange = (checkboxId, isChecked) => {
    setSidebarState(prev => ({ ...prev, [checkboxId]: isChecked }));
    if (aladinInstance) {
      if (checkboxId === 'display-grid') {
        try { isChecked ? aladinInstance.showCooGrid() : aladinInstance.hideCooGrid(); } catch {}
      } else if (checkboxId === 'display-reticle') {
        try { aladinInstance.showReticle(isChecked); } catch {}
      } else if (checkboxId === 'display-labels') {
        try { aladinInstance.getCatalogs().forEach(c => c.setShowLabels && c.setShowLabels(isChecked)); } catch {}
      } else if (checkboxId === 'display-healpix') {
        try { aladinInstance.showHealpixGrid(isChecked); } catch {}
      }
    }
    const mapControls = MAP_CONTROLS;
    if (mapControls[checkboxId]) {
      const config = mapControls[checkboxId];
      const currentObject = window.currentLoadedObject;
      if (currentObject) {
        if (window.loadObjectImages) window.loadObjectImages(currentObject);
      } else {
        const mapType = checkboxId.replace('map-', '');
        if (isChecked) {
          if (window.addMapToGallery) window.addMapToGallery(mapType, config.label, config.icon);
        } else {
          if (window.removeMapFromGallery) window.removeMapFromGallery(mapType);
        }
      }
    }
  };

  useEffect(() => { loadAladinScript(); }, []);
  useEffect(() => { if (scriptLoaded) initializeAladin(); }, [scriptLoaded]);
  useEffect(() => {
    if (!aladinInstance) return;
    if (!keyboardInitRef.current) { setupKeyboardControls(aladinInstance); keyboardInitRef.current = true; }
  }, [aladinInstance]);

  useEffect(() => {
    if (activeTab === 'maps') {
      if (tabTimeoutRef.current) clearTimeout(tabTimeoutRef.current);
      tabTimeoutRef.current = setTimeout(() => {
        if (aladinInstance) {
          // React handlers now; no setupControls needed
        }
        const currentObject = window.currentLoadedObject;
        const currentCoords = window.currentObjectCoords;
        const galleryItems = document.getElementById('gallery-items');
        if (galleryItems) { galleryItems.querySelectorAll('.gallery-item').forEach(item => item.remove()); }
        if (currentObject) { window.currentLoadedObject = currentObject; window.currentObjectCoords = currentCoords; }
        const mapControls = MAP_CONTROLS;
        if (!currentObject) {
          Object.entries(mapControls).forEach(([checkboxId, config]) => {
            const isChecked = sidebarState[checkboxId];
            if (isChecked) {
              const mapType = checkboxId.replace('map-', '');
              if (window.addMapToGallery) window.addMapToGallery(mapType, config.label, config.icon);
            }
          });
        }
      }, TIMEOUTS.restoreGalleryMs);
    }
    return () => { if (tabTimeoutRef.current) clearTimeout(tabTimeoutRef.current); };
  }, [activeTab, aladinInstance]);

  const setupAladinInstance = (aladin) => {
    setAladinInstance(aladin);
    window.aladinInstance = aladin;
    hideCoordinateFrames(aladin);
    setTimeout(hideCoordinateElements, TIMEOUTS.hideCoordsMs);
    setIsLoading(false);
  };

  const initializeAladin = async () => {
    try {
      if (typeof window.A === 'undefined') throw new Error('Aladin Lite script not loaded');
      await window.A.init.then(() => {
        const aladin = window.A.aladin('#aladin-lite-div', {
          survey: DEFAULTS.survey,
          fov: DEFAULTS.fov,
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
          reticleColor: RETICLE.color,
          reticleSize: RETICLE.size,
          log: true
        });
        setupAladinInstance(aladin);
      });
    } catch (error) {
      setError(error.message);
      setIsLoading(false);
    }
  };

  const loadAladinScript = async () => {
    try {
      if (window.A) { setScriptLoaded(true); return; }
      if (!window.jQuery && !window.$) { await loadScript('https://code.jquery.com/jquery-3.6.0.min.js'); }
      await loadScript('https://aladin.cds.unistra.fr/AladinLite/api/v3/latest/aladin.js');
      setScriptLoaded(true);
    } catch (error) {
      setError('Failed to load Aladin Lite script: ' + error.message);
      setIsLoading(false);
    }
  };

  const centerModal = () => {
    if (modalRndRef.current) {
      const modalWidth = MODAL_DIMENSIONS.widthPx;
      const modalHeight = MODAL_DIMENSIONS.heightPx;
      const centerX = Math.max(0, (window.innerWidth - modalWidth) / 2);
      const centerY = Math.max(0, (window.innerHeight - modalHeight) / 2);
      modalRndRef.current.updatePosition({ x: centerX, y: centerY });
    }
  };

  return (
    <div className="app">
      <header className="app-header">
        <div className="header-left">
          <div className="app-logo">
            <img src={logoImage} alt="AC³ Logo" className="logo-image" />
          </div>
          {/* Tab Navigation */}
          <TabNavigation activeTab={activeTab} onSelect={setActiveTab} />
          {/* App Selection for Pipeline */}
          {activeTab === 'pipeline' && (
            <PipelineAppSelector selectedApp={selectedApp} onSelect={setSelectedApp} />
          )}
        </div>
        {/* Integrated Controls */}
        {activeTab === 'maps' && (
          <HeaderControls
            onFormatChange={handleFormatChange}
            onSurveyChange={handleSurveyChange}
            onSearch={handleSearch}
          />
        )}
      </header>

      {/* Background Aladin Map */}
      <div className="aladin-background">
        {isLoading && !error && (
          <div className="loading-overlay">
            <div className="astro-loader-galaxy"></div>
            <p>
              {!scriptLoaded ? 'Loading Aladin Lite v3 script...' : 'Initializing Aladin Lite v3...'}
            </p>
          </div>
        )}
        {error && (
          <ErrorOverlay
            error={error}
            onRetry={() => {
              setError(null);
              setIsLoading(true);
              setScriptLoaded(false);
              loadAladinScript();
            }}
          />
        )}
        <div id="aladin-lite-div"></div>
      </div>

      <main className="app-main">
        <div className="content-layout">
          {activeTab === 'maps' && (
            <>
              {/* Right Sidebar */}
              <Sidebar 
                aladinInstance={aladinInstance} 
                checkboxStates={sidebarState}
                onCheckboxChange={handleCheckboxChange}
              />
            </>
          )}

          {activeTab === 'pipeline' && (
            <div className="pipeline-full-view">
              {/* Pipeline Monitor with full space */}
              <DatasetsList processorType={selectedApp} />
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
      {/* Image Modal */}
      <ImageModal ref={modalRndRef} width={MODAL_DIMENSIONS.widthPx} height={MODAL_DIMENSIONS.heightPx} />
    </div>
  );
}

export default App; 
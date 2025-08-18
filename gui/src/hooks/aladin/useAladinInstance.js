import { useState, useEffect, useRef } from 'react';
import { loadScript, hideCoordinateElements, hideCoordinateFrames, setupKeyboardControls } from '../../utils/aladin/aladinUtils';
import { TIMEOUTS, RETICLE, DEFAULTS } from '../../utils/constants/constants';

export const useAladinInstance = () => {
  const [aladinInstance, setAladinInstance] = useState(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState(null);
  const [scriptLoaded, setScriptLoaded] = useState(false);
  const keyboardInitRef = useRef(false);

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

  // Load Aladin script on mount
  useEffect(() => { 
    loadAladinScript(); 
  }, []);

  // Initialize Aladin when script loads
  useEffect(() => { 
    if (scriptLoaded) initializeAladin(); 
  }, [scriptLoaded]);

  // Setup keyboard controls when instance is ready
  useEffect(() => {
    if (!aladinInstance) return;
    if (!keyboardInitRef.current) { 
      setupKeyboardControls(aladinInstance); 
      keyboardInitRef.current = true; 
    }
  }, [aladinInstance]);

  const retry = () => {
    setError(null);
    setIsLoading(true);
    setScriptLoaded(false);
    setAladinInstance(null);
    keyboardInitRef.current = false;
    loadAladinScript();
  };

  return {
    aladinInstance,
    isLoading,
    error,
    scriptLoaded,
    retry
  };
}; 
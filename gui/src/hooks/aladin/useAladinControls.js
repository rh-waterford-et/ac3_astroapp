import { useState, useCallback, useRef, useEffect } from 'react';
import { DEFAULTS } from '../../utils/constants/constants';
import { registerObjectCoordinates } from '../../utils/gallery/coordinateRegistry';

export const useAladinControls = (aladinInstance, gallery = null) => {
  // Centralized state - NO MORE DOM QUERIES
  const [survey, setSurvey] = useState(DEFAULTS.survey);
  const [format, setFormat] = useState('fits');
  const [status, setStatus] = useState('');
  const [searchTerm, setSearchTerm] = useState('');
  
  // Keyboard setup ref
  const keyboardInitRef = useRef(false);

  // Survey options for cycling
  const surveys = [
    'P/DSS2/color',
    'P/2MASS/color', 
    'P/allWISE/color',
    'P/SDSS9/color',
    'P/GLIMPSE360'
  ];

  // Format options for cycling  
  const formats = ['fits', 'jpeg', 'png'];

  // Update status helper
  const updateStatus = useCallback((newFormat, newSurvey) => {
    setStatus(`Format: ${newFormat.toUpperCase()} | Survey: ${newSurvey}`);
  }, []);

  // Handle survey change - PURE REACT
  const handleSurveyChange = useCallback((newSurvey) => {
    if (!aladinInstance) return;
    
    try {
      setSurvey(newSurvey);
      aladinInstance.setImageSurvey(newSurvey);
      updateStatus(format, newSurvey);
    } catch (error) {
      // Silent error handling
    }
  }, [aladinInstance, format, updateStatus]);

  // Handle format change - PURE REACT
  const handleFormatChange = useCallback((newFormat) => {
    if (!aladinInstance) return;
    
    try {
      setFormat(newFormat);
      
      let surveyWithFormat = survey;
      
      // Apply format parameter if not already present
      if (newFormat === 'jpeg' && !survey.includes('?')) {
        surveyWithFormat = `${survey}?format=jpeg`;
      }
      if (newFormat === 'png' && !survey.includes('?')) {
        surveyWithFormat = `${survey}?format=png`;
      }
      
      aladinInstance.setImageSurvey(surveyWithFormat);
      updateStatus(newFormat, survey);
    } catch (error) {
      // Silent error handling
    }
  }, [aladinInstance, survey, updateStatus]);

  // Handle search - PURE REACT
  const handleSearch = useCallback((searchValue) => {
    if (!aladinInstance || !searchValue) return;
    
    try {
      setSearchTerm(searchValue);
      
      // Use Aladin's gotoObject method
      aladinInstance.gotoObject(searchValue, {
        success: (coords) => {
          if (coords && coords.length >= 2) {
            // Store coordinates globally for compatibility
            window.currentObjectCoords = coords;
            window.currentLoadedObject = searchValue;

            // Register object coordinates in the coordinate registry
            // This allows reverse lookup: coordinates -> object name
            registerObjectCoordinates(searchValue, coords[0], coords[1]);

            // Update status
            updateStatus(format, survey);

            // Only load gallery if we're searching for a different object
            // or if gallery is currently empty
            const isNewObject = window.currentLoadedObject !== searchValue;
            const galleryIsEmpty = gallery?.galleryItems?.length === 0;
            
            if (gallery?.loadObjectImages && (isNewObject || galleryIsEmpty)) {
              // Pass skipCoordinateCheck=true to bypass coordinate check during navigation
              gallery.loadObjectImages(searchValue, null, true);
            }
          }
        },
        error: () => {
          updateStatus(format, survey);
        }
      });
    } catch (error) {
      // Silent error handling
    }
  }, [aladinInstance, format, survey, updateStatus, gallery]);

  // Keyboard shortcuts - PURE REACT STATE
  const cycleSurvey = useCallback(() => {
    const currentIndex = surveys.indexOf(survey);
    const nextIndex = (currentIndex + 1) % surveys.length;
    handleSurveyChange(surveys[nextIndex]);
  }, [survey, surveys, handleSurveyChange]);

  const cycleFormat = useCallback(() => {
    const currentIndex = formats.indexOf(format);
    const nextIndex = (currentIndex + 1) % formats.length;
    handleFormatChange(formats[nextIndex]);
  }, [format, formats, handleFormatChange]);

  // Setup keyboard controls - NO DOM QUERIES
  useEffect(() => {
    if (!aladinInstance || keyboardInitRef.current) return;

    const handleKeyDown = (event) => {
      // Only trigger if not typing in an input field
      if (event.target.tagName === 'INPUT' || event.target.tagName === 'TEXTAREA') {
        return;
      }

      switch (event.key.toLowerCase()) {
        case 'f':
          event.preventDefault();
          cycleFormat();
          break;
        case 's':
          event.preventDefault();
          cycleSurvey();
          break;
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    keyboardInitRef.current = true;

    return () => {
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [aladinInstance, cycleFormat, cycleSurvey]);

  // Initialize status
  useEffect(() => {
    updateStatus(format, survey);
  }, [format, survey, updateStatus]);

  return {
    // State
    survey,
    format,
    status,
    searchTerm,
    
    // Handlers
    handleSurveyChange,
    handleFormatChange,
    handleSearch,
    setSearchTerm,
    
    // Keyboard actions
    cycleSurvey,
    cycleFormat,
    
    // Options for UI
    surveys,
    formats
  };
}; 
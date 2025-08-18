import { useCallback } from 'react';
import { DEFAULTS } from '../../utils/constants/constants';

export const useSurveyControls = (aladinInstance) => {
  const handleFormatChange = useCallback((format) => {
    if (!aladinInstance) return;
    
    try {
      const currentSurvey = document.getElementById('survey-select')?.value || DEFAULTS.survey;
      let surveyWithFormat = currentSurvey;
      
      // Apply format parameter if not already present
      if (format === 'jpeg' && !currentSurvey.includes('?')) {
        surveyWithFormat = `${currentSurvey}?format=jpeg`;
      }
      if (format === 'png' && !currentSurvey.includes('?')) {
        surveyWithFormat = `${currentSurvey}?format=png`;
      }
      
      // Update Aladin survey with format
      aladinInstance.setImageSurvey(surveyWithFormat);
      
      // Update status display
      const statusElement = document.getElementById('current-status');
      if (statusElement) {
        statusElement.textContent = `Format: ${format.toUpperCase()} | Survey: ${currentSurvey}`;
      }
    } catch (error) {
      // Silent error handling like the original
    }
  }, [aladinInstance]);

  const handleSurveyChange = useCallback((survey) => {
    if (!aladinInstance) return;
    
    try {
      // Update Aladin survey
      aladinInstance.setImageSurvey(survey);
      
      // Update status display with current format
      const statusElement = document.getElementById('current-status');
      if (statusElement) {
        const fmt = document.getElementById('format-select')?.value || 'fits';
        statusElement.textContent = `Format: ${fmt.toUpperCase()} | Survey: ${survey}`;
      }
    } catch (error) {
      // Silent error handling like the original
    }
  }, [aladinInstance]);

  return {
    handleFormatChange,
    handleSurveyChange
  };
}; 
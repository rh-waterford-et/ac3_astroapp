import { useState, useEffect, useCallback } from 'react';

const CONNECTOR_MODE_STORAGE_KEY = 'uc3-connector-mode';

/**
 * Custom hook to manage connector mode state with localStorage persistence
 * @returns {Object} - { isConnectorMode, setConnectorMode, toggleConnectorMode }
 */
export const useConnectorMode = () => {
  // Initialize state from localStorage or default to false
  const [isConnectorMode, setIsConnectorMode] = useState(() => {
    try {
      const stored = localStorage.getItem(CONNECTOR_MODE_STORAGE_KEY);
      return stored ? JSON.parse(stored) : false;
    } catch (error) {
      console.warn('Failed to load connector mode from localStorage:', error);
      return false;
    }
  });

  // Persist to localStorage whenever state changes
  useEffect(() => {
    try {
      localStorage.setItem(CONNECTOR_MODE_STORAGE_KEY, JSON.stringify(isConnectorMode));
    } catch (error) {
      console.warn('Failed to save connector mode to localStorage:', error);
    }
  }, [isConnectorMode]);

  // Set connector mode with validation
  const setConnectorMode = useCallback((enabled) => {
    if (typeof enabled !== 'boolean') {
      console.warn('setConnectorMode expects a boolean value');
      return;
    }
    setIsConnectorMode(enabled);
  }, []);

  // Toggle connector mode
  const toggleConnectorMode = useCallback(() => {
    setIsConnectorMode(prev => !prev);
  }, []);

  return {
    isConnectorMode,
    setConnectorMode,
    toggleConnectorMode
  };
};

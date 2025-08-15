import React, { createContext, useContext, useState, useEffect } from 'react';

const AppStateContext = createContext(null);

export const useAppState = () => {
  const context = useContext(AppStateContext);
  if (!context) {
    throw new Error('useAppState must be used within AppStateProvider');
  }
  return context;
};

export const AppStateProvider = ({ children }) => {
  const [currentLoadedObject, setCurrentLoadedObject] = useState(null);
  const [aladinInstance, setAladinInstance] = useState(null);

  // Sync with window variables (for backward compatibility during transition)
  useEffect(() => {
    // Monitor window.currentLoadedObject changes
    const checkCurrentObject = () => {
      if (window.currentLoadedObject !== currentLoadedObject) {
        setCurrentLoadedObject(window.currentLoadedObject);
      }
    };

    // Monitor window.aladinInstance changes  
    const checkAladinInstance = () => {
      if (window.aladinInstance !== aladinInstance) {
        setAladinInstance(window.aladinInstance);
      }
    };

    // Set up polling to sync with window variables
    const interval = setInterval(() => {
      checkCurrentObject();
      checkAladinInstance();
    }, 100);

    // Initial sync
    checkCurrentObject();
    checkAladinInstance();

    return () => clearInterval(interval);
  }, [currentLoadedObject, aladinInstance]);

  const value = {
    currentLoadedObject,
    aladinInstance,
    setCurrentLoadedObject,
    setAladinInstance
  };

  return (
    <AppStateContext.Provider value={value}>
      {children}
    </AppStateContext.Provider>
  );
}; 
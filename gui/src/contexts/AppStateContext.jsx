import React, { createContext, useContext, useState, useEffect, useRef } from 'react';

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
  
  // Use refs to access latest state in polling without dependency issues
  const currentLoadedObjectRef = useRef(currentLoadedObject);
  const aladinInstanceRef = useRef(aladinInstance);
  
  // Update refs when state changes
  useEffect(() => {
    currentLoadedObjectRef.current = currentLoadedObject;
  }, [currentLoadedObject]);
  
  useEffect(() => {
    aladinInstanceRef.current = aladinInstance;
  }, [aladinInstance]);

  useEffect(() => {
    // Monitor window.currentLoadedObject changes
    const checkCurrentObject = () => {
      if (window.currentLoadedObject !== currentLoadedObjectRef.current) {
        setCurrentLoadedObject(window.currentLoadedObject);
      }
    };

    // Monitor window.aladinInstance changes  
    const checkAladinInstance = () => {
      if (window.aladinInstance !== aladinInstanceRef.current) {
        setAladinInstance(window.aladinInstance);
      }
    };

    // Set up polling to sync with window variables - reduced from 100ms to 1000ms
    const interval = setInterval(() => {
      checkCurrentObject();
      checkAladinInstance();
    }, 1000);

    // Initial sync
    checkCurrentObject();
    checkAladinInstance();

    return () => clearInterval(interval);
  }, []); // Empty dependency array - effect only runs once

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
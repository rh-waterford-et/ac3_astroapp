// Gallery Context - Replaces global window functions with proper React communication
import React, { createContext, useContext, useCallback } from 'react';

const GalleryContext = createContext(null);

export const useGallery = () => {
  const context = useContext(GalleryContext);
  if (!context) {
    throw new Error('useGallery must be used within a GalleryProvider');
  }
  return context;
};

export const GalleryProvider = ({ children, galleryOperations }) => {
  // Provide the gallery operations to the context
  const contextValue = {
    addMapToGallery: useCallback((mapType, label, icon) => {
      if (galleryOperations?.addMapToGallery) {
        galleryOperations.addMapToGallery(mapType, label, icon);
      }
    }, [galleryOperations]),

    removeMapFromGallery: useCallback((mapType) => {
      if (galleryOperations?.removeMapFromGallery) {
        galleryOperations.removeMapFromGallery(mapType);
      }
    }, [galleryOperations]),

    loadObjectImages: useCallback((objectName, overrideCheckboxStates = null) => {
      if (galleryOperations?.loadObjectImages) {
        galleryOperations.loadObjectImages(objectName, overrideCheckboxStates);
      }
    }, [galleryOperations]),

    clearGallery: useCallback(() => {
      if (galleryOperations?.clearGallery) {
        galleryOperations.clearGallery();
      }
    }, [galleryOperations]),

    // Pagination state
    currentPage: galleryOperations?.currentPage ?? 0,
    totalPages: galleryOperations?.totalPages ?? 1,
    itemsPerPage: galleryOperations?.itemsPerPage ?? 50,
    galleryItems: galleryOperations?.galleryItems ?? [],

    // Pagination functions
    goToNextPage: useCallback(() => {
      if (galleryOperations?.goToNextPage) {
        galleryOperations.goToNextPage();
      }
    }, [galleryOperations]),

    goToPrevPage: useCallback(() => {
      if (galleryOperations?.goToPrevPage) {
        galleryOperations.goToPrevPage();
      }
    }, [galleryOperations]),

    goToPage: useCallback((page) => {
      if (galleryOperations?.goToPage) {
        galleryOperations.goToPage(page);
      }
    }, [galleryOperations])
  };

  return (
    <GalleryContext.Provider value={contextValue}>
      {children}
    </GalleryContext.Provider>
  );
}; 
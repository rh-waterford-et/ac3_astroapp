import React from 'react';
import { useState, useRef } from 'react';
import Sidebar from './components/aladin/Sidebar';
import Gallery from './components/aladin/Gallery';
import StatusBar from './components/aladin/StatusBar';
import ErrorOverlay from './components/aladin/ErrorOverlay';
import GalleryModal from './components/aladin/gallery/modal/GalleryModal';
import TabNavigation from './components/ui/TabNavigation';
import PipelineAppSelector from './components/pipeline/PipelineAppSelector';
import HeaderControls from './components/aladin/HeaderControls';

import AladinInteractions from './components/aladin/AladinInteractions';
import { MODAL_DIMENSIONS } from './utils/constants/constants';
import DatasetsList from './components/pipeline/DatasetsList';
import logoImage from './assets/AC3-LogoConFrase.jpg';
import { GalleryProvider, useGallery } from './contexts/GalleryContext';
import { AppStateProvider } from './contexts/AppStateContext';
import { useAladinInstance } from './hooks/aladin/useAladinInstance';
import { useSidebarControls } from './hooks/ui/useSidebarControls';
import { useGalaxySearch } from './hooks/ui/useGalaxySearch';
import { useTabManager } from './hooks/ui/useTabManager';
import { useSurveyControls } from './hooks/ui/useSurveyControls';
import { useUploadWarning } from './hooks/ui/useUploadWarning';

// Main App Content that uses Gallery Context
function AppContent({ onGalleryOperationsReady }) {
  // Use Aladin instance hook for all Aladin-related state and initialization
  const { aladinInstance, isLoading, error, retry, scriptLoaded } = useAladinInstance();
  
  const [selectedApp, setSelectedApp] = useState('starlight'); 
  const modalRndRef = useRef(null);

  // Use Gallery Context instead of window functions
  const gallery = useGallery();

  // Use sidebar controls hook for all checkbox state and control logic
  const { sidebarState, handleCheckboxChange } = useSidebarControls(aladinInstance, gallery);

  // Use galaxy search hook for search functionality
  const { handleSearch } = useGalaxySearch(aladinInstance, gallery);

  // Use tab manager hook for tab state and gallery restoration
  const { activeTab, setActiveTab } = useTabManager(aladinInstance, sidebarState, gallery);

  // Use survey controls hook for format and survey changes
  const { handleFormatChange, handleSurveyChange } = useSurveyControls(aladinInstance);

  // Use upload warning hook for beforeunload protection
  useUploadWarning();

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
            onRetry={retry}
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
                checkboxStates={sidebarState}
                onCheckboxChange={handleCheckboxChange}
              />
            </>
          )}

          {activeTab === 'pipeline' && (
            <div className="pipeline-full-view">
              {/* Pipeline Monitor with full space */}
              <DatasetsList selectedApp={selectedApp} />
            </div>
          )}
        </div>
      </main>

      {/* Bottom Overlays */}
      {activeTab === 'maps' && (
        <>
          {/* Bottom Gallery */}
          <div className="bottom-overlay gallery-overlay">
            <Gallery checkboxStates={sidebarState} onGalleryOperationsReady={onGalleryOperationsReady} />
          </div>
          {/* Status Bar */}
          <div className="bottom-overlay status-overlay">
            <StatusBar />
          </div>
        </>
      )}

      {/* Interactions Handler (invisible component) */}
      <AladinInteractions aladinInstance={aladinInstance} />
      
      {/* Gallery Modal */}
      <GalleryModal
        ref={modalRndRef}
        defaultPosition={{ x: 50, y: 50 }}
        defaultSize={{ width: MODAL_DIMENSIONS.width, height: MODAL_DIMENSIONS.height }}
      />
    </div>
  );
}

// Main App Wrapper with Contexts
function App() {
  const [galleryOperations, setGalleryOperations] = useState(null);

  const handleGalleryOperationsReady = (operations) => {
    setGalleryOperations(operations);
  };

  return (
    <AppStateProvider>
      <GalleryProvider galleryOperations={galleryOperations}>
        <AppContent onGalleryOperationsReady={handleGalleryOperationsReady} />
      </GalleryProvider>
    </AppStateProvider>
  );
}

export default App; 
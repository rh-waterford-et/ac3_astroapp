import React from 'react';
import TabNavigation from './components/ui/TabNavigation';
import Gallery from './components/aladin/Gallery';
import Sidebar from './components/aladin/Sidebar';
import AladinInteractions from './components/aladin/AladinInteractions';
import HeaderControls from './components/aladin/HeaderControls';
import StatusBar from './components/aladin/StatusBar';
import ImageModal from './components/ui/ImageModal';
import GalleryModal from './components/aladin/gallery/modal/GalleryModal';
import PipelineAppSelector from './components/pipeline/PipelineAppSelector';
import DatasetsList from './components/pipeline/DatasetsList';
import ConnectorModeToggle from './components/pipeline/ConnectorModeToggle';
import UploadZone from './components/upload/UploadZone';
import logoImage from './assets/AC3-LogoConFrase.jpg';
import { AppStateProvider } from './contexts/AppStateContext';
import { GalleryProvider, useGallery } from './contexts/GalleryContext';
import { useAladinInstance } from './hooks/aladin/useAladinInstance';
import { useAladinControls } from './hooks/aladin/useAladinControls';
import { useSidebarControls } from './hooks/ui/useSidebarControls';
import { useTabManager } from './hooks/ui/useTabManager';
import { useUploadWarning } from './hooks/ui/useUploadWarning';
import { useConnectorMode } from './hooks/data/useConnectorMode';

function AppContent({ onGalleryOperationsReady }) {
  // Aladin instance and loading state
  const { aladinInstance, isLoading, error, retry } = useAladinInstance();

  // Modal ref for centering
  const modalRndRef = React.useRef(null);

  // Use gallery context with proper hook
  const gallery = useGallery();

  // Centralized Aladin controls - REPLACES multiple hooks
  const {
    survey,
    format,
    status,
    searchTerm,
    surveys,
    formats,
    handleSurveyChange,
    handleFormatChange,
    handleSearch,
    setSearchTerm
  } = useAladinControls(aladinInstance, gallery);

  // Use sidebar controls hook for all checkbox state and control logic
  const { sidebarState, handleCheckboxChange } = useSidebarControls(aladinInstance, gallery);

  // Use tab manager hook for tab state and gallery restoration
  const { activeTab, setActiveTab } = useTabManager(aladinInstance, sidebarState, gallery);

  // Use upload warning hook for beforeunload protection
  useUploadWarning();

  // Selected app state for pipeline tab
  const [selectedApp, setSelectedApp] = React.useState('starlight');

  // Connector mode state
  const { isConnectorMode, setConnectorMode } = useConnectorMode();

  return (
    <div className="app">
      <header className="app-header">
        <div className="header-left">
          <div className="app-logo">
            <img src={logoImage} alt="AC3 Logo" className="logo-image" />
          </div>
          <TabNavigation activeTab={activeTab} onTabChange={setActiveTab} />
          {/* App Selection for Pipeline */}
          {activeTab === 'pipeline' && (
            <>
              <PipelineAppSelector selectedApp={selectedApp} onSelect={setSelectedApp} />
              <ConnectorModeToggle 
                isEnabled={isConnectorMode} 
                onToggle={setConnectorMode} 
              />
            </>
          )}
        </div>
        {/* Integrated Controls */}
        {activeTab === 'maps' && (
          <HeaderControls
            survey={survey}
            format={format}
            searchTerm={searchTerm}
            surveys={surveys}
            formats={formats}
            onFormatChange={handleFormatChange}
            onSurveyChange={handleSurveyChange}
            onSearch={handleSearch}
            onSearchTermChange={setSearchTerm}
          />
        )}
      </header>

      {/* Background Aladin Map */}
      <div className="aladin-background">
        {isLoading && !error && (
          <div className="loading-overlay">
            <div className="astro-loader-galaxy"></div>
            <p>
              Loading Aladin Lite
            </p>
          </div>
        )}
        {error && (
          <div className="error-overlay">
            <div className="error-content">
              <h3>⚠️ Aladin Lite Failed to Load</h3>
              <p>{error}</p>
              <button onClick={retry} className="retry-button">
                🔄 Retry Loading
              </button>
            </div>
          </div>
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
              <DatasetsList 
                processorType={selectedApp} 
                isConnectorMode={isConnectorMode}
              />
            </div>
          )}
        </div>
      </main>

      {/* Bottom Overlays */}
      {activeTab === 'maps' && (
        <>
          {/* Bottom Gallery */}
          <div className="bottom-overlay gallery-overlay">
            <Gallery 
              aladinInstance={aladinInstance} 
              checkboxStates={sidebarState}
              onGalleryOperationsReady={onGalleryOperationsReady}
            />
          </div>
          {/* Status Bar */}
          <div className="bottom-overlay status-overlay">
            <StatusBar />
          </div>
        </>
      )}

      {/* Interactions Handler (invisible component) */}
      <AladinInteractions aladinInstance={aladinInstance} />
      
      {/* Modal Components - Rendered at top level for full-screen display */}
      <GalleryModal modalRndRef={modalRndRef} />
    </div>
  );
}

function App() {
  const [galleryOperations, setGalleryOperations] = React.useState(null);

  const handleGalleryOperationsReady = React.useCallback((operations) => {
    setGalleryOperations(operations);
  }, []);

  return (
    <AppStateProvider>
      <GalleryProvider galleryOperations={galleryOperations}>
        <AppContent onGalleryOperationsReady={handleGalleryOperationsReady} />
      </GalleryProvider>
    </AppStateProvider>
  );
}

export default App; 
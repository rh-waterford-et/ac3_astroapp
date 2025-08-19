import React from 'react';

export default function TabNavigation({ activeTab, onTabChange }) {
  return (
    <div className="tab-navigation">
      <button 
        className={`tab-button ${activeTab === 'maps' ? 'active' : ''}`}
        onClick={() => onTabChange('maps')}
      >
        Maps
      </button>
      <button 
        className={`tab-button ${activeTab === 'pipeline' ? 'active' : ''}`}
        onClick={() => onTabChange('pipeline')}
      >
        Pipeline
      </button>
    </div>
  );
} 
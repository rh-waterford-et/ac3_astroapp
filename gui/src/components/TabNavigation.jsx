import React from 'react';

export default function TabNavigation({ activeTab, onSelect }) {
  return (
    <div className="tab-navigation">
      <button 
        className={`tab-button ${activeTab === 'maps' ? 'active' : ''}`}
        onClick={() => onSelect('maps')}
      >
        Maps
      </button>
      <button 
        className={`tab-button ${activeTab === 'pipeline' ? 'active' : ''}`}
        onClick={() => onSelect('pipeline')}
      >
        Pipeline
      </button>
    </div>
  );
} 
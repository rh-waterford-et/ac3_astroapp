import React from 'react';

export default function PipelineAppSelector({ selectedApp, onSelect }) {
  return (
    <div className="app-selection">
      <button 
        className={`app-btn ${selectedApp === 'starlight' ? 'active' : ''}`}
        onClick={() => onSelect('starlight')}
      >
        Starlight
      </button>
      <button 
        className={`app-btn ${selectedApp === 'ppxf' ? 'active' : ''}`}
        onClick={() => onSelect('ppxf')}
      >
        PPXF
      </button>
      <button 
        className={`app-btn ${selectedApp === 'voronoi' ? 'active' : ''}`}
        onClick={() => onSelect('voronoi')}
      >
        Voronoi Binning
      </button>
      <button 
        className={`app-btn ${selectedApp === 'steckmap' ? 'active' : ''} disabled`}
        onClick={() => onSelect('steckmap')}
        disabled
      >
        SteckMap
      </button>
      <div className="app-selection-divider"></div>
    </div>
  );
} 
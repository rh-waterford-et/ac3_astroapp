import React from 'react';

export default function ErrorOverlay({ error, onRetry }) {
  return (
    <div className="error-overlay">
      <div className="error-title">
        <span>⚠️</span>
        <span>Aladin Loading Error</span>
      </div>
      <p className="error-message">{error}</p>
      <p className="error-hint">Please check your internet connection and WebGL2 support.</p>
      <button className="retry-btn" onClick={onRetry}>Retry</button>
    </div>
  );
} 
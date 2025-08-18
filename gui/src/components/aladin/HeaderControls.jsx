import React, { useRef, useEffect } from 'react';

export default function HeaderControls({ onFormatChange, onSurveyChange, onSearch }) {
  const inputRef = useRef(null);
  const buttonRef = useRef(null);

  useEffect(() => {
    const btn = buttonRef.current;
    const input = inputRef.current;
    if (!btn || !input) return;

    const onKey = (event) => {
      if (event.key === 'Enter') {
        onSearch?.(input.value.trim());
      }
    };

    input.addEventListener('keypress', onKey);
    return () => {
      input.removeEventListener('keypress', onKey);
    };
  }, [onSearch]);

  return (
    <div className="header-controls">
      <div className="header-control-group">
        <div className="select-wrapper">
          <select
            id="format-select"
            defaultValue="fits"
            className="header-select"
            onChange={(e) => onFormatChange?.(e.target.value)}
          >
            <option value="fits">FITS</option>
            <option value="jpeg">JPEG</option>
            <option value="png">PNG</option>
          </select>
          <div className="select-arrow">
            <svg width="10" height="6" viewBox="0 0 12 8" fill="none">
              <path d="M1 1L6 6L11 1" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
            </svg>
          </div>
        </div>
      </div>

      <div className="header-control-group">
        <div className="select-wrapper">
          <select
            id="survey-select"
            defaultValue="P/DSS2/color"
            className="header-select"
            onChange={(e) => onSurveyChange?.(e.target.value)}
          >
            <option value="P/DSS2/color">DSS2 Color</option>
            <option value="P/2MASS/color">2MASS</option>
            <option value="P/allWISE/color">AllWISE</option>
            <option value="P/SDSS9/color">SDSS9</option>
            <option value="P/GLIMPSE360">GLIMPSE</option>
          </select>
          <div className="select-arrow">
            <svg width="10" height="6" viewBox="0 0 12 8" fill="none">
              <path d="M1 1L6 6L11 1" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
            </svg>
          </div>
        </div>
      </div>

      <div className="header-control-group">
        <div className="header-search-input-wrapper">
          <input
            ref={inputRef}
            type="text"
            id="galaxy-search"
            className="header-search-input"
            placeholder="Enter galaxy name..."
          />
          <div className="header-search-icon">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none">
              <circle cx="11" cy="11" r="8" stroke="currentColor" strokeWidth="2"/>
              <path d="m21 21-4.35-4.35" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
            </svg>
          </div>
        </div>
        <button
          ref={buttonRef}
          id="search-galaxy-btn"
          className="header-search-btn"
          onClick={() => onSearch?.(inputRef.current?.value.trim() ?? '')}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none">
            <circle cx="11" cy="11" r="8" stroke="currentColor" strokeWidth="2"/>
            <path d="m21 21-4.35-4.35" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
          </svg>
        </button>
      </div>
    </div>
  );
} 
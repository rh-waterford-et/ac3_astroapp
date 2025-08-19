import React from 'react';

const StatusBar = () => {
  return (
    <div className="status-bar">
      <div className="status-info">
        <span id="current-status">Ready</span>
      </div>
      <div className="controls-help">
        F: Format • S: Survey • G: Go to object • Click: Center & zoom 
      </div>
    </div>
  );
};

export default StatusBar; 
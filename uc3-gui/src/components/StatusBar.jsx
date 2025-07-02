import React from 'react';

const StatusBar = () => {
  return (
    <div className="status-bar">
      <div className="status-info">
        <span id="current-status">Ready - Move mouse to see coordinates</span>
      </div>
      <div className="controls-help">
        <span>Click to zoom | +/- keys to zoom | R to reset | G to go to object | H for help | F for format | S for survey</span>
      </div>
    </div>
  );
};

export default StatusBar; 
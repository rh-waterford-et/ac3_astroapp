import React from 'react';
import PropTypes from 'prop-types';

const ConnectorModeToggle = ({ isEnabled, onToggle, disabled = false }) => {
  return (
    <div className="connector-mode-toggle">
      <button
        className={`toggle-button ${isEnabled ? 'active' : 'inactive'}`}
        onClick={() => onToggle(!isEnabled)}
        disabled={disabled}
        title={isEnabled ? 'Switch to direct upload mode' : 'Switch to connector mode'}
      >
        <span className="toggle-icon">
          {isEnabled ? '🔗' : '📁'}
        </span>
        <span className="toggle-content">
          <span className="toggle-label">Connector Mode</span>
          <span className="toggle-status">
            {isEnabled ? '(2-bucket transfer)' : '(direct upload)'}
          </span>
        </span>
        <span className="toggle-indicator">
          {isEnabled ? 'ON' : 'OFF'}
        </span>
      </button>
    </div>
  );
};

ConnectorModeToggle.propTypes = {
  isEnabled: PropTypes.bool.isRequired,
  onToggle: PropTypes.func.isRequired,
  disabled: PropTypes.bool
};

export default ConnectorModeToggle;

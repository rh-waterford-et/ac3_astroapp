import React from 'react';
import PropTypes from 'prop-types';

const ConnectorModeToggle = ({ isEnabled, onToggle, disabled = false }) => {
  return (
    <button
      className={`app-btn connector-btn ${isEnabled ? 'active' : ''}`}
      onClick={() => onToggle(!isEnabled)}
      disabled={disabled}
      title={isEnabled ? 'Switch to direct upload mode' : 'Switch to connector mode'}
    >
      <span className="plug-icon">🔌</span>
    </button>
  );
};

ConnectorModeToggle.propTypes = {
  isEnabled: PropTypes.bool.isRequired,
  onToggle: PropTypes.func.isRequired,
  disabled: PropTypes.bool
};

export default ConnectorModeToggle;

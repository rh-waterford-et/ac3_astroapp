import React, { useState } from 'react';
import PropTypes from 'prop-types';
import { restartRabbitMQ } from '../../services/api';

const ResetQueueButton = ({ disabled = false }) => {
  const [isLoading, setIsLoading] = useState(false);

  const handleReset = async () => {
    const confirmed = window.confirm(
      'Reset queue and restart RabbitMQ, consumer, and producer?\n\n' +
      'This will clear all queued messages and reset processing.\n' +
      'This action cannot be undone.'
    );

    if (!confirmed) {
      return;
    }

    setIsLoading(true);
    try {
      const result = await restartRabbitMQ();
      if (result.success) {
        alert(`✅ ${result.message}`);
      } else {
        alert(`❌ ${result.message}`);
      }
    } catch (error) {
      alert(`❌ Failed to reset queue: ${error.message}`);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <button
      className="app-btn reset-queue-btn"
      onClick={handleReset}
      disabled={disabled || isLoading}
      title="Reset queue and restart RabbitMQ, consumer, and producer (clears all queued messages)"
    >
      <span className="stop-icon">
        {isLoading ? (
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <circle cx="12" cy="12" r="10" />
            <path d="M12 6v6l4 2" />
          </svg>
        ) : (
          <svg 
            width="16" 
            height="16" 
            viewBox="0 0 24 24" 
            fill="currentColor" 
            stroke="none"
          >
            <rect x="6" y="6" width="12" height="12" rx="1" />
          </svg>
        )}
      </span>
    </button>
  );
};

ResetQueueButton.propTypes = {
  disabled: PropTypes.bool
};

export default ResetQueueButton;


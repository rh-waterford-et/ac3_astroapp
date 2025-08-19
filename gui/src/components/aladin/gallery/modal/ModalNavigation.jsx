// Modal navigation - handles navigation buttons and keyboard controls
import React, { useEffect } from 'react';
import PropTypes from 'prop-types';

const ModalNavigation = ({ 
  onPrevious, 
  onNext, 
  onClose, 
  hasMultipleImages, 
  currentIndex, 
  totalImages,
  isModalOpen 
}) => {

  // Handle keyboard navigation
  useEffect(() => {
    if (!isModalOpen) return;

    const handleKeyDown = (e) => {
      switch (e.key) {
        case 'ArrowLeft':
          e.preventDefault();
          if (hasMultipleImages) onPrevious();
          break;
        case 'ArrowRight':
          e.preventDefault();
          if (hasMultipleImages) onNext();
          break;
        case 'Escape':
          e.preventDefault();
          onClose();
          break;
        default:
          break;
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [isModalOpen, hasMultipleImages, onPrevious, onNext, onClose]);

  return (
    <div className="modal-nav-buttons">
      <button 
        className="modal-nav-btn modal-prev" 
        id="modal-prev"
        title="Previous image"
        disabled={!hasMultipleImages}
        onClick={onPrevious}
      >
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
          <path 
            d="M15 18l-6-6 6-6" 
            stroke="currentColor" 
            strokeWidth="2" 
            strokeLinecap="round" 
            strokeLinejoin="round"
          />
        </svg>
      </button>
      
      <button 
        className="modal-nav-btn modal-next" 
        id="modal-next"
        title="Next image"
        disabled={!hasMultipleImages}
        onClick={onNext}
      >
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
          <path 
            d="M9 18l6-6-6-6" 
            stroke="currentColor" 
            strokeWidth="2" 
            strokeLinecap="round" 
            strokeLinejoin="round"
          />
        </svg>
      </button>
      
      {/* Display current position if multiple images */}
      {hasMultipleImages && totalImages > 0 && (
        <span 
          className="modal-nav-counter"
          style={{
            fontSize: '12px',
            color: '#a0aec0',
            marginLeft: '8px',
            userSelect: 'none'
          }}
        >
          {currentIndex + 1} / {totalImages}
        </span>
      )}
    </div>
  );
};

ModalNavigation.propTypes = {
  onPrevious: PropTypes.func.isRequired,
  onNext: PropTypes.func.isRequired,
  onClose: PropTypes.func.isRequired,
  hasMultipleImages: PropTypes.bool,
  currentIndex: PropTypes.number,
  totalImages: PropTypes.number,
  isModalOpen: PropTypes.bool
};

ModalNavigation.defaultProps = {
  hasMultipleImages: false,
  currentIndex: 0,
  totalImages: 0,
  isModalOpen: false
};

export default ModalNavigation; 
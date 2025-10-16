// Modal image viewer - handles image and PDF display with interactions
import React, { useRef, useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useImageInteractions } from '../../../../hooks/modal/useImageInteractions';

const ModalImageViewer = ({ imageData, isVisible, onLoad, onError }) => {
  const imageRef = useRef(null);
  const [isImageLoaded, setIsImageLoaded] = useState(false);
  const { transform, isDragging, resetTransform } = useImageInteractions(imageRef);

  // Reset transform when image changes
  useEffect(() => {
    if (imageData) {
      resetTransform();
      setIsImageLoaded(false);
    }
  }, [imageData, resetTransform]);

  const handleImageLoad = () => {
    setIsImageLoaded(true);
    if (onLoad) onLoad();
  };

  const handleImageError = (error) => {
    setIsImageLoaded(false);
    if (onError) onError(error);
  };

  if (!imageData || !isVisible) {
    return null;
  }

  const isPdf = imageData.isPdf;
  const isPlaceholder = imageData.isPlaceholder;

  return (
    <>
      {isPlaceholder ? (
        <div 
          className="modal-placeholder"
          style={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            width: '100%',
            height: '100%',
            minHeight: '400px',
            backgroundColor: 'rgba(45, 55, 72, 0.3)',
            borderRadius: '8px',
            border: '2px dashed rgba(79, 209, 197, 0.3)',
            padding: '2rem'
          }}
        >
          <div style={{
            fontSize: '6rem',
            marginBottom: '1rem',
            opacity: 0.6
          }}>
            {imageData.icon || '📊'}
          </div>
          <div style={{
            fontSize: '1.5rem',
            fontWeight: '600',
            color: 'var(--blue-primary)',
            marginBottom: '0.5rem',
            textAlign: 'center'
          }}>
            {imageData.title}
          </div>
          <div style={{
            fontSize: '0.9rem',
            color: '#a0aec0',
            textAlign: 'center',
            maxWidth: '400px'
          }}>
            Placeholder - Backend integration needed
          </div>
        </div>
      ) : isPdf ? (
        <iframe
          className="pdf-iframe"
          src={imageData.src}
          style={{
            width: '100%',
            height: '100%',
            border: 'none',
            borderRadius: '4px'
          }}
          title={`${imageData.title} for ${imageData.objectName}`}
          onLoad={handleImageLoad}
          onError={handleImageError}
        />
      ) : (
        <img
          ref={imageRef}
          className={`modal-image ${isDragging ? 'dragging' : ''}`}
          src={imageData.src}
          alt={`${imageData.title} for ${imageData.objectName}`}
          style={{
            maxWidth: '100%',
            maxHeight: '100%',
            width: 'auto',
            height: 'auto',
            objectFit: 'contain',
            borderRadius: '8px',
            boxShadow: '0 4px 20px rgba(0, 0, 0, 0.5)',
            cursor: isDragging ? 'grabbing' : 'grab',
            userSelect: 'none',
            transition: isDragging ? 'none' : 'transform 0.1s ease-out',
            display: isImageLoaded ? 'block' : 'none'
          }}
          onLoad={handleImageLoad}
          onError={handleImageError}
        />
      )}
      
    </>
  );
};

ModalImageViewer.propTypes = {
  imageData: PropTypes.shape({
    src: PropTypes.string.isRequired,
    title: PropTypes.string.isRequired,
    objectName: PropTypes.string.isRequired,
    isPdf: PropTypes.bool,
    isPlaceholder: PropTypes.bool,
    icon: PropTypes.string
  }),
  isVisible: PropTypes.bool,
  onLoad: PropTypes.func,
  onError: PropTypes.func
};

ModalImageViewer.defaultProps = {
  imageData: null,
  isVisible: true,
  onLoad: null,
  onError: null
};

export default ModalImageViewer; 
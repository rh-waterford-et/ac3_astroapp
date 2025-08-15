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

  return (
    <>
      {isPdf ? (
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
      
      {/* Loading indicator */}
      {!isImageLoaded && (
        <div style={{
          position: 'absolute',
          top: '50%',
          left: '50%',
          transform: 'translate(-50%, -50%)',
          color: '#a0aec0',
          fontSize: '14px',
          pointerEvents: 'none',
          zIndex: 10
        }}>
          Loading {isPdf ? 'PDF' : 'image'}...
        </div>
      )}
    </>
  );
};

ModalImageViewer.propTypes = {
  imageData: PropTypes.shape({
    src: PropTypes.string.isRequired,
    title: PropTypes.string.isRequired,
    objectName: PropTypes.string.isRequired,
    isPdf: PropTypes.bool
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
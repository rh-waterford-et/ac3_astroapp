// Image gallery item - for static images (NGC7025 maps)
import React from 'react';
import PropTypes from 'prop-types';
import GalleryItem from './GalleryItem';

const ImageGalleryItem = ({ imageSrc, mapType, objectName, onStatusUpdate }) => {
  const handleClick = () => {
    // Open modal with full-size image
    if (window.openImageModal) {
      // Find the actual DOM element for compatibility with existing modal system
      const galleryContainer = document.querySelector('.gallery-rows-container') || document.querySelector('.gallery-content');
      const clickedItem = galleryContainer?.querySelector(`[data-map-type="${mapType.key}"]`);
      window.openImageModal(imageSrc, mapType.label, objectName, clickedItem);
    }
    
    // Update status
    const statusElement = document.getElementById('current-status');
    if (statusElement) {
      statusElement.textContent = `Viewing ${objectName} map: ${mapType.label}`;
    }
    
    if (onStatusUpdate) {
      onStatusUpdate(`Viewing ${objectName} map: ${mapType.label}`);
    }
  };

  return (
    <GalleryItem
      className="object-map-item"
      mapType={mapType.key}
      objectName={objectName}
      onClick={handleClick}
    >
      <div className="gallery-thumbnail">
        <img 
          src={imageSrc} 
          alt={`${mapType.label} for ${objectName}`} 
          className="thumbnail-image" 
        />
      </div>
      <div className="gallery-label">{mapType.label}</div>
    </GalleryItem>
  );
};

ImageGalleryItem.propTypes = {
  imageSrc: PropTypes.string.isRequired,
  mapType: PropTypes.shape({
    key: PropTypes.string.isRequired,
    label: PropTypes.string.isRequired
  }).isRequired,
  objectName: PropTypes.string.isRequired,
  onStatusUpdate: PropTypes.func
};

export default ImageGalleryItem; 
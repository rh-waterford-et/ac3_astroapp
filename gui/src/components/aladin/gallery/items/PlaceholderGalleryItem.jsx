// Placeholder gallery item - for placeholder items when checkboxes are selected
import React from 'react';
import PropTypes from 'prop-types';
import GalleryItem from './GalleryItem';

const PlaceholderGalleryItem = ({ mapType, label, icon, onStatusUpdate }) => {
  const handleClick = () => {
    console.log(`Clicked on map: ${mapType}`);
    
    // Update status
    const statusElement = document.getElementById('current-status');
    if (statusElement) {
      statusElement.textContent = `Selected map: ${label} (placeholder - backend integration needed)`;
    }
    
    if (onStatusUpdate) {
      onStatusUpdate(`Selected map: ${label} (placeholder - backend integration needed)`);
    }
  };

  return (
    <GalleryItem
      className="placeholder-item"
      mapType={mapType}
      onClick={handleClick}
    >
      <div className="gallery-thumbnail">
        <div className="thumbnail-placeholder map-placeholder">
          <span className="map-icon">{icon}</span>
        </div>
      </div>
      <div className="gallery-label">{label}</div>
    </GalleryItem>
  );
};

PlaceholderGalleryItem.propTypes = {
  mapType: PropTypes.string.isRequired,
  label: PropTypes.string.isRequired,
  icon: PropTypes.string.isRequired,
  onStatusUpdate: PropTypes.func
};

export default PlaceholderGalleryItem; 
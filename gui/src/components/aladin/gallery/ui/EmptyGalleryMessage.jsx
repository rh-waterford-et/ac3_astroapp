// Empty gallery message - handles various empty states and messages
import React from 'react';
import PropTypes from 'prop-types';

const EmptyGalleryMessage = ({ type, objectName }) => {
  const getMessageContent = () => {
    switch (type) {
      case 'empty':
        return (
          <p>Select at least 1 option from the sidebar and navigate to a celestial object to view maps</p>
        );
      
      case 'no-images':
        return (
          <>
            <p>No map images available for {objectName}</p>
            <p className="sub-message">
              Images will be loaded when available in the format: {objectName.replace(/\s+/g, '').toUpperCase()}_maptype.jpg
            </p>
          </>
        );
      
      case 'no-options':
        return (
          <>
            <p>Select map options from the sidebar to view {objectName} images</p>
            <p className="sub-message">Check the boxes in "Available Maps" to load corresponding images</p>
          </>
        );
      
      case 'navigate-to-object':
        return (
          <>
            <p>Navigate closer to the searched object to view map images</p>
            <p className="sub-message">Use the galaxy search to find and navigate to celestial objects</p>
          </>
        );
      
      default:
        return <p>No content available</p>;
    }
  };

  const className = type === 'empty' ? 'empty-gallery-message' : 'no-images-message';

  return (
    <div className={className}>
      {getMessageContent()}
    </div>
  );
};

EmptyGalleryMessage.propTypes = {
  type: PropTypes.oneOf(['empty', 'no-images', 'no-options', 'navigate-to-object']).isRequired,
  objectName: PropTypes.string
};

export default EmptyGalleryMessage; 
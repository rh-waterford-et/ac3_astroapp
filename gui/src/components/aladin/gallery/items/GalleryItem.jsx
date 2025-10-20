// Base gallery item component - common layout and behavior for all gallery items
import React from 'react';
import PropTypes from 'prop-types';

const GalleryItem = ({ 
  children,
  className = '',
  mapType,
  objectName,
  cellNumber,
  pdfKey,
  onClick,
  selected = false,
  ...otherDataAttributes
}) => {
  // Build the complete className
  const fullClassName = `gallery-item ${className}`.trim();
  
  // Handle click
  const handleClick = (event) => {
    if (onClick) {
      onClick(event);
    }
    
    // Toggle selection class on this element
    event.currentTarget.classList.toggle('selected');
    
    // Remove selection from other items (mimicking original behavior)
    const galleryContainer = document.querySelector('.gallery-rows-container') || document.querySelector('.gallery-content');
    if (galleryContainer && mapType) {
      const otherItems = galleryContainer.querySelectorAll(`.gallery-item:not([data-map-type="${mapType}"])`);
      otherItems.forEach(item => item.classList.remove('selected'));
    }
  };

  // Build data attributes object
  const dataAttributes = {
    'data-map-type': mapType,
    'data-object-name': objectName,
    'data-cell-number': cellNumber,
    'data-pdf-key': pdfKey,
    ...otherDataAttributes
  };

  // Filter out undefined values
  Object.keys(dataAttributes).forEach(key => {
    if (dataAttributes[key] === undefined) {
      delete dataAttributes[key];
    }
  });

  return (
    <div 
      className={fullClassName}
      onClick={handleClick}
      {...dataAttributes}
    >
      {children}
    </div>
  );
};

GalleryItem.propTypes = {
  children: PropTypes.node.isRequired,
  className: PropTypes.string,
  mapType: PropTypes.string,
  objectName: PropTypes.string,
  cellNumber: PropTypes.string,
  pdfKey: PropTypes.string,
  onClick: PropTypes.func,
  selected: PropTypes.bool
};

export default GalleryItem; 
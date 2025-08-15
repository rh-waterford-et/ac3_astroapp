// Gallery loader - loading indicator for gallery operations
import React from 'react';
import PropTypes from 'prop-types';

const GalleryLoader = ({ objectName }) => {
  return (
    <div 
      className="gallery-loader"
      style={{
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        width: '100%',
        height: '200px'
      }}
    >
      <div 
        className="astro-loading-container"
        style={{
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'center',
          alignItems: 'center',
          textAlign: 'center',
          padding: '2rem'
        }}
      >
        <div 
          className="astro-loader-galaxy"
          style={{
            width: '32px',
            height: '32px'
          }}
        />
        <div 
          className="astro-loading-text"
          style={{
            fontSize: '14px',
            marginTop: '0.5rem'
          }}
        >
          Loading maps for {objectName}...
        </div>
      </div>
    </div>
  );
};

GalleryLoader.propTypes = {
  objectName: PropTypes.string.isRequired
};

export default GalleryLoader; 
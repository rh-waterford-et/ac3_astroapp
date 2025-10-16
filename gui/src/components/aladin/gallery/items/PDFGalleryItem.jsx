// PDF gallery item - for PDF items with thumbnail generation
import React, { useEffect } from 'react';
import PropTypes from 'prop-types';
import GalleryItem from './GalleryItem';
import { 
  createPdfModalUrl, 
  createPdfThumbnailUrl, 
  generatePdfThumbnail 
} from '../../../../utils/gallery/pdfUtils';
import { 
  extractCellNumber, 
  generatePdfDisplayName 
} from '../../../../utils/gallery/galleryUtils';

const PDFGalleryItem = ({ pdfFile, objectName, onStatusUpdate }) => {
  const cellNumber = extractCellNumber(pdfFile.name);
  const displayName = generatePdfDisplayName(cellNumber);
  
  // Generate PDF thumbnail when component mounts
  useEffect(() => {
    const thumbnailUrl = createPdfThumbnailUrl(pdfFile.key);
    generatePdfThumbnail(thumbnailUrl, cellNumber);
  }, [pdfFile.key, cellNumber]);

  const handleClick = () => {
    // Create PDF URL for modal display
    const pdfUrl = createPdfModalUrl(pdfFile.key);
    
    // Open PDF in modal using existing modal system
    if (window.openImageModal) {
      // Find the actual DOM element for compatibility with existing modal system
      const galleryContainer = document.querySelector('.gallery-rows-container') || document.querySelector('.gallery-content');
      const clickedItem = galleryContainer?.querySelector(`[data-cell-number="${cellNumber}"]`);
      window.openImageModal(pdfUrl, `${displayName} PDF`, objectName, clickedItem, true);
    }
    
    // Update status
    const statusElement = document.getElementById('current-status');
    if (statusElement) {
      statusElement.textContent = `Viewing ${objectName} H4 PDF: Cell ${cellNumber}`;
    }
    
    if (onStatusUpdate) {
      onStatusUpdate(`Viewing ${objectName} H4 PDF: Cell ${cellNumber}`);
    }
  };

  return (
    <GalleryItem
      className="object-map-item pdf-item"
      mapType="h4"
      objectName={objectName}
      cellNumber={cellNumber}
      pdfKey={pdfFile.key}
      onClick={handleClick}
    >
      <div className="gallery-thumbnail">
        <div 
          className="thumbnail-placeholder pdf-placeholder" 
          id={`pdf-thumb-${cellNumber}`}
        >
          <canvas 
            className="pdf-thumbnail-canvas" 
            width="150" 
            height="200" 
            style={{ display: 'none' }}
          />
          <div className="pdf-loading-indicator">
            <span className="cell-label">Cell {cellNumber}</span>
            <div className="loading-text">Loading preview...</div>
          </div>
        </div>
      </div>
      <div className="gallery-label">{displayName}</div>
    </GalleryItem>
  );
};

PDFGalleryItem.propTypes = {
  pdfFile: PropTypes.shape({
    name: PropTypes.string.isRequired,
    key: PropTypes.string.isRequired
  }).isRequired,
  objectName: PropTypes.string.isRequired,
  onStatusUpdate: PropTypes.func
};

export default PDFGalleryItem; 
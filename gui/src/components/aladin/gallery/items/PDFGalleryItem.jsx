// PDF gallery item - for PDF items with thumbnail generation
import React, { useEffect, useMemo } from 'react';
import PropTypes from 'prop-types';
import GalleryItem from './GalleryItem';
import { 
  createPdfModalUrl, 
  createPdfThumbnailUrl, 
  generatePdfThumbnail,
  generateStaticPdfThumbnail 
} from '../../../../utils/gallery/pdfUtils';
import { 
  extractCellNumber, 
  generatePdfDisplayName 
} from '../../../../utils/gallery/galleryUtils';

const PDFGalleryItem = ({ pdfFile, objectName, onStatusUpdate, mapType }) => {
  const isStatic = pdfFile.isStatic || false;
  const cellNumber = isStatic ? null : extractCellNumber(pdfFile.name);
  const displayName = isStatic ? pdfFile.name : generatePdfDisplayName(cellNumber);
  
  // Create unique ID for static PDFs (use hash of the key)
  const uniqueId = useMemo(() => {
    if (isStatic) {
      return pdfFile.key.split('/').pop().replace(/\W/g, '');
    }
    return cellNumber;
  }, [isStatic, pdfFile.key, cellNumber]);
  
  // Generate PDF thumbnail
  useEffect(() => {
    if (isStatic) {
      // For static PDFs, use the imported asset path directly
      generateStaticPdfThumbnail(pdfFile.key, uniqueId);
    } else if (cellNumber) {
      // For S3 PDFs, fetch from API
      const thumbnailUrl = createPdfThumbnailUrl(pdfFile.key);
      generatePdfThumbnail(thumbnailUrl, cellNumber);
    }
  }, [pdfFile.key, cellNumber, isStatic, uniqueId]);

  const handleClick = async () => {
    let pdfUrl;
    
    if (isStatic) {
      // For static PDFs, fetch as blob and create blob URL to avoid routing through API
      try {
        const response = await fetch(pdfFile.key);
        const blob = await response.blob();
        pdfUrl = URL.createObjectURL(blob) + '#toolbar=0';
      } catch (error) {
        // Fallback to direct path
        pdfUrl = `${pdfFile.key}#toolbar=0`;
      }
    } else {
      // For S3 PDFs, use API endpoint with parameters
      pdfUrl = createPdfModalUrl(pdfFile.key);
    }
    
    // Open PDF in modal using existing modal system
    if (window.openImageModal) {
      // Find the actual DOM element for compatibility with existing modal system
      const galleryContainer = document.querySelector('.gallery-rows-container') || document.querySelector('.gallery-content');
      const selector = isStatic ? `[data-pdf-key="${pdfFile.key}"]` : `[data-cell-number="${cellNumber}"]`;
      const clickedItem = galleryContainer?.querySelector(selector);
      window.openImageModal(pdfUrl, `${displayName} PDF`, objectName, clickedItem, true);
    }
    
    // Update status
    const statusLabel = mapType || 'pPXF Fitting';
    const statusElement = document.getElementById('current-status');
    if (statusElement) {
      statusElement.textContent = `Viewing ${objectName} ${statusLabel}: ${displayName}`;
    }
    
    if (onStatusUpdate) {
      onStatusUpdate(`Viewing ${objectName} ${statusLabel}: ${displayName}`);
    }
  };

  return (
    <GalleryItem
      className="object-map-item pdf-item"
      mapType={mapType || "ppxf-fitting"}
      objectName={objectName}
      cellNumber={cellNumber}
      pdfKey={pdfFile.key}
      onClick={handleClick}
    >
      <div className="gallery-thumbnail">
        <div 
          className="thumbnail-placeholder pdf-placeholder" 
          id={isStatic ? `static-pdf-thumb-${uniqueId}` : `pdf-thumb-${cellNumber}`}
        >
          <canvas 
            className="pdf-thumbnail-canvas" 
            width="150" 
            height="200" 
            style={{ display: 'none' }}
          />
          <div className="pdf-loading-indicator">
            {isStatic ? (
              <>
                <div className="pdf-icon">📄</div>
                <div className="loading-text">{displayName}</div>
              </>
            ) : (
              <>
                <span className="cell-label">Cell {cellNumber}</span>
                <div className="loading-text">Loading preview...</div>
              </>
            )}
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
    key: PropTypes.string.isRequired,
    isStatic: PropTypes.bool
  }).isRequired,
  objectName: PropTypes.string.isRequired,
  onStatusUpdate: PropTypes.func,
  mapType: PropTypes.string
};

export default PDFGalleryItem; 
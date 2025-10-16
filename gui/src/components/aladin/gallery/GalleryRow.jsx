import React from 'react';
import PropTypes from 'prop-types';
import ImageGalleryItem from './items/ImageGalleryItem';
import PDFGalleryItem from './items/PDFGalleryItem';
import PlaceholderGalleryItem from './items/PlaceholderGalleryItem';

const ITEMS_PER_PAGE = 20;

const GalleryRow = ({ group, onPageChange, onStatusUpdate }) => {
  const { mapType, label, items, currentPage } = group;
  
  const totalPages = Math.ceil(items.length / ITEMS_PER_PAGE);
  const startIdx = currentPage * ITEMS_PER_PAGE;
  const endIdx = startIdx + ITEMS_PER_PAGE;
  const pageItems = items.slice(startIdx, endIdx);
  
  const handlePrevPage = () => {
    if (currentPage > 0) {
      onPageChange(mapType, currentPage - 1);
    }
  };
  
  const handleNextPage = () => {
    if (currentPage < totalPages - 1) {
      onPageChange(mapType, currentPage + 1);
    }
  };
  
  return (
    <div className="gallery-row">
      <h4 className="gallery-row-header">
        {label} ({items.length})
      </h4>
      
      {items.length === 0 ? (
        <div className="gallery-row-empty">
          No Maps Available
        </div>
      ) : (
        <div className="gallery-row-content">
          {/* Left arrow */}
          {totalPages > 1 && currentPage > 0 && (
            <button 
              className="gallery-row-arrow gallery-row-arrow-left"
              onClick={handlePrevPage}
              title="Previous page"
              aria-label="Previous page"
            >
              ‹
            </button>
          )}
          
          {/* Items container */}
          <div className="gallery-row-items">
            {pageItems.map(item => {
              switch (item.type) {
                case 'image':
                  return (
                    <ImageGalleryItem
                      key={item.id}
                      imageSrc={item.imageSrc}
                      mapType={item.mapType}
                      objectName={item.objectName}
                      onStatusUpdate={onStatusUpdate}
                    />
                  );
                case 'pdf':
                  return (
                    <PDFGalleryItem
                      key={item.id}
                      pdfFile={item.pdfFile}
                      objectName={item.objectName}
                      onStatusUpdate={onStatusUpdate}
                    />
                  );
                case 'placeholder':
                  return (
                    <PlaceholderGalleryItem
                      key={item.id}
                      mapType={item.mapType}
                      label={item.label}
                      icon={item.icon}
                      onStatusUpdate={onStatusUpdate}
                    />
                  );
                default:
                  return null;
              }
            })}
          </div>
          
          {/* Right arrow */}
          {totalPages > 1 && currentPage < totalPages - 1 && (
            <button 
              className="gallery-row-arrow gallery-row-arrow-right"
              onClick={handleNextPage}
              title="Next page"
              aria-label="Next page"
            >
              ›
            </button>
          )}
        </div>
      )}
    </div>
  );
};

GalleryRow.propTypes = {
  group: PropTypes.shape({
    mapType: PropTypes.string.isRequired,
    label: PropTypes.string.isRequired,
    items: PropTypes.array.isRequired,
    currentPage: PropTypes.number.isRequired,
    order: PropTypes.number.isRequired,
  }).isRequired,
  onPageChange: PropTypes.func.isRequired,
  onStatusUpdate: PropTypes.func,
};

export default GalleryRow;


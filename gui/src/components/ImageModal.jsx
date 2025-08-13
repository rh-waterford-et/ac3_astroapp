import React, { forwardRef } from 'react';
import { Rnd } from 'react-rnd';

const ImageModal = forwardRef(function ImageModal({ width, height }, modalRndRef) {
  return (
    <div className="image-modal" id="image-modal">
      <div className="modal-backdrop" id="modal-backdrop"></div>
      <Rnd
        ref={modalRndRef}
        default={{
          x: 0,
          y: 0,
          width: width,
          height: height,
        }}
        minWidth={480}
        minHeight={320}
        maxWidth="90vw"
        maxHeight="85vh"
        bounds="window"
        dragHandleClassName="modal-header"
        cancel=".modal-close, .transparency-control, .modal-nav-buttons"
        className="modal-content-rnd"
        onDragStart={() => {
          const modalContent = document.querySelector('.modal-content');
          if (modalContent) {
            modalContent.classList.add('dragging');
          }
        }}
        onDragStop={() => {
          const modalContent = document.querySelector('.modal-content');
          if (modalContent) {
            modalContent.classList.remove('dragging');
          }
        }}
        style={{
          display: 'none',
        }}
      >
        <div className="modal-content" id="modal-content">
          <div className="modal-header">
            <span className="modal-object-code" id="modal-object">Object</span>
            <h3 className="modal-title" id="modal-title">Image Title</h3>
            <div className="modal-nav-buttons">
              <button className="modal-nav-btn modal-prev" id="modal-prev" title="Previous image">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
                  <path d="M15 18l-6-6 6-6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                </svg>
              </button>
              <button className="modal-nav-btn modal-next" id="modal-next" title="Next image">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
                  <path d="M9 18l6-6-6-6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                </svg>
              </button>
            </div>
            <div className="modal-controls">
              <div className="transparency-control">
                <label htmlFor="transparency-slider" className="transparency-label" aria-label="Image transparency control">
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                    <circle cx="12" cy="12" r="3" stroke="currentColor" strokeWidth="2"/>
                    <path d="M12 1v6m0 6v6m11-7h-6m-6 0H1" stroke="currentColor" strokeWidth="2"/>
                  </svg>
                </label>
                <input 
                  type="range" 
                  id="transparency-slider" 
                  className="transparency-slider"
                  min="20" 
                  max="100" 
                  defaultValue="95"
                  aria-label="Adjust image transparency from 20% to 100%"
                  onChange={(e) => {
                    const modalBody = document.querySelector('.modal-body');
                    if (modalBody) {
                      const opacity = e.target.value / 100;
                      modalBody.style.opacity = opacity;
                    }
                  }}
                />
              </div>
              <button className="modal-close" id="modal-close">×</button>
            </div>
          </div>
          <div className="modal-body">
            <img className="modal-image" id="modal-image" alt="" />
          </div>
        </div>
      </Rnd>
    </div>
  );
});

export default ImageModal; 
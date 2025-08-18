// PDF utilities - PDF.js integration and thumbnail generation
import { PDF_CONFIG } from '../constants/galleryConstants';
import { extractCellNumber } from './galleryUtils';

/**
 * Load PDF.js library dynamically if not already loaded
 * @returns {Promise} - Promise that resolves when PDF.js is loaded
 */
export const loadPdfJs = () => {
  if (typeof window.pdfjsLib !== 'undefined') {
    return Promise.resolve();
  }
  
  return new Promise((resolve, reject) => {
    const script = document.createElement('script');
    script.src = 'https://cdnjs.cloudflare.com/ajax/libs/pdf.js/3.11.174/pdf.min.js';
    script.onload = () => {
      window.pdfjsLib.GlobalWorkerOptions.workerSrc = 'https://cdnjs.cloudflare.com/ajax/libs/pdf.js/3.11.174/pdf.worker.min.js';
      resolve();
    };
    script.onerror = reject;
    document.head.appendChild(script);
  });
};

/**
 * Generate a PDF thumbnail using PDF.js
 * @param {string} pdfUrl - URL of the PDF file
 * @param {string} cellNumber - Cell number for the thumbnail container ID
 * @returns {Promise} - Promise that resolves when thumbnail is generated
 */
export const generatePdfThumbnail = async (pdfUrl, cellNumber) => {
  try {
    // Check if PDF.js is available
    if (typeof window.pdfjsLib === 'undefined') {
      console.log('PDF.js not available, keeping placeholder for Cell', cellNumber);
      return;
    }

    const pdf = await window.pdfjsLib.getDocument(pdfUrl).promise;
    const page = await pdf.getPage(1); // Get first page
    
    const container = document.getElementById(`pdf-thumb-${cellNumber}`);
    if (!container) return;
    
    const canvas = container.querySelector('.pdf-thumbnail-canvas');
    const loadingIndicator = container.querySelector('.pdf-loading-indicator');
    
    if (!canvas || !loadingIndicator) return;
    
    const context = canvas.getContext('2d');
    const viewport = page.getViewport({ scale: PDF_CONFIG.THUMBNAIL_SCALE });
    
    canvas.width = viewport.width;
    canvas.height = viewport.height;
    
    await page.render({
      canvasContext: context,
      viewport: viewport
    }).promise;
    
    // Show canvas, hide loading indicator
    canvas.style.display = 'block';
    loadingIndicator.style.display = 'none';
    
    console.log(`✅ Generated thumbnail for Cell ${cellNumber}`);
    
  } catch (error) {
    console.log(`⚠️ Could not generate thumbnail for Cell ${cellNumber}:`, error.message);
    
    // Show error state in the placeholder
    const container = document.getElementById(`pdf-thumb-${cellNumber}`);
    if (container) {
      const loadingIndicator = container.querySelector('.pdf-loading-indicator');
      if (loadingIndicator) {
        loadingIndicator.innerHTML = `
          <span class="cell-label">Cell ${cellNumber}</span>
          <div class="loading-text" style="color: #ff6b6b;">Preview failed</div>
        `;
      }
    }
  }
};

/**
 * Create PDF URL for modal display
 * @param {string} pdfKey - S3 key for the PDF file
 * @returns {string} - PDF URL with viewer settings
 */
export const createPdfModalUrl = (pdfKey) => {
  return `/api/files/download?key=${encodeURIComponent(pdfKey)}#zoom=100&toolbar=0&navpanes=0`;
};

/**
 * Create PDF thumbnail URL for download
 * @param {string} pdfKey - S3 key for the PDF file
 * @returns {string} - PDF URL for thumbnail generation
 */
export const createPdfThumbnailUrl = (pdfKey) => {
  return `/api/files/download?key=${encodeURIComponent(pdfKey)}`;
};

/**
 * Sort PDF files by cell number (numeric sort)
 * @param {Array} pdfFiles - Array of PDF file objects
 * @returns {Array} - Sorted array of PDF files
 */
export const sortPdfFilesByCellNumber = (pdfFiles) => {
  return pdfFiles.sort((a, b) => {
    const cellA = parseInt(extractCellNumber(a.name));
    const cellB = parseInt(extractCellNumber(b.name));
    return cellA - cellB;
  });
};

/**
 * Filter files for PDFs that are in cell subdirectories
 * @param {Array} files - Array of file objects
 * @returns {Array} - Filtered array of PDF files
 */
export const filterPdfFiles = (files) => {
  return files.filter(file => 
    file.name.includes('/') && 
    file.name.toLowerCase().endsWith('.pdf')
  );
};

/**
 * Check if a PDF item already exists in the gallery
 * @param {HTMLElement} galleryContainer - Gallery container element
 * @param {string} cellNumber - Cell number to check
 * @returns {boolean} - True if PDF item already exists
 */
export const pdfItemExists = (galleryContainer, cellNumber) => {
  return !!galleryContainer.querySelector(`[data-map-type="h4"][data-cell-number="${cellNumber}"]`);
};

/**
 * Initialize PDF.js when the module loads
 * This loads PDF.js asynchronously and catches errors gracefully
 */
export const initializePdfJs = () => {
  loadPdfJs().catch(() => {
    console.log('PDF.js could not be loaded, thumbnails will show placeholders');
  });
}; 
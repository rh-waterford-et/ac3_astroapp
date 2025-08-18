// Gallery utility functions - pure functions with no side effects
import { COORDINATE_TOLERANCE } from '../constants/galleryConstants';

/**
 * Generate cache key for PDF data
 * @param {string} objectName - Object name
 * @param {string} processorType - Processor type (e.g., 'ppxf')
 * @returns {string} - Cache key
 */
export const getCacheKey = (objectName, processorType) => `${objectName}-${processorType}`;

/**
 * Get cached PDF data for an object and processor type
 * @param {string} objectName - Object name
 * @param {string} processorType - Processor type
 * @param {Map} pdfCache - PDF cache map
 * @returns {Object} - Cached PDF data or default structure
 */
export const getCachedPdfs = (objectName, processorType, pdfCache) => {
  const key = getCacheKey(objectName, processorType);
  return pdfCache.get(key) || { batches: [], total: 0, lastUpdated: 0 };
};

/**
 * Set cached PDF data for an object and processor type
 * @param {string} objectName - Object name
 * @param {string} processorType - Processor type
 * @param {Object} data - PDF data to cache
 * @param {Map} pdfCache - PDF cache map
 */
export const setCachedPdfs = (objectName, processorType, data, pdfCache) => {
  const key = getCacheKey(objectName, processorType);
  pdfCache.set(key, { ...data, lastUpdated: Date.now() });
};

/**
 * Normalize object name for API calls and file naming
 * @param {string} objectName - Original object name
 * @returns {string} - Normalized object name
 */
export const normalizeObjectName = (objectName) => {
  return objectName
    .toUpperCase()
    .replace(/\s+/g, '')  // Remove spaces
    .replace(/^NGC(\d+)$/, 'NGC$1'); // Ensure NGC format
};

/**
 * Check if current Aladin view is at or near the object coordinates
 * @param {string} objectName - The object name
 * @param {Object} aladinInstance - Aladin instance
 * @returns {boolean} - True if at object coordinates
 */
export const isAtObjectCoordinates = (objectName, aladinInstance) => {
  // If no coordinates stored or no object loaded, don't show images
  if (!window.currentObjectCoords || !window.currentLoadedObject) {
    console.log(`📍 No coordinates stored (coords: ${!!window.currentObjectCoords}, object: ${window.currentLoadedObject})`);
    return false;
  }
  
  // Only check coordinates if this is the currently loaded object
  if (window.currentLoadedObject !== objectName) {
    console.log(`📍 Object mismatch: requested ${objectName}, loaded ${window.currentLoadedObject}`);
    return false;
  }
  
  // Get current Aladin position
  if (!aladinInstance) {
    console.log(`📍 No Aladin instance available`);
    return false;
  }
  
  try {
    const currentPos = aladinInstance.getRaDec();
    const objectCoords = window.currentObjectCoords;
    
    // Validate coordinates
    if (!currentPos || !objectCoords || currentPos.length < 2 || objectCoords.length < 2) {
      console.log(`📍 Invalid coordinate data: current=${currentPos}, object=${objectCoords}`);
      return false;
    }
    
    // Calculate angular distance between current position and object
    const deltaRA = Math.abs(currentPos[0] - objectCoords[0]);
    const deltaDec = Math.abs(currentPos[1] - objectCoords[1]);
    
    const isNear = deltaRA < COORDINATE_TOLERANCE && deltaDec < COORDINATE_TOLERANCE;
    console.log(`📍 Coordinate check: Current(${currentPos[0].toFixed(3)}, ${currentPos[1].toFixed(3)}) vs Object(${objectCoords[0].toFixed(3)}, ${objectCoords[1].toFixed(3)}) - Distance: RA=${deltaRA.toFixed(3)}°, Dec=${deltaDec.toFixed(3)}° - Near: ${isNear}`);
    
    return isNear;
  } catch (error) {
    console.error('Error checking coordinates:', error);
    return false; // Don't show images if coordinate check fails
  }
};

/**
 * Check if an image exists at the given path
 * @param {string} imagePath - The image path to check
 * @returns {Promise<boolean>} - True if image exists
 */
export const checkImageExists = (imagePath) => {
  return new Promise((resolve) => {
    const img = new Image();
    img.onload = () => resolve(true);
    img.onerror = () => resolve(false);
    img.src = imagePath;
  });
};

/**
 * Try to load an image for a specific object and map type
 * @param {Object} mapType - Map type configuration object
 * @param {string} objectName - The original object name for display
 * @param {Object} imageMap - Mapping of object names to their image assets
 * @returns {Object|null} - Image data if found, null otherwise
 */
export const tryGetObjectImage = (mapType, objectName, imageMap) => {
  // Normalize object name to match image map keys
  const normalizedName = normalizeObjectName(objectName);
  
  // Check if we have an image map for this object
  if (!imageMap[normalizedName]) {
    console.log(`No image map found for object: ${normalizedName}`);
    return null;
  }
  
  // Get the image source for this map type
  const imageSrc = imageMap[normalizedName][mapType.suffix];
  
  if (imageSrc) {
    console.log(`✅ Found image for ${objectName} ${mapType.label}: ${mapType.suffix}`);
    return {
      imageSrc,
      mapType,
      objectName
    };
  } else {
    console.log(`❌ No image found for ${objectName} ${mapType.label}: ${mapType.suffix}`);
    return null;
  }
};

/**
 * Replace message template placeholders with actual values
 * @param {string} template - Message template with {placeholder} syntax
 * @param {Object} values - Object with replacement values
 * @returns {string} - Message with placeholders replaced
 */
export const formatMessage = (template, values = {}) => {
  return template.replace(/\{(\w+)\}/g, (match, key) => {
    return values[key] || match;
  });
};

/**
 * Check minimum checkbox selection requirement
 * @param {number} checkedCount - Number of checked boxes
 * @param {number} minimum - Minimum required (default: 1)
 * @returns {boolean} - True if requirement is met
 */
export const meetsMinimumSelection = (checkedCount, minimum = 1) => {
  return checkedCount >= minimum;
};

/**
 * Extract cell number from PDF filename
 * @param {string} filename - PDF filename like "0/filename.pdf"
 * @returns {string} - Cell number
 */
export const extractCellNumber = (filename) => {
  return filename.split('/')[0];
};

/**
 * Generate display name for PDF cell
 * @param {string} cellNumber - Cell number
 * @returns {string} - Display name like "Cell 0 H4"
 */
export const generatePdfDisplayName = (cellNumber) => {
  return `Cell ${cellNumber} H4`;
}; 
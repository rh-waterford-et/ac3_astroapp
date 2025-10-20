/**
 * Coordinate Registry - Maps celestial coordinates to object names
 * Enables reverse lookup: coordinates -> object name
 * Used for loading gallery images when user navigates to known coordinates
 */

// Tolerance for coordinate matching (in degrees)
const COORDINATE_MATCH_TOLERANCE = 0.05; // ~3 arcminutes

/**
 * Initialize the global coordinate registry
 */
export const initializeCoordinateRegistry = () => {
  if (!window.objectCoordinateRegistry) {
    window.objectCoordinateRegistry = new Map();
  }
};

/**
 * Generate a coordinate key from RA/Dec for registry lookup
 * Rounds to 2 decimal places for fuzzy matching
 * @param {number} ra - Right Ascension in degrees
 * @param {number} dec - Declination in degrees
 * @returns {string} Coordinate key in format "ra_dec"
 */
const getCoordinateKey = (ra, dec) => {
  return `${ra.toFixed(2)}_${dec.toFixed(2)}`;
};

/**
 * Register an object at given coordinates
 * @param {string} objectName - Name of the celestial object
 * @param {number} ra - Right Ascension in degrees
 * @param {number} dec - Declination in degrees
 */
export const registerObjectCoordinates = (objectName, ra, dec) => {
  initializeCoordinateRegistry();
  
  const key = getCoordinateKey(ra, dec);
  window.objectCoordinateRegistry.set(key, {
    objectName,
    ra,
    dec,
    timestamp: Date.now()
  });
};

/**
 * Find object at given coordinates (fuzzy match within tolerance)
 * @param {number} ra - Right Ascension in degrees
 * @param {number} dec - Declination in degrees
 * @param {number} tolerance - Match tolerance in degrees (default: 0.05)
 * @returns {string|null} Object name if found, null otherwise
 */
export const findObjectAtCoordinates = (ra, dec, tolerance = COORDINATE_MATCH_TOLERANCE) => {
  initializeCoordinateRegistry();
  
  // Iterate through registered objects and find closest match
  for (const [key, data] of window.objectCoordinateRegistry.entries()) {
    const deltaRA = Math.abs(data.ra - ra);
    const deltaDec = Math.abs(data.dec - dec);
    
    // Check if within tolerance
    if (deltaRA < tolerance && deltaDec < tolerance) {
      return data.objectName;
    }
  }
  
  return null;
};

/**
 * Check if an object is registered in the coordinate registry
 * @param {string} objectName - Name of the celestial object
 * @returns {boolean} True if object is registered
 */
export const isObjectRegistered = (objectName) => {
  initializeCoordinateRegistry();
  
  for (const data of window.objectCoordinateRegistry.values()) {
    if (data.objectName === objectName) {
      return true;
    }
  }
  
  return false;
};

/**
 * Get coordinates for a registered object
 * @param {string} objectName - Name of the celestial object
 * @returns {Object|null} Object with ra and dec if found, null otherwise
 */
export const getObjectCoordinates = (objectName) => {
  initializeCoordinateRegistry();
  
  for (const data of window.objectCoordinateRegistry.values()) {
    if (data.objectName === objectName) {
      return { ra: data.ra, dec: data.dec };
    }
  }
  
  return null;
};

/**
 * Clear the entire coordinate registry
 */
export const clearCoordinateRegistry = () => {
  if (window.objectCoordinateRegistry) {
    window.objectCoordinateRegistry.clear();
  }
};

/**
 * Get all registered objects
 * @returns {Array} Array of registered object data
 */
export const getAllRegisteredObjects = () => {
  initializeCoordinateRegistry();
  return Array.from(window.objectCoordinateRegistry.values());
};


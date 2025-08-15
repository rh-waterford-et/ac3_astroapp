// Gallery constants and static data
// Import NGC7025 images from assets
import NGC7025_stellar_velocity from '../assets/NGC7025_stellar_velocity.jpg';
import NGC7025_stellar_velocity_error from '../assets/NGC7025_stellar_velocity_error.jpg';
import NGC7025_velocity_dispersion from '../assets/NGC7025_velocity_dispersion.jpg';
import NGC7025_velocity_dispersion_error from '../assets/NGC7025_velocity_dispersion_error.jpg';
import NGC7025_h3 from '../assets/NGC7025_h3.jpg';
import NGC7025_h4 from '../assets/NGC7025_h4.jpg';
import NGC7025_age from '../assets/NGC7025_age.jpeg';
import NGC7025_age_mass_weighted from '../assets/NGC7025_age_mass_weighted.jpg';
import NGC7025_metallicity from '../assets/NGC7025_metallicity.jpg';

// Image mapping for specific objects (can be extended for other objects)
// To add support for other objects, import their images and add them to this map
export const IMAGE_MAP = {
  NGC7025: {
    stellar_velocity: NGC7025_stellar_velocity,
    stellar_velocity_error: NGC7025_stellar_velocity_error,
    velocity_dispersion: NGC7025_velocity_dispersion,
    velocity_dispersion_error: NGC7025_velocity_dispersion_error,
    h3: NGC7025_h3,
    h4: NGC7025_h4,
    age: NGC7025_age,
    age_mass_weighted: NGC7025_age_mass_weighted,
    metallicity: NGC7025_metallicity
  }
};

// Map types that we expect to find images for
export const MAP_TYPES = [
  { key: 'stellar-velocity', suffix: 'stellar_velocity', label: 'Stellar Velocity', checkboxId: 'map-stellar-velocity' },
  { key: 'stellar-velocity-error', suffix: 'stellar_velocity_error', label: 'Stellar Velocity Error', checkboxId: 'map-stellar-velocity-error' },
  { key: 'velocity-dispersion', suffix: 'velocity_dispersion', label: 'Velocity Dispersion', checkboxId: 'map-velocity-dispersion' },
  { key: 'velocity-dispersion-error', suffix: 'velocity_dispersion_error', label: 'Velocity Dispersion Error', checkboxId: 'map-velocity-dispersion-error' },
  { key: 'h3', suffix: 'h3', label: 'H3', checkboxId: 'map-h3' },
  { key: 'h4', suffix: 'h4', label: 'H4', checkboxId: 'map-h4' },
  { key: 'age-lum-weighted', suffix: 'age', label: 'Age (Lum. Weighted)', checkboxId: 'map-age-weighted' },
  { key: 'age-mass-weighted', suffix: 'age_mass_weighted', label: 'Age (Mass Weighted)', checkboxId: 'map-age-mass-weighted' },
  { key: 'metallicity', suffix: 'metallicity', label: 'Metallicity', checkboxId: 'map-metallicity' }
];

// PDF configuration
export const PDF_CONFIG = {
  BATCH_SIZE: 50,
  CACHE_KEY_SEPARATOR: '-',
  MAX_OFFSET_SAFETY: 10000,
  THUMBNAIL_SCALE: 0.5,
  THUMBNAIL_CANVAS_WIDTH: 150,
  THUMBNAIL_CANVAS_HEIGHT: 200
};

// Coordinate tolerance for object positioning (in degrees)
export const COORDINATE_TOLERANCE = 0.05; // 3 arcminutes

// Gallery loading messages
export const GALLERY_MESSAGES = {
  EMPTY: 'Select at least 1 option from the sidebar and navigate to a celestial object to view maps',
  NO_OPTIONS: 'Select map options from the sidebar to view {objectName} images',
  NO_IMAGES: 'No map images available for {objectName}',
  NAVIGATE_TO_OBJECT: 'Navigate closer to the searched object to view map images',
  LOADING: 'Loading maps for {objectName}...'
}; 
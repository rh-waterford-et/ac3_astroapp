export const MAP_CONTROLS = {
    'map-stellar-velocity': { label: 'Stellar velocity', icon: '🌀' },
    'map-velocity-dispersion': { label: 'Velocity dispersion', icon: '📊' },
    'map-stellar-velocity-error': { label: 'Stellar velocity Error', icon: '⚠️' },
    'map-velocity-dispersion-error': { label: 'Velocity dispersion Error', icon: '📈' },
    'map-h3': { label: 'h3', icon: 'H₃' },
    'map-h4': { label: 'h4', icon: 'H₄' },
    'map-age-weighted': { label: 'Age (lum. weighted)', icon: '⏳' },
    'map-age-mass-weighted': { label: 'Age (Mass Weighted)', icon: '⚖️' },
    'map-metallicity': { label: 'Metallicity', icon: '⚛️' },
    'map-ppxf-fitting': { label: 'pPXF Fitting', icon: '📐' }
  };

export const MAP_CHECKBOX_IDS = [
  'map-stellar-velocity',
  'map-stellar-velocity-error',
  'map-velocity-dispersion',
  'map-velocity-dispersion-error',
  'map-h3',
  'map-h4',
  'map-age-weighted',
  'map-age-mass-weighted',
  'map-metallicity',
  'map-ppxf-fitting'
];

export const KINEMATICS_CHECKBOXES = [
  { id: 'map-stellar-velocity', label: 'Stellar velocity' },
  { id: 'map-stellar-velocity-error', label: 'Stellar velocity Error' },
  { id: 'map-velocity-dispersion', label: 'Velocity dispersion' },
  { id: 'map-velocity-dispersion-error', label: 'Velocity dispersion Error' },
  { id: 'map-h3', label: 'h3' },
  { id: 'map-h4', label: 'h4' }
];

export const POPULATION_CHECKBOXES = [
  { id: 'map-age-weighted', label: 'Age (lum. weighted)' },
  { id: 'map-age-mass-weighted', label: 'Age (Mass Weighted)' },
  { id: 'map-metallicity', label: 'Metallicity' }
];

export const PPXF_CHECKBOXES = [
  { id: 'map-ppxf-fitting', label: 'pPXF Fitting' }
];

export const DISPLAY_CHECKBOXES = [
  { id: 'display-grid', label: 'Coordinate Grid' },
  { id: 'display-reticle', label: 'Center Reticle' },
  { id: 'display-labels', label: 'Object Labels' },
  { id: 'display-healpix', label: 'HEALPix Grid' }
];

export const MODAL_DIMENSIONS = { widthPx: 928, heightPx: 500 };
export const TIMEOUTS = { restoreGalleryMs: 100, hideCoordsMs: 1000, objectResolutionMs: 2000 };
export const RETICLE = { color: '#ff89ba', size: 22 };
export const DEFAULTS = { survey: 'P/DSS2/color', fov: 1.5 };
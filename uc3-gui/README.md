# AC³ Astronomy Maps

A modern web application for exploring astronomical data using interactive sky maps and FITS image visualization. Built with React and Aladin Lite v3 for professional astronomical research and education.

## Features

### 🌌 Interactive Sky Navigation
- **Aladin Lite v3 Integration**: Professional-grade astronomical sky viewer
- **Multiple Survey Support**: DSS2 Color, 2MASS, AllWISE, SDSS9, GLIMPSE
- **Galaxy Search**: Intelligent search with name resolution for common galaxies
- **Coordinate Grid & Overlays**: Customizable display options including HEALPix grids

### 🗺️ Map Visualization System
- **Dynamic Image Loading**: Automatically loads astronomical maps based on object selection
- **Proximity-Based Display**: Images only show when viewing the searched object (within 3 arcminutes)
- **Multiple Map Types**:
  - **Kinematics**: Stellar velocity, Velocity dispersion, Stellar/Velocity errors, h3, h4
  - **Stellar Populations**: Age (luminosity weighted), Age (mass weighted), Metallicity

### 🖼️ Gallery & Image Viewer
- **Interactive Gallery**: Click-to-view image gallery with modal viewer
- **Zoom & Pan**: Full image interaction with mouse and touch support
- **Smart Selection**: Only displays maps for selected sidebar options
- **Modal Viewer**: Full-screen image viewing with zoom controls

### ⌨️ Keyboard Controls
- **Navigation**: +/- for zoom, R for reset, G for goto
- **Quick Access**: F for format toggle, S for survey cycling
- **Help System**: H key for keyboard shortcuts

## Architecture

The application follows a modular component architecture:

```
src/
├── components/
│   ├── App.jsx              # Main application & Aladin initialization
│   ├── Sidebar.jsx          # Map selection & display controls
│   ├── Gallery.jsx          # Image gallery & modal viewer
│   ├── Controls.jsx         # Format, survey & search controls
│   ├── StatusBar.jsx        # Status information display
│   └── AladinInteractions.jsx # Mouse/keyboard interaction handling
├── assets/                  # Static images and logos
└── styles/                  # CSS styling
```

### Key Design Principles
- **Single Responsibility**: Each component handles one specific aspect
- **Modular Architecture**: Easy to maintain and extend
- **Coordinate-Based Logic**: Images tied to specific celestial coordinates
- **Dynamic Loading**: Images loaded based on naming conventions

## Installation & Setup

### Prerequisites
- Node.js (v16 or higher)
- npm or yarn package manager

### Installation
```bash
# Clone the repository
git clone [repository-url]
cd uc3-gui

# Install dependencies
npm install

# Start development server
npm run dev
```

### Production Build
```bash
# Build for production
npm run build

# Preview production build
npm run preview
```

## Usage

### Basic Navigation
1. **Search for Objects**: Use the Galaxy Search to find celestial objects
2. **Select Map Types**: Choose from Kinematics or Stellar Populations options
3. **View Images**: Gallery automatically populates with available maps
4. **Explore**: Use mouse/keyboard to navigate the sky

### Image Naming Convention
The application expects images in the assets folder with the format:
```
OBJECTNAME_maptype.extension
```

Examples:
- `NGC7025_stellar_velocity.jpg`
- `NGC7025_metallicity.png`
- `M31_age_mass_weighted.fits`

### Supported Map Types
- `stellar_velocity` - Stellar Velocity
- `velocity_dispersion` - Velocity Dispersion  
- `stellar_velocity_error` - Stellar Velocity Error
- `velocity_dispersion_error` - Velocity Dispersion Error
- `h3` - H3 moment
- `h4` - H4 moment
- `age` - Age (Luminosity Weighted)
- `age_mass_weighted` - Age (Mass Weighted)
- `metallicity` - Metallicity

## Configuration

### Display Options
- **Coordinate Grid**: Toggle coordinate overlay
- **Center Reticle**: Show/hide center crosshair
- **Object Labels**: Display catalog object labels
- **HEALPix Grid**: Show HEALPix tessellation

### Survey Options
- **DSS2 Color**: Optical survey data
- **2MASS**: Near-infrared survey
- **AllWISE**: Mid-infrared all-sky survey
- **SDSS9**: Sloan Digital Sky Survey
- **GLIMPSE**: Galactic Legacy Infrared survey

## Technical Details

### Dependencies
- **React 18**: Modern React with hooks
- **Vite**: Fast build tool and dev server
- **Aladin Lite v3**: Astronomical visualization library
- **jQuery**: Required for Aladin Lite

### Browser Compatibility
- Requires WebGL2-compatible browser
- Modern browsers (Chrome 60+, Firefox 60+, Safari 12+)
- Mobile browser support for touch interactions

### Performance
- Dynamic script loading for optimal initial load
- Debounced coordinate checking (500ms)
- Efficient image existence checking
- Lazy loading of astronomical catalogs

## Development

### Component Structure
Each component is self-contained with its own setup functions and event handlers. The global window object is used for cross-component communication where necessary.

### Adding New Map Types
1. Add checkbox to `Sidebar.jsx`
2. Add configuration to `mapControls` object
3. Add map type definition to `Gallery.jsx` `mapTypes` array
4. Follow naming convention for image files

### Debugging
The application includes comprehensive console logging:
- `📍` Coordinate checking
- `🔄` Image loading
- `🖼️` Gallery operations
- `⌨️` Keyboard interactions

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

## License

[Add your license information here]

## Acknowledgments

- **Aladin Lite**: Centre de Données astronomiques de Strasbourg (CDS)
- **AC³ Project**: [Add project acknowledgments]
- **Astronomical Data**: Various sky surveys and catalogs

import React, { useState, useEffect } from 'react';
// Re-enabling FileUpload component
import FileUpload from './ProgressMonitor';
import PipelineProgressMonitor from './PipelineProgressMonitor';
// Temporarily commented out to test Aladin loading
// import EnvironmentStatus from './EnvironmentStatus';

function PipelineMonitor({ selectedApp }) {
  const [selectedDataset, setSelectedDataset] = useState('dataset1');
  const [isCollapsed, setIsCollapsed] = useState(false);

  const [datasets] = useState([
    {
      id: 'dataset1',
      name: 'NGC7025',
      type: 'MEGARA',
      status: 'ready',
      progress: 0,
      stage: 'Ready for processing'
    },
    {
      id: 'dataset2', 
      name: 'IC1683',
      type: 'MEGARA',
      status: 'processing',
      progress: 67,
      stage: 'Stellar population analysis'
    },
    {
      id: 'dataset3',
      name: 'NGC2906',
      type: 'MUSE',
      status: 'completed',
      progress: 100,
      stage: 'Analysis completed'
    },
    {
      id: 'dataset4',
      name: 'MANGA-8250',
      type: 'MANGA',
      status: 'queued',
      progress: 0,
      stage: 'Waiting in queue'
    }
  ]);

  // File data for each dataset
  const [fileData] = useState({
    dataset1: {
      input: [
        { name: 'NGC7025_LR-V_final_cube.fits', size: '2.3 GB', uploaded: '2024-01-15 14:30:22', status: 'processed' },
        { name: 'NGC7025_spectrum_xpos_00_ypos_01.txt', size: '156 KB', uploaded: '2024-01-15 14:30:45', status: 'processed' },
        { name: 'NGC7025_spectrum_xpos_00_ypos_02.txt', size: '158 KB', uploaded: '2024-01-15 14:30:47', status: 'processed' },
        { name: 'NGC7025_kinematic_info.txt', size: '8.2 KB', uploaded: '2024-01-15 14:30:50', status: 'ready' },
        { name: 'NGC7025_grid_example.in', size: '1.4 KB', uploaded: '2024-01-15 14:31:02', status: 'ready' }
      ],
      output: [
        { name: 'NGC7025_age_mass_weighted.fits', size: '1.8 GB', generated: '2024-01-15 15:45:12', status: 'ready' },
        { name: 'NGC7025_metallicity_light_weighted.fits', size: '1.8 GB', generated: '2024-01-15 15:45:18', status: 'ready' },
        { name: 'NGC7025_velocity_dispersion.fits', size: '1.7 GB', generated: '2024-01-15 15:45:22', status: 'ready' },
        { name: 'NGC7025_stellar_velocity.fits', size: '1.7 GB', generated: '2024-01-15 15:45:25', status: 'ready' }
      ]
    },
    dataset2: {
      input: [
        { name: 'IC1683_LR-R_final_cube.fits', size: '2.1 GB', uploaded: '2024-01-16 09:15:33', status: 'processing' },
        { name: 'IC1683_LR-V_final_cube.fits', size: '2.2 GB', uploaded: '2024-01-16 09:16:12', status: 'processing' },
        { name: 'IC1683_kinematic_info.txt', size: '7.8 KB', uploaded: '2024-01-16 09:16:45', status: 'queued' }
      ],
      output: [
        { name: 'IC1683_age_mass_weighted.fits', size: '1.9 GB', generated: 'Processing...', status: 'processing' },
        { name: 'IC1683_metallicity_light_weighted.fits', size: 'Processing...', generated: 'Processing...', status: 'queued' }
      ]
    },
    dataset3: {
      input: [
        { name: 'NGC2906_cube.fits', size: '3.4 GB', uploaded: '2024-01-10 16:22:18', status: 'processed' },
        { name: 'NGC2906_spectrum_files.tar.gz', size: '245 MB', uploaded: '2024-01-10 16:23:42', status: 'processed' },
        { name: 'NGC2906_kinematic_info.txt', size: '12.1 KB', uploaded: '2024-01-10 16:24:15', status: 'processed' }
      ],
      output: [
        { name: 'NGC2906_age_mass_weighted.fits', size: '2.8 GB', generated: '2024-01-10 18:45:33', status: 'ready' },
        { name: 'NGC2906_metallicity_light_weighted.fits', size: '2.8 GB', generated: '2024-01-10 18:46:12', status: 'ready' },
        { name: 'NGC2906_velocity_dispersion.fits', size: '2.7 GB', generated: '2024-01-10 18:46:45', status: 'ready' },
        { name: 'NGC2906_stellar_velocity.fits', size: '2.7 GB', generated: '2024-01-10 18:47:18', status: 'ready' },
        { name: 'NGC2906_starlight_summary.log', size: '67 KB', generated: '2024-01-10 18:47:55', status: 'ready' }
      ]
    },
    dataset4: {
      input: [
        { name: 'manga-8250-1902-LINCUBE.fits', size: '1.8 GB', uploaded: '2024-01-17 11:30:45', status: 'queued' },
        { name: 'manga-8250-1902-LOGCUBE.fits', size: '1.8 GB', uploaded: '2024-01-17 11:31:22', status: 'queued' }
      ],
      output: []
    }
  });

  const getDatasetStatusColor = (status) => {
    switch (status) {
      case 'completed': return '#68D391';
      case 'processing': return '#4FD1C5';
      case 'queued': return '#F6AD55';
      case 'ready': return '#9F7AEA';
      case 'error': return '#FC8181';
      default: return '#A0AEC0';
    }
  };

  const getFileStatusColor = (status) => {
    switch (status) {
      case 'ready': return '#4FD1C5';
      case 'processed': return '#68D391';
      case 'processing': return '#F6AD55';
      case 'queued': return '#9F7AEA';
      case 'error': return '#FC8181';
      default: return '#A0AEC0';
    }
  };

  const currentFiles = fileData[selectedDataset] || { input: [], output: [] };
  const selectedDatasetInfo = datasets.find(dataset => dataset.id === selectedDataset);
  const datasetName = selectedDatasetInfo ? selectedDatasetInfo.name : 'Unknown';

  return (
    <div className="pipeline-wrapper">
      <div className="pipeline-container">
        {/* Three-pane layout */}
        <div className={`pipeline-panes ${isCollapsed ? 'collapsed' : ''}`}>
        
        {/* Left Pane - Dataset Selection */}
        <div className="pipeline-pane datasets-pane">
          <div className="pane-header">
            <div className="pane-header-left">
              <button 
                className="collapse-toggle"
                onClick={() => setIsCollapsed(!isCollapsed)}
                title={isCollapsed ? "Expand Pipeline" : "Collapse Pipeline"}
              >
                <span className={`toggle-icon ${isCollapsed ? 'collapsed' : ''}`}>
                  {isCollapsed ? '▲' : '▼'}
                </span>
              </button>
              <h3>Datasets</h3>
            </div>
            <div className="pane-count">{datasets.length}</div>
          </div>
          <div className="pane-content">
            {datasets.map(dataset => (
              <button
                key={dataset.id}
                className={`dataset-item ${selectedDataset === dataset.id ? 'active' : ''}`}
                onClick={() => setSelectedDataset(dataset.id)}
              >
                <div className="dataset-info">
                  <div className="dataset-name">{dataset.name}</div>
                  <div className="dataset-type">{dataset.type}</div>
                </div>
                <div className="dataset-status">
                  <span 
                    className="status-dot"
                    style={{ backgroundColor: getDatasetStatusColor(dataset.status) }}
                    title={dataset.status}
                  ></span>
                </div>
              </button>
            ))}
          </div>
        </div>

        {/* Middle Pane - Input Files */}
        <div className="pipeline-pane files-pane">
          <div className="pane-header">
            <h3>Input Files - {datasetName}</h3>
            <div className="pane-count">{currentFiles.input.length}</div>
          </div>
          <div className="pane-content">
            {currentFiles.input.length > 0 ? (
              currentFiles.input.map((file, index) => (
                <div key={index} className="file-item">
                  <div className="file-info">
                    <div className="file-name">{file.name}</div>
                    <div className="file-details">
                      <span className="file-size">{file.size}</span>
                      <span className="file-timestamp">{file.uploaded}</span>
                    </div>
                  </div>
                  <div className="file-status">
                    <span 
                      className="status-dot"
                      style={{ backgroundColor: getFileStatusColor(file.status) }}
                      title={file.status}
                    ></span>
                  </div>
                </div>
              ))
            ) : (
              <div className="empty-pane">
                <div className="empty-icon">📁</div>
                <p>No input files</p>
              </div>
            )}
          </div>
        </div>

        {/* Right Pane - Output Files */}
        <div className="pipeline-pane files-pane">
          <div className="pane-header">
            <div className="pane-header-left">
              <h3>Output Files - {datasetName}</h3>
            </div>
            <div className="pane-header-right">
              <div className="pane-count">{currentFiles.output.length}</div>
              {currentFiles.output.some(file => file.status === 'ready') && (
                              <button className="download-all-btn">
                Download All
              </button>
              )}
            </div>
          </div>
          <div className="pane-content">
            {currentFiles.output.length > 0 ? (
              currentFiles.output.map((file, index) => (
                <div key={index} className="file-item">
                  <div className="file-info">
                    <div className="file-name">{file.name}</div>
                    <div className="file-details">
                      <span className="file-size">{file.size}</span>
                      <span className="file-timestamp">{file.generated}</span>
                    </div>
                  </div>
                  <div className="file-status">
                    <span 
                      className="status-dot"
                      style={{ backgroundColor: getFileStatusColor(file.status) }}
                      title={file.status}
                    ></span>
                  </div>
                </div>
              ))
            ) : (
              <div className="empty-pane">
                <div className="empty-icon">📄</div>
                <p>No output files</p>
              </div>
            )}
          </div>
        </div>

        </div>
      </div>
      
      {/* File Upload - Re-enabled */}
      <FileUpload selectedDataset={selectedDataset} datasetName={datasetName} />
      
      {/* Pipeline Progress Monitor */}
      <PipelineProgressMonitor datasets={datasets} />
      
      {/* Environment Status - Temporarily commented out to test Aladin loading */}
      {/* <EnvironmentStatus /> */}
    </div>
  );
}

export default PipelineMonitor; 
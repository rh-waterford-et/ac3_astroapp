import React from 'react';
import PropTypes from 'prop-types';
import VirtualizedFileList from './VirtualizedFileList';
import { truncateFileName } from '../../utils/file/fileUtils';
import { getFileStatusColor } from '../../utils/ui/styleUtils';
import RefreshIcon from '../ui/RefreshIcon';
import DownloadIcon from '../ui/DownloadIcon';
import { getDownloadAllUrl } from '../../services/api';

const FilesPane = ({
  title,
  filesData,
  selectedDataset,
  onDeleteFile,
  onProcessFile,
  processorType
}) => {
  const { files, loading, error, pagination, refresh, loadMoreFiles } = filesData;
  
  const isOutputPane = title.toLowerCase().includes('output');

  const handleDownloadAll = () => {
    if (!selectedDataset || files.length === 0) return;
    
    const totalFiles = pagination.total > 0 ? pagination.total : files.length;
    
    const confirmed = window.confirm(
      `Download all ${totalFiles} output files from ${selectedDataset} as a zip file?`
    );
    
    if (!confirmed) return;
    
    console.log(`[FilesPane] Initiating bulk zip download for dataset="${selectedDataset}", processor="${processorType}"`);
    
    // Create download URL
    const downloadUrl = getDownloadAllUrl(selectedDataset, processorType, 'output');
    
    // Trigger download
    const link = document.createElement('a');
    link.href = downloadUrl;
    link.style.display = 'none';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    
    console.log(`[FilesPane] Zip download initiated for dataset="${selectedDataset}"`);
  };

  const handleDownloadFile = (fileKey, fileName) => {
    console.log(`[FilesPane] Downloading file: ${fileName}, key: ${fileKey}`);
    
    // Create download URL for single file
    const downloadUrl = `/api/files/download?key=${encodeURIComponent(fileKey)}`;
    
    // Trigger download
    const link = document.createElement('a');
    link.href = downloadUrl;
    link.download = fileName;
    link.style.display = 'none';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    
    console.log(`[FilesPane] Download initiated for file: ${fileName}`);
  };

  return (
    <div className="pipeline-pane files-pane">
      <div className="pane-header">
        <div className="pane-header-left">
          <h3>{title}</h3>
        </div>
        <div className="pane-header-right">
          {isOutputPane && (
            <button 
              className="download-btn" 
              onClick={handleDownloadAll}
              disabled={loading || !selectedDataset || pagination.total === 0}
              title={`Download all ${title.toLowerCase()} as zip`}
            >
              <DownloadIcon />
            </button>
          )}
          <button 
            className="refresh-btn" 
            onClick={refresh}
            disabled={loading || !selectedDataset}
            title={`Refresh ${title.toLowerCase()}`}
          >
            <RefreshIcon />
          </button>
          <div className="pane-count">
            {pagination.total > 0 ? pagination.total : files.length}
          </div>
        </div>
      </div>
      <div className="pane-content">
        {!loading && !error && files.length === 0 && selectedDataset ? (
          <div className="empty-pane">
            <div className="empty-icon">📁</div>
            <p>No {title.toLowerCase()} available</p>
          </div>
        ) : !selectedDataset ? (
          <div className="empty-pane">
            <div className="empty-icon">📂</div>
            <p>Select a dataset to view {title.toLowerCase()}</p>
          </div>
        ) : loading ? (
          <div className="astro-loading-container" style={{ padding: '0.5rem 0', gap: '0.5rem' }}>
            <div className="astro-loader-galaxy" style={{ width: '24px', height: '24px' }}></div>
            <div className="astro-loading-text" style={{ fontSize: '12px' }}>Loading {title.toLowerCase()}...</div>
          </div>
        ) : error ? (
          <div className="error-pane">
            <div className="error-icon">⚠️</div>
            <p style={{ fontSize: '12px', color: '#FF6B6B' }}>{error}</p>
          </div>
        ) : (
          <VirtualizedFileList
            items={files}
            isLoading={loading}
            error={error}
            emptyMessage={`No ${title.toLowerCase()} available`}
            emptyIcon="📁"
            selectedDataset={selectedDataset}
            onDelete={onDeleteFile}
            onProcessFile={title.includes('Input') ? onProcessFile : undefined}
            onDownloadFile={isOutputPane ? handleDownloadFile : undefined}
            loadingMessage={`Loading ${title.toLowerCase()}...`}
            onLoadMore={loadMoreFiles}
            hasNextPage={pagination.hasMore}
            isLoadingMore={pagination.loading}
            itemHeight={48}
            truncateFileName={truncateFileName}
            getFileStatusColor={getFileStatusColor}
            isInputFile={title.includes('Input')}
            processorType={processorType}
          />
        )}
      </div>
    </div>
  );
};

FilesPane.propTypes = {
  title: PropTypes.string.isRequired,
  filesData: PropTypes.shape({
    files: PropTypes.array.isRequired,
    loading: PropTypes.bool.isRequired,
    error: PropTypes.string,
    pagination: PropTypes.object.isRequired,
    refresh: PropTypes.func.isRequired
  }).isRequired,
  selectedDataset: PropTypes.string,
  onDeleteFile: PropTypes.func.isRequired,
  onProcessFile: PropTypes.func,
  processorType: PropTypes.string.isRequired
};

export default FilesPane; 
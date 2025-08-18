import React from 'react';
import PropTypes from 'prop-types';
import VirtualizedFileList from './VirtualizedFileList';
import { truncateFileName } from '../../utils/file/fileUtils';
import { getFileStatusColor } from '../../utils/ui/styleUtils';

const FilesPane = ({
  title,
  filesData,
  selectedDataset,
  onDeleteFile,
  onProcessFile,
  processorType
}) => {
  const { files, loading, error, pagination, refresh, loadMoreFiles } = filesData;

  return (
    <div className="pipeline-pane files-pane">
      <div className="pane-header">
        <div className="pane-header-left">
          <h3>{title}</h3>
        </div>
        <div className="pane-header-right">
          <button 
            className="refresh-btn" 
            onClick={refresh}
            disabled={loading || !selectedDataset}
            title={`Refresh ${title.toLowerCase()}`}
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M23 4v6h-6"/>
              <path d="M1 20v-6h6"/>
              <path d="m3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
            </svg>
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
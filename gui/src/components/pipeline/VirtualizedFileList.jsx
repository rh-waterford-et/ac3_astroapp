import React, { useCallback, useRef } from 'react';
import { FixedSizeList as List } from 'react-window';
import InfiniteLoader from 'react-window-infinite-loader';
import AutoSizer from 'react-virtualized-auto-sizer';
import PropTypes from 'prop-types';

const VirtualizedFileList = ({ 
  items = [], 
  isLoading = false, 
  error = null,
  emptyMessage = "No files available",
  emptyIcon = "📁",
  selectedDataset = null,
  onDelete = null,
  onProcessFile = null,
  onDownloadFile = null,
  loadingMessage = "Loading files...",
  onLoadMore = null,
  hasNextPage = false,
  isLoadingMore = false,
  totalCount = 0,
  itemHeight = 60,
  truncateFileName = (name) => name,
  isInputFile = true,
  processorType = 'starlight',
  // Dataset-specific props
  isDatasetMode = false,
  selectedDatasetId = null,
  onSelectDataset = null,
  onStartProcessing = null,
  isConnectorMode = false
}) => {
  const listRef = useRef();

  // Calculate total item count for react-window
  // Only use loaded items + 1 for loader, don't create huge scrollable areas
  const itemCount = hasNextPage ? items.length + 1 : items.length;

  // Check if item is loaded
  const isItemLoaded = useCallback((index) => {
    return index < items.length;
  }, [items]);

  // Load more items
  const loadMoreItems = useCallback(async (startIndex, stopIndex) => {
    if (onLoadMore && !isLoadingMore) {
      try {
        await onLoadMore(startIndex, stopIndex);
      } catch (err) {
        console.error('Error loading more items:', err);
      }
    }
  }, [onLoadMore, isLoadingMore]);

  // File item component - styled to match current theme
  const FileItem = React.memo(({ index, style }) => {
    const item = items[index];
    
    // If index is beyond our loaded items, show loader only at the boundary
    if (index >= items.length) {
      if (hasNextPage && index === items.length) {
        return (
          <div style={style} className="file-loader-container">
            <div className="file-loader-item">
              <div className="astro-loading-container" style={{ padding: '0.5rem 0', gap: '0.5rem' }}>
                <div className="astro-loader-galaxy" style={{ width: '16px', height: '16px' }}></div>
                <div className="astro-loading-text" style={{ fontSize: '10px' }}>Loading more {isDatasetMode ? 'datasets' : 'files'}...</div>
              </div>
            </div>
          </div>
        );
      }
      return <div style={style}></div>;
    }

    // Regular item (file or dataset)
    if (!item) {
      return <div style={style}></div>;
    }

    // Dataset item rendering
    if (isDatasetMode) {
      const isSelected = selectedDatasetId === item.id;
      
      return (
        <div style={style}>
          <div className={`dataset-item-container ${isSelected ? 'active' : ''}`}>
            <button
              className="dataset-item"
              onClick={() => onSelectDataset && onSelectDataset(item.id)}
            >
              <div className="dataset-info">
                <div className="dataset-name">{item.name}</div>
              </div>
            </button>
            <div className="dataset-actions">
              {onStartProcessing && (
                <button
                  className={`dataset-process-btn ${isConnectorMode ? 'connector-mode' : ''}`}
                  onClick={(e) => {
                    e.stopPropagation();
                    if (!isConnectorMode) {
                      onStartProcessing(item.name);
                    }
                  }}
                  disabled={isConnectorMode}
                  title={
                    isConnectorMode 
                      ? `Waiting for files to transfer to consumer bucket - "${item.name}"` 
                      : `Start processing "${item.name}"`
                  }
                >
                  {isConnectorMode ? '⏸' : '▶'}
                </button>
              )}
              {onDelete && (
                <button
                  className="dataset-delete-btn"
                  onClick={(e) => {
                    e.stopPropagation();
                    onDelete(item.id, item.name);
                  }}
                  title={`Delete dataset "${item.name}"`}
                >
                  ×
                </button>
              )}
            </div>
          </div>
        </div>
      );
    }

    // File item rendering (existing logic)
    return (
      <div style={style}>
        <div className="file-item-container">
          <div className="file-item">
            <div className="file-info">
              <div className="file-name" title={item.name} style={{ whiteSpace: 'nowrap', overflow: 'hidden' }}>
                {truncateFileName(item.name)}
              </div>
              <div className="file-details">
                <div className="file-size">{item.size}</div>
              </div>
            </div>

          </div>
          {onProcessFile && isInputFile && (
            <button 
              type="button"
              className="file-process-btn"
              aria-label={`Process file ${item.name} with ${processorType}`}
              onClick={(e) => {
                e.stopPropagation();
                onProcessFile(item.name);
              }}
              title={`Process file "${item.name}" with ${processorType}`}
            >
              ▶
            </button>
          )}
          {onDownloadFile && !isInputFile && (
            <button 
              type="button"
              className="file-download-btn"
              aria-label={`Download file ${item.name}`}
              onClick={(e) => {
                e.stopPropagation();
                onDownloadFile(item.key, item.name);
              }}
              title={`Download file "${item.name}"`}
            >
              ⬇
            </button>
          )}
          {onDelete && !isDatasetMode && (
            <button 
              type="button"
              className="file-delete-btn"
              aria-label={`Delete file ${item.name}`}
              onClick={(e) => {
                e.stopPropagation();
                onDelete(item.key, item.name, isInputFile);
              }}
              title={`Delete file "${item.name}"`}
            >
              ×
            </button>
          )}
        </div>
      </div>
    );
  });

  FileItem.displayName = 'FileItem';

  // Empty state
  if (!isDatasetMode && !selectedDataset) {
    return (
      <div className="empty-pane">
        <div className="empty-icon">📂</div>
        <p>Select a dataset to view files</p>
      </div>
    );
  }

  // Loading state (initial load)
  if (isLoading && items.length === 0) {
    return (
      <div className="astro-loading-container" style={{ padding: '0.5rem 0', gap: '0.5rem' }}>
        <div className="astro-loader-galaxy" style={{ width: '24px', height: '24px' }}></div>
        <div className="astro-loading-text" style={{ fontSize: '12px' }}>{loadingMessage}</div>
      </div>
    );
  }

  // Error state
  if (error) {
    return (
      <div className="empty-pane">
        <div className="empty-icon">❌</div>
        <p>Error loading files</p>
        <p style={{ fontSize: '12px', color: '#FF6B6B' }}>{error}</p>
      </div>
    );
  }

  // Empty state (no files)
  if (!isLoading && items.length === 0) {
    return (
      <div className="empty-pane">
        <div className="empty-icon">{emptyIcon}</div>
        <p>{emptyMessage}</p>
      </div>
    );
  }

  // Virtualized list with infinite loading
  return (
    <div style={{ height: '100%', width: '100%' }}>
      <AutoSizer>
        {({ height, width }) => (
          <InfiniteLoader
            isItemLoaded={isItemLoaded}
            itemCount={itemCount}
            loadMoreItems={loadMoreItems}
            threshold={5} // Start loading when 5 items from the end
          >
            {({ onItemsRendered, ref }) => (
              <List
                ref={(list) => {
                  ref(list);
                  listRef.current = list;
                }}
                height={height}
                width={width}
                itemCount={itemCount}
                itemSize={itemHeight}
                itemKey={(index) => items[index]?.key || items[index]?.name || index}
                onItemsRendered={onItemsRendered}
                overscanCount={10} // Render 10 extra items for smoother scrolling
              >
                {FileItem}
              </List>
            )}
          </InfiniteLoader>
        )}
      </AutoSizer>
    </div>
  );
};

VirtualizedFileList.propTypes = {
  items: PropTypes.array,
  isLoading: PropTypes.bool,
  error: PropTypes.string,
  emptyMessage: PropTypes.string,
  emptyIcon: PropTypes.string,
  selectedDataset: PropTypes.string,
  onDelete: PropTypes.func,
  onProcessFile: PropTypes.func,
  onDownloadFile: PropTypes.func,
  loadingMessage: PropTypes.string,
  onLoadMore: PropTypes.func,
  hasNextPage: PropTypes.bool,
  isLoadingMore: PropTypes.bool,

  itemHeight: PropTypes.number,
  truncateFileName: PropTypes.func,

  isInputFile: PropTypes.bool,
  processorType: PropTypes.string,
  // Dataset-specific props
  isDatasetMode: PropTypes.bool,
  selectedDatasetId: PropTypes.string,
  onSelectDataset: PropTypes.func,
  onStartProcessing: PropTypes.func,
  isConnectorMode: PropTypes.bool
};

export default VirtualizedFileList; 
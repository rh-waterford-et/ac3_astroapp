import React, { useState, useEffect, useCallback, useRef } from 'react';
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
  loadingMessage = "Loading files...",
  onLoadMore = null,
  hasNextPage = false,
  isLoadingMore = false,
  totalCount = 0,
  itemHeight = 60,
  truncateFileName = (name) => name,
  getFileStatusColor = (status) => '#4CAF50',
  isInputFile = true
}) => {
  const [localItems, setLocalItems] = useState([]);
  const listRef = useRef();

  // Update local items when props change
  useEffect(() => {
    setLocalItems(items);
  }, [items]);

  // Calculate total item count for react-window
  // Only use loaded items + 1 for loader, don't create huge scrollable areas
  const itemCount = hasNextPage ? localItems.length + 1 : localItems.length;

  // Check if item is loaded
  const isItemLoaded = useCallback((index) => {
    return index < localItems.length;
  }, [localItems]);

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
    const file = localItems[index];
    
    // If index is beyond our loaded items, show loader only at the boundary
    if (index >= localItems.length) {
      if (hasNextPage && index === localItems.length) {
        // Show loader only for the first unloaded item
        return (
          <div style={style}>
            <div className="file-loader-container">
              <div className="file-loader-item">
                <div className="astro-loading-container" style={{ padding: '0.5rem 0', gap: '0.5rem' }}>
                  <div className="astro-loader-galaxy" style={{ width: '16px', height: '16px' }}></div>
                  <div className="astro-loading-text" style={{ fontSize: '10px' }}>Loading more files...</div>
                </div>
              </div>
            </div>
          </div>
        );
      } else {
        // For all other unloaded items, render empty space
        return <div style={style}></div>;
      }
    }

    // Regular file item
    if (!file) {
      return <div style={style}></div>;
    }

    return (
      <div style={style}>
        <div className="file-item-container">
          <div className="file-item">
            <div className="file-info">
              <div className="file-name" title={file.name} style={{ whiteSpace: 'nowrap', overflow: 'hidden' }}>
                {truncateFileName(file.name)}
              </div>
              <div className="file-details">
                <div className="file-size">{file.size}</div>
              </div>
            </div>
            <div className="file-status" style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <span 
                className="status-dot"
                style={{ backgroundColor: getFileStatusColor(file.status) }}
                title={file.status}
              ></span>
            </div>
          </div>
          {onDelete && (
            <button 
              className="file-delete-btn"
              onClick={(e) => {
                e.stopPropagation();
                onDelete(file.key, file.name, isInputFile);
              }}
              title={`Delete file "${file.name}"`}
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
  if (!selectedDataset) {
    return (
      <div className="empty-pane">
        <div className="empty-icon">📂</div>
        <p>Select a dataset to view files</p>
      </div>
    );
  }

  // Loading state (initial load)
  if (isLoading && localItems.length === 0) {
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
  if (!isLoading && localItems.length === 0) {
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
  loadingMessage: PropTypes.string,
  onLoadMore: PropTypes.func,
  hasNextPage: PropTypes.bool,
  isLoadingMore: PropTypes.bool,
  totalCount: PropTypes.number,
  itemHeight: PropTypes.number,
  truncateFileName: PropTypes.func,
  getFileStatusColor: PropTypes.func,
  isInputFile: PropTypes.bool
};

export default VirtualizedFileList; 
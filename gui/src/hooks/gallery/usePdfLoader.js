import { useCallback } from 'react';
import { 
  getCachedPdfs, 
  setCachedPdfs, 
  normalizeObjectName,
  extractCellNumber,
  generatePdfDisplayName
} from '../../utils/gallery/galleryUtils';
import { getDatasetOutputFilesPaginated } from '../../services/api.js';

// Module-level PDF cache for progressive loading (shared across hook instances)
const pdfCache = new Map();

export const usePdfLoader = (onPdfLoaded, onClearPdfs) => {
  
  // Main entry point for loading PDF files
  const tryLoadPpxfPdfFiles = useCallback(async (objectName) => {
    
    // Clear existing H4 items using callback
    if (onClearPdfs) {
      onClearPdfs();
    }
    
    // Normalize object name for S3 folder lookup
    const normalizedObjectName = normalizeObjectName(objectName);
    
    try {
      // Progressive loading with cache
      return await loadPdfsProgressively(normalizedObjectName, objectName, null);
    } catch (error) {
      return 0;
    }
  }, [onPdfLoaded, onClearPdfs]);

  // Progressive loading with cache
  const loadPdfsProgressively = useCallback(async (normalizedObjectName, objectName, totalPdfs) => {
    
    // 1. Check cache first - show immediately if available
    const cached = getCachedPdfs(normalizedObjectName, 'ppxf', pdfCache);
    let pdfsLoaded = 0;
    
    if (cached.batches.length > 0) {
      cached.batches.forEach(batch => {
        batch.files.forEach(pdfFile => {
          pdfsLoaded += addPdfToGallery(pdfFile, objectName);
        });
      });
      
      // If cache is complete and we know the total, return early
      if (totalPdfs && cached.total === totalPdfs) {
        return pdfsLoaded;
      }
    }
    
    // 2. Load first batch (0-49) using paginated API
    try {
      const firstBatch = await getDatasetOutputFilesPaginated(normalizedObjectName, 'ppxf', 50, 0);
      
      // Add first batch to gallery
      let firstBatchLoaded = 0;
      firstBatch.files.forEach(pdfFile => {
        firstBatchLoaded += addPdfToGallery(pdfFile, objectName);
      });
      pdfsLoaded += firstBatchLoaded;
      
      // 3. Update cache with first batch
      setCachedPdfs(normalizedObjectName, 'ppxf', {
        batches: [{ offset: 0, files: firstBatch.files }],
        total: firstBatch.total
      }, pdfCache);
      
      // 4. Start background loading for remaining batches
      if (firstBatch.hasMore) {
        const remainingText = firstBatch.total > 0 ? `${firstBatch.total - firstBatch.files.length} PDFs` : 'additional PDFs';
        loadRemainingBatches(normalizedObjectName, objectName, 50);
      }
      
      const backgroundText = firstBatch.total > 0 ? `${firstBatch.total - firstBatch.files.length}` : 'unknown number of';
      return pdfsLoaded;
      
    } catch (error) {
      return pdfsLoaded; // Return any cached PDFs that were loaded
    }
  }, [onPdfLoaded]);

  // Background batch loading
  const loadRemainingBatches = useCallback(async (normalizedObjectName, objectName, batchSize) => {
    let offset = batchSize; // Start from second batch (first batch already loaded)
    let hasMore = true;
    let totalLoaded = batchSize; // Track how many we've loaded so far
    
    while (hasMore) {
      try {
        const batch = await getDatasetOutputFilesPaginated(normalizedObjectName, 'ppxf', batchSize, offset);
        
        // Add to gallery state
        batch.files.forEach(pdfFile => {
          addPdfToGallery(pdfFile, objectName);
        });
        
        // Update cache
        const cached = getCachedPdfs(normalizedObjectName, 'ppxf', pdfCache);
        cached.batches.push({ offset, files: batch.files });
        // Update total in cache if we now know it
        if (batch.total > 0) {
          cached.total = batch.total;
        }
        setCachedPdfs(normalizedObjectName, 'ppxf', cached, pdfCache);
        
        totalLoaded += batch.files.length;
        hasMore = batch.hasMore;
        
        const totalText = batch.total > 0 ? `/${batch.total}` : '';
        
        // Move to next batch
        offset += batchSize;
        
        // Safety break to prevent infinite loops
        if (offset > 10000) {
          break;
        }
        
      } catch (error) {
        // Stop loading on error to prevent infinite retries
        break;
      }
    }
    
  }, [onPdfLoaded]);

  // Helper function to add PDF to gallery (calls the callback)
  const addPdfToGallery = useCallback((pdfFile, objectName) => {
    const cellNumber = extractCellNumber(pdfFile.name);
    const displayName = generatePdfDisplayName(cellNumber);
    
    if (onPdfLoaded) {
      const pdfItem = {
        id: `pdf-${cellNumber}-${Date.now()}`,
        type: 'pdf',
        pdfFile,
        objectName
      };
      
      const added = onPdfLoaded(pdfItem);
      if (added) {
        return 1;
      } else {
        return 0;
      }
    }
    
    return 0;
  }, [onPdfLoaded]);

  return {
    tryLoadPpxfPdfFiles,
    loadPdfsProgressively,
    loadRemainingBatches
  };
}; 
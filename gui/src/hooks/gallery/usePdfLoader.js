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
    console.log(`🔄 Loading H4 PDF files for: ${objectName}`);
    
    // Clear existing H4 items using callback
    if (onClearPdfs) {
      onClearPdfs();
    }
    console.log(`🧹 Cleared existing H4 items for ${objectName}`);
    
    // Normalize object name for S3 folder lookup
    const normalizedObjectName = normalizeObjectName(objectName);
    console.log(`📁 Normalized object name: "${objectName}" → "${normalizedObjectName}"`);
    
    try {
      // Progressive loading with cache
      console.log(`📄 Starting progressive PDF loading for ${normalizedObjectName}`);
      return await loadPdfsProgressively(normalizedObjectName, objectName, null);
    } catch (error) {
      console.error(`❌ Error loading H4 PDF files for ${normalizedObjectName} (original: ${objectName}):`, error);
      return 0;
    }
  }, [onPdfLoaded, onClearPdfs]);

  // Progressive loading with cache
  const loadPdfsProgressively = useCallback(async (normalizedObjectName, objectName, totalPdfs) => {
    console.log(`📄 Progressive loading for ${normalizedObjectName}: ${totalPdfs || 'unknown'} total PDFs`);
    
    // 1. Check cache first - show immediately if available
    const cached = getCachedPdfs(normalizedObjectName, 'ppxf', pdfCache);
    let pdfsLoaded = 0;
    
    if (cached.batches.length > 0) {
      console.log(`💨 Loading ${cached.total} cached PDFs immediately`);
      cached.batches.forEach(batch => {
        batch.files.forEach(pdfFile => {
          pdfsLoaded += addPdfToGallery(pdfFile, objectName);
        });
      });
      
      // If cache is complete and we know the total, return early
      if (totalPdfs && cached.total === totalPdfs) {
        console.log(`✅ Cache complete: ${pdfsLoaded} PDFs loaded from cache`);
        return pdfsLoaded;
      }
    }
    
    // 2. Load first batch (0-49) using paginated API
    try {
      const firstBatch = await getDatasetOutputFilesPaginated(normalizedObjectName, 'ppxf', 50, 0);
      console.log(`📄 First batch loaded: ${firstBatch.files.length} PDFs (0-${firstBatch.files.length - 1})`);
      
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
        console.log(`🔄 Starting background loading for remaining ${remainingText}`);
        loadRemainingBatches(normalizedObjectName, objectName, 50);
      }
      
      const backgroundText = firstBatch.total > 0 ? `${firstBatch.total - firstBatch.files.length}` : 'unknown number of';
      console.log(`🎯 Progressive loading started: ${pdfsLoaded} PDFs loaded immediately, ${backgroundText} loading in background`);
      return pdfsLoaded;
      
    } catch (error) {
      console.error(`❌ Error in progressive PDF loading:`, error);
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
        console.log(`📄 Background batch loaded: ${batch.files.length} PDFs (${offset}-${offset + batch.files.length - 1})`);
        
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
        console.log(`✅ Background batch complete: ${totalLoaded}${totalText} PDFs loaded`);
        
        // Move to next batch
        offset += batchSize;
        
        // Safety break to prevent infinite loops
        if (offset > 10000) {
          console.warn('⚠️ Safety break: stopped loading after 10000 offset');
          break;
        }
        
      } catch (error) {
        console.error(`❌ Error loading background batch at offset ${offset}:`, error);
        // Stop loading on error to prevent infinite retries
        break;
      }
    }
    
    console.log(`🎉 All background loading complete: ${totalLoaded} PDFs loaded for ${normalizedObjectName}`);
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
        console.log(`✅ Added H4 PDF to gallery: Cell ${cellNumber}`);
        return 1;
      } else {
        console.log(`⏭️ Skipping ${displayName}: already exists`);
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
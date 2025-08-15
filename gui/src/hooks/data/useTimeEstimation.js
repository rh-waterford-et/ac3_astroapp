import { useCallback } from 'react';
import { mean, median, standardDeviation } from 'simple-statistics';

export const useTimeEstimation = (processorType) => {

  // Get estimated completion time based on processing status and history
  const getEstimatedTime = useCallback((progress, status, filesProcessed, filesTotal, processingHistory = []) => {
    if (status === 'completed') return 'Completed';
    if (status === 'queued') return 'Waiting to start';
    if (status === 'ready') return 'Ready to start';
    if (status === 'error') return 'Error occurred';
    
    if (status === 'processing' && filesProcessed > 0 && filesTotal > 0) {
      // Calculate remaining files correctly based on processor type
      let remainingFiles;
      let actualFilesProcessed;
      
      if (processorType === 'ppxf') {
        // For pPXF: filesTotal = input files, filesProcessed = output files
        // Each input file produces 5 output files, so divide by 5 to get input files processed
        actualFilesProcessed = Math.floor(filesProcessed / 5);
        remainingFiles = filesTotal - actualFilesProcessed;
      } else {
        // For STARLIGHT: 1:1 ratio, so use as-is
        actualFilesProcessed = filesProcessed;
        remainingFiles = filesTotal - filesProcessed;
      }
      
      if (remainingFiles <= 0) return 'Finalizing...';
      
      // Debug logging
      console.log(`Time estimation for ${processorType}: ${filesTotal} input files, ${filesProcessed} output files, ${actualFilesProcessed} input files processed, ${remainingFiles} remaining`);
      console.log(`Processing history available:`, processingHistory.length, 'entries');
      
      // Use actual processing history if available
      if (processingHistory.length > 0) {
        const recentHistory = processingHistory.slice(-10); // Use last 10 files
        const processingTimes = recentHistory.map(h => h.processingTime / 1000 / 60); // Convert to minutes
        
        console.log(`Using processing history:`, processingTimes.map(t => `${t.toFixed(2)}m`));
        
        let estimatedMinutesPerFile;
        
        if (processingTimes.length >= 3) {
          // Use statistical analysis for better estimates
          const avgTime = mean(processingTimes);
          const medianTime = median(processingTimes);
          const stdDev = standardDeviation(processingTimes);
          
          console.log(`Stats - Avg: ${avgTime.toFixed(2)}m, Median: ${medianTime.toFixed(2)}m, StdDev: ${stdDev.toFixed(2)}m`);
          
          // Use median if there's high variance, otherwise use mean
          if (stdDev > avgTime * 0.5) {
            estimatedMinutesPerFile = medianTime;
            console.log(`High variance detected, using median: ${medianTime.toFixed(2)}m`);
          } else {
            estimatedMinutesPerFile = avgTime;
            console.log(`Low variance, using mean: ${avgTime.toFixed(2)}m`);
          }
          
          // Add buffer for uncertainty (10-20% based on standard deviation)
          const uncertaintyBuffer = Math.min(0.2, stdDev / avgTime * 0.5);
          estimatedMinutesPerFile *= (1 + uncertaintyBuffer);
          
        } else {
          // Use simple average for small sample sizes
          estimatedMinutesPerFile = mean(processingTimes);
          console.log(`Small sample, using simple average: ${estimatedMinutesPerFile.toFixed(2)}m`);
        }
        
        const estimatedMinutes = remainingFiles * estimatedMinutesPerFile;
        console.log(`Final estimate: ${remainingFiles} files × ${estimatedMinutesPerFile.toFixed(2)}m = ${estimatedMinutes.toFixed(1)}m`);
        
        if (estimatedMinutes > 60) {
          const hours = Math.ceil(estimatedMinutes / 60);
          return `~${hours}h remaining`;
        }
        return `~${Math.ceil(estimatedMinutes)}m remaining`;
      }
      
      // Improved fallback estimate based on user feedback: 10 minutes total / ~40-50 files = ~15 seconds per file
      console.log(`No processing history, using fallback estimate`);
      const estimatedSeconds = remainingFiles * 15; // 15 seconds per file
      const estimatedMinutes = estimatedSeconds / 60;
      
      if (estimatedMinutes > 60) {
        return `~${Math.ceil(estimatedMinutes / 60)}h remaining`;
      }
      return `~${Math.ceil(estimatedMinutes)}m remaining`;
    }
    
    return 'Calculating...';
  }, [processorType]);

  // Get statistical analysis of processing history
  const getProcessingStats = useCallback((processingHistory = []) => {
    if (processingHistory.length === 0) return null;
    
    const recentHistory = processingHistory.slice(-10);
    const processingTimes = recentHistory.map(h => h.processingTime / 1000 / 60); // Convert to minutes
    
    if (processingTimes.length < 2) return null;
    
    const avgTime = mean(processingTimes);
    const medianTime = median(processingTimes);
    const stdDev = standardDeviation(processingTimes);
    
    return {
      avgTime: avgTime.toFixed(1),
      medianTime: medianTime.toFixed(1),
      stdDev: stdDev.toFixed(1),
      sampleSize: processingTimes.length
    };
  }, []);

  return {
    getEstimatedTime,
    getProcessingStats
  };
}; 
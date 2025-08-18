import { useEffect, useCallback } from 'react';

export const useAutoRefresh = (selectedDataset, refreshCallbacks, intervalMs = 5000) => {
  // Combined background refresh function
  const performBackgroundRefresh = useCallback(async () => {
    if (!selectedDataset) return;
    
    console.log('🔄 Performing smart count refresh...');
    
    // Call all provided refresh callbacks
    await Promise.all(refreshCallbacks.filter(callback => typeof callback === 'function'));
    
    console.log('✅ Smart count refresh completed');
  }, [selectedDataset, refreshCallbacks]);

  // Auto-refresh timer - silent background updates
  useEffect(() => {
    if (!selectedDataset) return;
    
    console.log('🔧 Setting up background refresh timer for dataset:', selectedDataset);
    
    const interval = setInterval(() => {
      performBackgroundRefresh();
    }, intervalMs);

    return () => {
      console.log('🧹 Clearing background refresh timer');
      clearInterval(interval);
    };
  }, [selectedDataset, performBackgroundRefresh, intervalMs]);

  return {
    performBackgroundRefresh
  };
}; 
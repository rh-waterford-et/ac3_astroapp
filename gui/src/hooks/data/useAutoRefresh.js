import { useEffect, useCallback } from 'react';

export const useAutoRefresh = (selectedDataset, refreshCallbacks, intervalMs = 5000) => {
  // Combined background refresh function
  const performBackgroundRefresh = useCallback(async () => {
    if (!selectedDataset) return;
    
    // Call all provided refresh callbacks
    await Promise.all(refreshCallbacks.filter(callback => typeof callback === 'function'));
    
  }, [selectedDataset, refreshCallbacks]);

  // Auto-refresh timer - silent background updates
  useEffect(() => {
    if (!selectedDataset) return;
    
    const interval = setInterval(() => {
      performBackgroundRefresh();
    }, intervalMs);

    return () => {
      clearInterval(interval);
    };
  }, [selectedDataset, performBackgroundRefresh, intervalMs]);

  return {
    performBackgroundRefresh
  };
}; 
import { useEffect, useCallback } from 'react';

export const useAutoRefresh = (selectedDataset, refreshCallbacks, intervalMs = 60000) => {

  const performBackgroundRefresh = useCallback(async () => {
    if (!selectedDataset) return;
    
    // Call all provided refresh callbacks
    const validCallbacks = refreshCallbacks.filter(callback => typeof callback === 'function');
    await Promise.all(validCallbacks.map(callback => callback()));
    
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
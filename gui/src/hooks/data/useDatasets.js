import { useState, useEffect, useCallback } from 'react';
import { getDatasets as apiGetDatasets, createDataset as apiCreateDataset } from '../../services/api';

export default function useDatasets(processorType) {
  const [availableDatasets, setAvailableDatasets] = useState([]);
  const [currentDataset, setCurrentDataset] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  const refresh = useCallback(async (showLoading = true, autoSelectIfEmpty = true) => {
    if (showLoading) setLoading(true);
    setError(null);
    try {
      const names = await apiGetDatasets(processorType);
      setAvailableDatasets(names);
      if (autoSelectIfEmpty && !currentDataset && names.length > 0) {
        setCurrentDataset(names[0]);
      }
    } catch (err) {
      setError(err?.message || 'Failed to load datasets');
    } finally {
      if (showLoading) setLoading(false);
    }
  }, [processorType, currentDataset]);

  // Clear all data when processor type changes
  useEffect(() => {
    setAvailableDatasets([]);
    setCurrentDataset('');
    setError(null);
    setLoading(false);
  }, [processorType]);

  useEffect(() => {
    // Initial fetch for this processor; keep previous list until new data arrives
    refresh(true, true);
  }, [processorType, refresh]);

  const createDataset = useCallback(async (name, configOrNull = null, isConnectorMode = false) => {
    try {
      const result = await apiCreateDataset(name, processorType, configOrNull, isConnectorMode);
      return result;
    } catch (err) {
      return { success: false, message: err?.message || 'Failed to create dataset' };
    }
  }, [processorType]);

  return {
    availableDatasets,
    currentDataset,
    setCurrentDataset,
    loading,
    error,
    refresh,
    createDataset,
  };
} 
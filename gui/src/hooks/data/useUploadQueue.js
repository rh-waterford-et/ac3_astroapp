import { useState, useCallback } from 'react';
import { uploadFiles as apiUploadFiles } from '../../services/api';

export default function useUploadQueue(processorType, isConnectorMode = false) {
  const [queue, setQueue] = useState([]);

  const addFiles = useCallback((fileList) => {
    const items = Array.from(fileList).map(file => ({
      id: Date.now() + Math.random(),
      file,
      name: file.name,
      sizeBytes: file.size,
      status: 'ready',
      progress: 0,
    }));
    setQueue(prev => [...prev, ...items]);
  }, []);

  const remove = useCallback((id) => {
    setQueue(prev => prev.filter(f => f.id !== id));
  }, []);

  const clear = useCallback(() => setQueue([]), []);
  const clearCompleted = useCallback(() => setQueue(prev => prev.filter(f => f.status !== 'completed')), []);

  const formatSize = (bytes) => {
    if (!bytes) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`;
  };

  const totalSize = () => queue.reduce((sum, f) => sum + (f.sizeBytes || 0), 0);

  const uploadAll = useCallback(async (dataset) => {
    if (!dataset) throw new Error('Dataset is required');
    const filesToUpload = queue.filter(q => q.status === 'ready').map(q => q.file);
    if (filesToUpload.length === 0) return [];

    setQueue(prev => prev.map(f => ({ ...f, status: 'uploading', progress: 0 })));

    const results = await apiUploadFiles(
      filesToUpload,
      dataset,
      (file, progress) => {
        setQueue(prev => prev.map(item => item.file === file ? { ...item, progress } : item));
      },
      null,
      processorType,
      isConnectorMode
    );

    setQueue(prev => prev.map(item => {
      const match = results.find(r => r.file === item.file);
      if (!match) return item;
      return { ...item, status: match.success ? 'completed' : 'error', progress: match.success ? 100 : 0 };
    }));

    return results;
  }, [processorType, queue, isConnectorMode]);

  return {
    queue,
    addFiles,
    remove,
    clear,
    clearCompleted,
    formatSize,
    totalSize,
    uploadAll,
  };
} 
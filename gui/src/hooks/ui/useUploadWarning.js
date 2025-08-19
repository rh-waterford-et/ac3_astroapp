import { useEffect } from 'react';

export const useUploadWarning = () => {
  useEffect(() => {
    const handleBeforeUnload = (e) => {
      // Check if any uploads are in progress
      const uploadElements = document.querySelectorAll('[data-upload-status="uploading"]');
      if (uploadElements.length > 0) {
        e.preventDefault();
        e.returnValue = 'File uploads are in progress. Are you sure you want to leave?';
        return 'File uploads are in progress. Are you sure you want to leave?';
      }
    };

    window.addEventListener('beforeunload', handleBeforeUnload);
    
    return () => {
      window.removeEventListener('beforeunload', handleBeforeUnload);
    };
  }, []);
}; 
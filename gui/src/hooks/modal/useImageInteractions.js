// Image interactions hook - manages zoom, pan, and drag functionality
import { useState, useCallback, useRef, useEffect } from 'react';

export const useImageInteractions = (imageRef) => {
  const [transform, setTransform] = useState({ x: 0, y: 0, zoom: 1 });
  const [isDragging, setIsDragging] = useState(false);
  
  // Refs for interaction state
  const dragStartRef = useRef({ x: 0, y: 0 });
  const lastTouchDistanceRef = useRef(0);
  const lastTouchCenterRef = useRef({ x: 0, y: 0 });

  /**
   * Update transform and apply to image element
   */
  const updateTransform = useCallback((newTransform) => {
    setTransform(newTransform);
    if (imageRef.current) {
      imageRef.current.style.transform = `translate(${newTransform.x}px, ${newTransform.y}px) scale(${newTransform.zoom})`;
    }
  }, [imageRef]);

  /**
   * Reset image to default position and zoom
   */
  const resetTransform = useCallback(() => {
    const resetTransform = { x: 0, y: 0, zoom: 1 };
    updateTransform(resetTransform);
  }, [updateTransform]);

  /**
   * Handle mouse wheel zoom
   */
  const handleWheel = useCallback((e) => {
    e.preventDefault();
    
    if (!imageRef.current) return;
    
    const rect = imageRef.current.getBoundingClientRect();
    const centerX = rect.left + rect.width / 2;
    const centerY = rect.top + rect.height / 2;
    
    const zoomFactor = e.deltaY > 0 ? 0.9 : 1.1;
    const newZoom = Math.max(0.1, Math.min(5, transform.zoom * zoomFactor));
    
    // Zoom towards mouse position
    const mouseX = e.clientX - centerX;
    const mouseY = e.clientY - centerY;
    
    const newX = transform.x + mouseX * (1 - zoomFactor);
    const newY = transform.y + mouseY * (1 - zoomFactor);
    
    updateTransform({ x: newX, y: newY, zoom: newZoom });
  }, [transform, updateTransform, imageRef]);

  /**
   * Handle mouse drag start
   */
  const handleMouseDown = useCallback((e) => {
    setIsDragging(true);
    dragStartRef.current = {
      x: e.clientX - transform.x,
      y: e.clientY - transform.y
    };
    
    if (imageRef.current) {
      imageRef.current.classList.add('dragging');
    }
    
    e.preventDefault();
  }, [transform, imageRef]);

  /**
   * Handle mouse drag move
   */
  const handleMouseMove = useCallback((e) => {
    if (!isDragging) return;
    
    const newX = e.clientX - dragStartRef.current.x;
    const newY = e.clientY - dragStartRef.current.y;
    
    updateTransform({ ...transform, x: newX, y: newY });
    e.preventDefault();
  }, [isDragging, transform, updateTransform]);

  /**
   * Handle mouse drag end
   */
  const handleMouseUp = useCallback(() => {
    setIsDragging(false);
    
    if (imageRef.current) {
      imageRef.current.classList.remove('dragging');
    }
  }, [imageRef]);

  /**
   * Get touch distance for pinch zoom
   */
  const getTouchDistance = useCallback((touches) => {
    const dx = touches[0].clientX - touches[1].clientX;
    const dy = touches[0].clientY - touches[1].clientY;
    return Math.sqrt(dx * dx + dy * dy);
  }, []);

  /**
   * Get touch center point
   */
  const getTouchCenter = useCallback((touches) => {
    return {
      x: (touches[0].clientX + touches[1].clientX) / 2,
      y: (touches[0].clientY + touches[1].clientY) / 2
    };
  }, []);

  /**
   * Handle touch start
   */
  const handleTouchStart = useCallback((e) => {
    e.preventDefault();
    
    if (e.touches.length === 1) {
      // Single touch - dragging
      setIsDragging(true);
      const touch = e.touches[0];
      dragStartRef.current = {
        x: touch.clientX - transform.x,
        y: touch.clientY - transform.y
      };
      
      if (imageRef.current) {
        imageRef.current.classList.add('dragging');
      }
    } else if (e.touches.length === 2) {
      // Two touches - pinch zoom
      setIsDragging(false);
      
      if (imageRef.current) {
        imageRef.current.classList.remove('dragging');
      }
      
      lastTouchDistanceRef.current = getTouchDistance(e.touches);
      lastTouchCenterRef.current = getTouchCenter(e.touches);
    }
  }, [transform, getTouchDistance, getTouchCenter, imageRef]);

  /**
   * Handle touch move
   */
  const handleTouchMove = useCallback((e) => {
    e.preventDefault();
    
    if (e.touches.length === 1 && isDragging) {
      // Single touch drag
      const touch = e.touches[0];
      const newX = touch.clientX - dragStartRef.current.x;
      const newY = touch.clientY - dragStartRef.current.y;
      
      updateTransform({ ...transform, x: newX, y: newY });
    } else if (e.touches.length === 2) {
      // Pinch zoom
      const newDistance = getTouchDistance(e.touches);
      const newCenter = getTouchCenter(e.touches);
      
      if (lastTouchDistanceRef.current > 0 && imageRef.current) {
        const zoomFactor = newDistance / lastTouchDistanceRef.current;
        const newZoom = Math.max(0.1, Math.min(5, transform.zoom * zoomFactor));
        
        // Zoom towards touch center
        const rect = imageRef.current.getBoundingClientRect();
        const imageCenterX = rect.left + rect.width / 2;
        const imageCenterY = rect.top + rect.height / 2;
        
        const touchX = newCenter.x - imageCenterX;
        const touchY = newCenter.y - imageCenterY;
        
        const newX = transform.x + touchX * (1 - zoomFactor);
        const newY = transform.y + touchY * (1 - zoomFactor);
        
        updateTransform({ x: newX, y: newY, zoom: newZoom });
      }
      
      lastTouchDistanceRef.current = newDistance;
      lastTouchCenterRef.current = newCenter;
    }
  }, [isDragging, transform, updateTransform, getTouchDistance, getTouchCenter, imageRef]);

  /**
   * Handle touch end
   */
  const handleTouchEnd = useCallback((e) => {
    if (e.touches.length === 0) {
      setIsDragging(false);
      lastTouchDistanceRef.current = 0;
      
      if (imageRef.current) {
        imageRef.current.classList.remove('dragging');
      }
    } else if (e.touches.length === 1) {
      // Switch back to dragging mode
      const touch = e.touches[0];
      dragStartRef.current = {
        x: touch.clientX - transform.x,
        y: touch.clientY - transform.y
      };
      setIsDragging(true);
      
      if (imageRef.current) {
        imageRef.current.classList.add('dragging');
      }
      
      lastTouchDistanceRef.current = 0;
    }
  }, [transform, imageRef]);

  /**
   * Handle double-click/tap to reset
   */
  const handleDoubleClick = useCallback(() => {
    resetTransform();
  }, [resetTransform]);

  // Set up event listeners
  useEffect(() => {
    const image = imageRef.current;
    if (!image) return;

    // Mouse events
    image.addEventListener('wheel', handleWheel, { passive: false });
    image.addEventListener('mousedown', handleMouseDown);
    image.addEventListener('dblclick', handleDoubleClick);
    
    // Touch events
    image.addEventListener('touchstart', handleTouchStart, { passive: false });
    
    // Document events (for drag continuation outside image)
    document.addEventListener('mousemove', handleMouseMove);
    document.addEventListener('mouseup', handleMouseUp);
    document.addEventListener('touchmove', handleTouchMove, { passive: false });
    document.addEventListener('touchend', handleTouchEnd);

    return () => {
      // Cleanup
      image.removeEventListener('wheel', handleWheel);
      image.removeEventListener('mousedown', handleMouseDown);
      image.removeEventListener('dblclick', handleDoubleClick);
      image.removeEventListener('touchstart', handleTouchStart);
      
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleMouseUp);
      document.removeEventListener('touchmove', handleTouchMove);
      document.removeEventListener('touchend', handleTouchEnd);
    };
  }, [
    handleWheel, handleMouseDown, handleMouseMove, handleMouseUp,
    handleTouchStart, handleTouchMove, handleTouchEnd, handleDoubleClick,
    imageRef
  ]);

  return {
    transform,
    isDragging,
    resetTransform,
    updateTransform
  };
}; 
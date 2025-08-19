import React from 'react';
import PropTypes from 'prop-types';

const RefreshIcon = ({ width = 14, height = 14 }) => (
  <svg 
    width={width} 
    height={height} 
    viewBox="0 0 24 24" 
    fill="none" 
    stroke="currentColor" 
    strokeWidth="2" 
    strokeLinecap="round" 
    strokeLinejoin="round"
  >
    <path d="M23 4v6h-6"/>
    <path d="M1 20v-6h6"/>
    <path d="m3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
  </svg>
);

RefreshIcon.propTypes = {
  width: PropTypes.number,
  height: PropTypes.number
};

export default RefreshIcon; 
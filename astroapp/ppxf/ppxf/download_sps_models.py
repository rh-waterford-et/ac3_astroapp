#!/usr/bin/env python3

import os
from urllib import request
from os import path

def download_sps_models():
    """Download SPS model files to replace Git LFS pointer files"""
    
    # Directory containing the SPS models
    ssp_dir = "run_ppxf/ssp_models/"
    
    # SPS model files to download
    model_files = [
        "spectra_emiles_9.0.npz",
        "spectra_coelho_9.0.npz", 
        "spectra_coelho_mini_9.0.npz",
        "spectra_fsps_9.0.npz",
        "spectra_galaxev_9.0.npz",
        "spectra_sun_vega.npz"
    ]
    
    base_url = "https://raw.githubusercontent.com/micappe/ppxf_data/main/"
    
    for filename in model_files:
        filepath = os.path.join(ssp_dir, filename)
        
        # Only download if file doesn't exist or is very small (LFS pointer)
        if not path.isfile(filepath) or os.path.getsize(filepath) < 1000:
            url = base_url + filename
            print(f"Downloading {filename}...")
            try:
                request.urlretrieve(url, filepath)
                print(f"Successfully downloaded {filename} ({os.path.getsize(filepath)} bytes)")
            except Exception as e:
                print(f"Failed to download {filename}: {e}")
                # Continue with other files even if one fails

if __name__ == "__main__":
    download_sps_models() 
#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Created on Fri Feb 28 16:19:24 2025

@author: mario
"""

import os
from astropy.io import fits
import numpy as np
import matplotlib.pyplot as plt
from os import path
import sys
# from urllib import request

from ppxf.ppxf import ppxf, robust_sigma
import ppxf.ppxf_util as util
import ppxf.sps_util as lib

from astropy import units as u
from specutils import Spectrum1D
from specutils.manipulation import FluxConservingResampler, LinearInterpolatedResampler, SplineInterpolatedResampler

import argparse

parser = argparse.ArgumentParser(description='Script for analyzing individual spectrum')
parser.add_argument('--filenames', nargs='+', type=str, help='Paths to the spectra files to be analyzed')
parser.add_argument('--filenames_file', type=str, help='Text file containing spectra filenames')
parser.add_argument('--wave-range', nargs=2, type=float, metavar=('start', 'end'),
                    help='Range of wavelengths to focus on')
parser.add_argument('--suffix', type=str, default='', help='Optional label to append to the output file names')
parser.add_argument('--sps-name', choices=['emiles', 'fsps', 'galaxev', 'coelho', 'coelho_mini'], type=str, default='emiles', metavar='SPS_NAME',
                    help='Name of the SPS model to be used (default: emiles)')
parser.add_argument('--redshift', type=float, default=0.0, metavar='REDSHIFT',
                    help='Redshift of the galaxy (default: 0)')
parser.add_argument('--velocity-dispersion', type=float, default=100.0, metavar='VEL_DISP',
                    help='Initial guess for the velocity dispersion of the galaxy (default: 100 km/s)')
parser.add_argument('--mask-file', help='Path to the file containing the wavelength intervals you do not want to take into account')
parser.add_argument('--output-dir', type=str, default='./', metavar='OUTPUT_DIR',
                    help='Directory where the output FITS files will be saved (default: current directory)')

args = parser.parse_args()

# ==========================
# Function Definitions
# ==========================

class read_individual_spectrum(object):
    def __init__(self, spectra_filename, wave_range):
        """
        Read MEGARA cube, log rebin it and compute coordinates of each spaxel.
        
        """
        filename = spectra_filename
        print("=" * 80)
        print(f"Analyzing file:", filename)
        print("=" * 80)
        hdu = fits.open(filename)
        head = hdu[0].header
        spectrum = hdu[0].data   
        
        vph_parameters = {
            'LR-U': (0.672, 4051),
            'LR-B': (0.792, 4800),
            'LR-V': (0.937, 5695),
            'LR-R': (1.106, 6747),
            'LR-I': (1.308, 7991),
            'LR-Z': (1.455, 8900),
            'MR-U': (0.326, 4104),
            'MR-UB': (0.358, 4431),
            'MR-B': (0.395, 4814),
            'MR-G': (0.433, 5213),
            'MR-V': (0.476, 5667),
            'MR-VR': (0.522, 6170),
            'MR-R': (0.558, 6563),
            'MR-RI': (0.608, 7115),
            'MR-I': (0.666, 7767),
            'MR-Z': (0.796, 9262),
            'HR-R': (0.355, 6646),
            'HR-I': (0.462, 8634)
        }
        
        try:
            red = head['VPH']
            print("=" * 80)
            print("Observation VPH:", red)
            print("=" * 80)
            FWHM_VPH, lambda_c = vph_parameters.get(red, (0.937, 5695))  # Default values for missing 'VPH' keys
        except KeyError:
            instrument = head['INSTRUME']
            print("=" * 80)
            print("Instrument observed with:", instrument)
            print("=" * 80)
            if instrument == 'MaNGA':
                FWHM_VPH, lambda_c = 2, 7000 # AA MaNGA
            elif instrument == 'MUSE':
                FWHM_VPH, lambda_c = 2.4, 7050 # AA MUSE
            else:
                FWHM_VPH, lambda_c = 0.937, 5695  # Default values if instrument is not recognized
                                        
        # Transform cube into 2-dim array of spectra
        npix = spectrum.shape[0]
        wave = head['CRVAL1'] + head['CDELT1']*np.arange(npix)

        # Only use a restricted wavelength range
        w = (wave > wave_range[0]) & (wave < wave_range[1])
        wave = wave[w]
        spectrum = spectrum[w]
        spectrum = np.maximum(spectrum, 0)
        
        
        c = 299792.458  # speed of light in km/s
        velscale = c*np.diff(np.log(wave[-2:]))  # Smallest velocity step
        lam_range_temp = [np.min(wave), np.max(wave)]
        spectrum, ln_lam_gal, velscale = util.log_rebin(lam_range_temp, spectrum, velscale=velscale)

        self.original_data = spectrum
        self.header = head
        self.spectrum = spectrum
        self.ln_lam_gal = ln_lam_gal
        self.lam_gal = wave
        self.fwhm_gal = FWHM_VPH
        self.lambda_c = lambda_c

        
## Function to iteratively clip the outliers

def clip_outliers(galaxy, bestfit, goodpixels):
    """
    Repeat the fit after clipping bins deviants more than 3*sigma
    in relative error until the bad bins don't change any more.
    """
    while True:
        scale = galaxy[goodpixels] @ bestfit[goodpixels]/np.sum(bestfit[goodpixels]**2)
        resid = scale*bestfit[goodpixels] - galaxy[goodpixels]
        err = robust_sigma(resid, zero=1)
        ok_old = goodpixels
        goodpixels = np.flatnonzero(np.abs(bestfit - galaxy) < 3*err)
        if np.array_equal(goodpixels, ok_old):
            break
            
    return goodpixels

## Function to fit the stellar kinematics
#The following function fits the spectrum with `pPXF` while masking the gas emission lines, then iteratively clips the outliers and finally refit the spectrum with `pPXF` on the cleaned spectrum.

def fit_and_clean(templates, galaxy, velscale, start, goodpixels0, lam, lam_temp, plot_title):
    
    print('##############################################################')
    goodpixels = goodpixels0.copy()
    pp = ppxf(templates, galaxy, np.ones_like(galaxy), velscale, start,
              moments=4, degree=4, mdegree=4, lam=lam, lam_temp=lam_temp,
              goodpixels=goodpixels, plot_title=plot_title, reddening=2)
    
    #plt.figure(figsize=(8, 3.7))
    #plt.subplot(121)
    #pp.plot_mine()

    goodpixels = clip_outliers(galaxy, pp.bestfit, goodpixels)

    # Add clipped pixels to the original masked emission lines regions and repeat the fit
    goodpixels = np.intersect1d(goodpixels, goodpixels0)
    pp = ppxf(templates, galaxy, np.ones_like(galaxy), velscale, start,
              moments=4, degree=4, mdegree=4, lam=lam, lam_temp=lam_temp,
              goodpixels=goodpixels, plot_title=plot_title, reddening=2)
    
    #plt.subplot(122)
    pp.plot_mine()


    optimal_template = templates @ pp.weights
    
    return pp, optimal_template


def create_mask(lam, exclude_ranges, apply_redshift, z_galaxy):
    """
    Create a boolean mask to exclude specific wavelength ranges, optionally
    considering the redshift of the galaxy.

    Parameters:
    -----------
    lam : array_like
        Array of observed wavelengths.

    exclude_ranges : list of tuples
        List of tuples specifying the rest-frame wavelength ranges to exclude.
        Each tuple should contain the start and end wavelengths of the range.

    apply_redshift : list of bool
        List of boolean values indicating whether to apply the redshift to
        each exclusion range.

    z_galaxy : float
        Redshift of the galaxy.

    Returns:
    --------
    mask : array_like
        Boolean mask with the same shape as `lam`, where `True` indicates
        wavelengths to be excluded.
    """
    mask = np.ones_like(lam, dtype=bool)
    for i, (start, end) in enumerate(exclude_ranges):
        observed_start = start * (1 + z_galaxy) if apply_redshift[i] else start
        observed_end = end * (1 + z_galaxy) if apply_redshift[i] else end
        mask &= (lam < observed_start) | (lam > observed_end)
    return mask

def read_mask_info(mask_file):
    """
    Reads mask information from a text file.

    Parameters:
    -----------
    mask_file : str
        Name of the file containing the mask information.

    Returns:
    --------
    exclude_ranges : list of tuples
        List of tuples specifying the wavelength ranges to exclude.

    apply_redshift : list of bool
        List of boolean values indicating whether to apply redshift or not to each range.
    """
    exclude_ranges = []
    apply_redshift = []
    with open(mask_file, 'r') as f:
        for line in f:
            parts = line.split()
            if len(parts) >= 3:
                start, end, apply_z = parts[0], parts[1], parts[2]
                exclude_ranges.append((float(start), float(end)))
                apply_redshift.append(apply_z.lower() == 'true')
    return exclude_ranges, apply_redshift


def save_results_as_fits(result_data, original_file, new_filename, keywords=None):
    """
    Saves the result data as a new FITS file with the header extracted from the original file.
    Optionally modifies specified keywords in the header.

    Parameters:
    result_data (array_like): The data to be saved in the FITS file.
    original_file (str): The path to the original FITS file from which to extract the header.
    new_filename (str): The desired filename for the new FITS file.
    keywords (dict, optional): A dictionary of keywords and their new values to be updated in the header.

    Returns:
    None
    """
    # Read the header from the original file
    with fits.open(original_file) as hdul:
        header = hdul[0].header

    # Modify the keywords with the new values if provided
    if keywords:
        for key, value in keywords.items():
            header[key] = value
    
    # Create a new FITS extension with the result data and the original header
    new_hdu = fits.PrimaryHDU(data=result_data, header=header)

    # Save the FITS file
    new_hdu.writeto(new_filename, overwrite=True)

def get_age_metal_weights(self, weights):
    """
    Get the age, metallicity, and their corresponding weights.

    Args:
        weights (numpy.ndarray): Array of weights returned by pPXF.

    Returns:
        tuple: A tuple containing three arrays:
            - age_values: Array of age values.
            - metal_values: Array of metallicity values.
            - weight_values: Array of corresponding weights.
    """
    # Initialize lists to store values
    age_values = []
    metal_values = []
    weight_values = []

    # Iterate over each grid point
    for i in range(self.age_grid.shape[0]):
        for j in range(self.age_grid.shape[1]):
            # Get age, metallicity, and weight at grid point (i, j)
            age = self.age_grid[i, j]
            metallicity = self.metal_grid[i, j]
            weight = weights[i, j]

            # Append values to lists
            age_values.append(age)
            metal_values.append(metallicity)
            weight_values.append(weight)

    # Convert lists to numpy arrays
    age_values = np.array(age_values)
    metal_values = np.array(metal_values)
    weight_values = np.array(weight_values)

    return age_values, metal_values, weight_values

def correct_sigma(fwhm_gal, fwhm_template, sigma_ppxf, central_wavelength):
    """
    Correct the sigma values when instrumental dispersion is smaller than
    the template dispersion.

    Args:
        fwhm_gal (float): FWHM of the galaxy in angstrom.
        fwhm_template (float): FWHM of the template in angstrom.
        sigma_ppxf (float): pPXF measured dispersion.
        central_wavelength (float): Central wavelength in angstrom.

    Returns:
        float: Corrected sigma value.
    """
    # Convert FWHM to sigma in km/s
    c = 299792.458  # Speed of light in km/s
    
    sigma_gal = (fwhm_gal / (2 * np.sqrt(2 * np.log(2)))) * (c / central_wavelength)
    sigma_template = (fwhm_template / (2 * np.sqrt(2 * np.log(2)))) * (c / central_wavelength)
    
    # Calculate the difference in sigma
#    sigma_diff_sq = np.sqrt(sigma_gal**2 - sigma_template**2)
    
    # Correct sigma
#    corrected_sigma = np.sqrt(sigma_ppxf**2 - sigma_diff_sq)
    corrected_sigma = np.sqrt(sigma_ppxf**2 - (sigma_gal**2 - sigma_template**2))
    return corrected_sigma

def rebin_spectrum(original_spectrum, original_wavelength, final_wavelength, method):
    """
    Rebins a spectrum to a new wavelength array using the specified method.

    Parameters:
    original_spectrum (array): The original spectrum array.
    original_wavelength (array): The original wavelength array.
    final_wavelength (array): The new wavelength array to rebin the spectrum to.
    method (str): The rebinning method ('flux_conserving', 'linear', 'spline').

    Returns:
    Spectrum1D: The rebinned spectrum.
    """
    
    # Create Spectrum1D object from original data
    input_spec = Spectrum1D(spectral_axis=original_wavelength * u.AA, flux=original_spectrum * (u.erg / (u.cm**2 * u.s * u.AA)))
    
    # Choose the resampling method
    if method == 'flux_conserving':
        resampler = FluxConservingResampler()
    elif method == 'linear':
        resampler = LinearInterpolatedResampler()
    elif method == 'spline':
        resampler = SplineInterpolatedResampler()
    else:
        raise ValueError("Invalid method. Choose from 'flux_conserving', 'linear', or 'spline'.")
    
    # Rebin the spectrum
    rebinned_spectrum = resampler(input_spec, final_wavelength * u.AA)
    
    return rebinned_spectrum

# ==========================
# Main Script
# ==========================

# Choose the spectra to be fitted
if args.filenames_file:
    with open(args.filenames_file, 'r') as file:
        spectra_filenames = [line.strip() for line in file.readlines()]
else:
    spectra_filenames = args.filenames

first_spectra_filename = spectra_filenames[0]

# Read header to get wave range if not provided
if args.wave_range is None:
    with fits.open(first_spectra_filename) as hdul:
        header = hdul[0].header
        if 'WAVLIMF1' in header and 'WAVLIMF2' in header:
            wave_range = [header['WAVLIMF1'], header['WAVLIMF2']]
        else:
            # Use full wavelength range of observation
            # Extract wavelength range from header
            crval3 = header['CRVAL1']  # starting wavelength
            cdelt3 = header['CDELT1']  # wavelength step
            npix = header['NAXIS1']    # number of pixels in the wavelength direction
            wave_range = [crval3, crval3 + (npix - 1) * cdelt3]
else:
    wave_range = args.wave_range

print("=" * 80)
print("Using wavelength range:", wave_range)
print("=" * 80)

s = read_individual_spectrum(first_spectra_filename, wave_range)

c_kms = 299792.458  # speed of light in km/s
velscale = c_kms*np.diff(s.ln_lam_gal[:2])   # eq.(8) of Cappellari (2017)
velscale = velscale[0]   # must be a scalar

# pPXF can be used with any set of SPS population templates. However, I am currently providing (with permission) ready-to-use template files for three SPS. One can just uncomment one of the three models below. The included files are only a subset of the SPS that can be produced with the models, and one should use the relevant software to produce different sets of SPS templates if needed.

# 1. If you use the [fsps v3.2](https://github.com/cconroy20/fsps) SPS model templates, please also cite [Conroy et al. (2009)](https://ui.adsabs.harvard.edu/abs/2009ApJ...699..486C) and [Conroy et al. (2010)](https://ui.adsabs.harvard.edu/abs/2010ApJ...712..833C) in your paper.

# 2. If you use the [GALAXEV v2000](http://www.bruzual.org/bc03/) SPS model templates, please also cite [Bruzual & Charlot (2003)](https://ui.adsabs.harvard.edu/abs/2003MNRAS.344.1000B) in your paper.

# 3. If you use the [E-MILES](http://miles.iac.es/) SPS model templates, please also cite [Vazdekis et al. (2016)](https://ui.adsabs.harvard.edu/abs/2016MNRAS.463.3409V) in your paper. 
# <font color='red'>WARNING: the E-MILES models do not include very young SPS and should not be used for highly star forming galaxies.</font>  

sps_name = args.sps_name

# Read SPS models file from my GitHub if not already in the pPXF package dir. I am not distributing the templates with pPXF anymore.
# The SPS model files are also available [this GitHub page](https://github.com/micappe/ppxf_data).

ppxf_dir = path.dirname(path.realpath(lib.__file__))
basename = f"spectra_{sps_name}_9.0.npz"
filename = path.join(ppxf_dir, 'sps_models', basename)

# Uncomment these lines to give the option of searching for the SSP model files on GitHub.

# if not path.isfile(filename):
#     url = "https://raw.githubusercontent.com/micappe/ppxf_data/main/" + basename
#     request.urlretrieve(url, filename)
    
FWHM_gal = s.fwhm_gal   # set this to None to skip convolution
sps = lib.sps_lib(filename, velscale, FWHM_gal, norm_range=[5300, 5950])
stars_templates, ln_lam_temp = sps.templates, sps.ln_lam_temp


# The stellar templates are reshaped into a 2-dim array with each spectrum as a column, however we save the original array dimensions, which are needed to specify the regularization dimensions

reg_dim = stars_templates.shape[1:]
stars_templates = stars_templates.reshape(stars_templates.shape[0], -1)

# See the pPXF documentation for the keyword REGUL, for an explanation of the following two lines.

stars_templates /= np.median(stars_templates) # Normalizes stellar templates by a scalar
regul_err = 0.01 # Desired regularization error

z = args.redshift  # redshift estimate from NED
print("=" * 80)
print("Galaxy redshift:", z)
print("=" * 80)
vel0 = c_kms*np.log(1 + z)  # Initial estimate of the galaxy velocity in km/s. eq. (8) of Cappellari (2017)
sigma = args.velocity_dispersion
print("Velocity dispersion initial guess:", sigma, "km/s")
print("=" * 80)
start = [vel0, sigma]  # (km/s), starting guess for [V,sigma]

lam_range_temp = np.exp(ln_lam_temp[[0, -1]])
goodpixels0 = util.determine_goodpixels(s.ln_lam_gal, lam_range_temp, z, width=1000)

lam_gal = np.exp(s.ln_lam_gal)

# Output FITS files header values
crval3 = lam_gal[0]  # Starting wavelength
cdelt3 = (lam_gal[-1] - lam_gal[0]) / lam_gal.size  # Wavelength increment per pixel (calculated from your original vector)
naxis3 = lam_gal.size  # Number of pixels (adjust as necessary)

# Generate the uniform wavelength vector
final_wavelength = crval3 + cdelt3 * np.arange(naxis3)

# Read mask information from the text file if mask_file argument is provided
if args.mask_file:
    # Create a complete array from 0 to the shape of lam_gal
    complete_array = np.arange(lam_gal.shape[0])    
    # Find the missing positions by comparing the complete array with goodpixels0
    missing_positions = np.setdiff1d(complete_array, goodpixels0)    
    # Read the mask information from the provided mask file
    exclude_ranges, apply_redshift = read_mask_info(args.mask_file)    
    # Create a mask based on the lambda array, exclude ranges, redshift application, and redshift value
    mask = create_mask(lam_gal, exclude_ranges, apply_redshift, z)    
    # Get the goodpixels that are masked
    goodpixels_masked = complete_array[mask]
else:
    goodpixels_masked = goodpixels0



# pPXF analysis

for spectra_filename in spectra_filenames:

    # Extract the spectrum information
    s = read_individual_spectrum(spectra_filename, wave_range)
    
    # Saving files nomenclature
    base_filename = os.path.basename(spectra_filename).replace('.fits', '')
    suffix = f"_{args.suffix}" if args.suffix else ''
    
        
    # Save the information about stellar pops SSP used for the fitting of each Voronoi bin
    voronoi_output_file_name = os.path.join(args.output_dir, f"{base_filename}_kinematics_and_stellar_pops_info{suffix}.txt")
    
    # pPXF analysis call
        
    galaxy = s.spectrum
    
    if np.all(galaxy == 0) == False:
    
        plot_title = str('pPXF fitting')
        
        pp, bestfit_template = fit_and_clean(stars_templates, galaxy, velscale, start, goodpixels_masked, lam_gal, sps.lam_temp, plot_title)
        velbin, sigbin, h3bin, h4bin = pp.sol  # Uncomment for fitting h3 and h4 and comment the previous line
        velbin_error, sigbin_error, h3bin_error, h4bin_error = pp.error*np.sqrt(pp.chi2)  # Uncomment for fitting h3 and h4 and comment the previous line
        optimal_templates= bestfit_template
        attbin = pp.reddening
        # Calculate the residuals for the current bin
        residuals = galaxy - pp.bestfit
        
        residulas_rebin = rebin_spectrum(residuals, lam_gal, final_wavelength, 'flux_conserving')
        bestfits_rebin = rebin_spectrum(pp.bestfit, lam_gal, final_wavelength, 'flux_conserving')
        galaxy_rebin = rebin_spectrum(galaxy, lam_gal, final_wavelength, 'flux_conserving')
        
        with open(voronoi_output_file_name, 'w') as voronoi_output_file:
            voronoi_output_file.write("Kinematics results:\n")
            voronoi_output_file.write("\n")
            voronoi_output_file.write("{:<20} {:<20} {:<30} {:<30} {:<15} {:<15} {:<15} {:<15}\n".format(
                "Velocity(km/s)", "Error(Velocity)", "Velocity_Dispersion(km/s)", "Error(Velocity_Dispersion)", "h3", "Error(h3)", "h4", "Error(h4)"))
            voronoi_output_file.write("{:<20.8f} {:<20.8f} {:<30.8f} {:<30.8f} {:<15.8f} {:<15.8f} {:<15.8f} {:<15.8f}\n".format(
                velbin, velbin_error, sigbin, sigbin_error, h3bin, h3bin_error, h4bin, h4bin_error))
            voronoi_output_file.write('*' * 162 + "\n")
            voronoi_output_file.write("\n")
        
        # Save figures of the fitting
    
        plt.savefig(os.path.join(args.output_dir, f"{base_filename}_pPXF_fitting{suffix}.pdf"),dpi=600)
        plt.close()
        
        light_weights = pp.weights.reshape(reg_dim)
        lg_age_bin, metalbin = sps.mean_age_metal(light_weights)
        
        #M_L_ratio_V = sps.mass_to_light(light_weights, band='V', redshift=args.redshift)
        
        with open(voronoi_output_file_name, 'a') as voronoi_output_file:
            voronoi_output_file.write("Stellar populations results:\n")
            voronoi_output_file.write("\n")
            #voronoi_output_file.write("{:<20} {:<20} {:<20} {:<15}\n".format("Weihgted_Age(Gyr)", "Weighted_Metallicity", "M*/L(V_band)", "A_V"))
            #voronoi_output_file.write("{:<20.8f} {:<20.8f} {:<20.8f} {:<15.8f}\n".format((10**lg_age_bin)/10**9, metalbin, M_L_ratio_V, attbin))
            voronoi_output_file.write("{:<20} {:<20} {:<15}\n".format("Weihgted_Age(Gyr)", "Weighted_Metallicity", "A_V"))
            voronoi_output_file.write("{:<20.8f} {:<20.8f} {:<15.8f}\n".format((10**lg_age_bin)/10**9, metalbin, attbin))
            voronoi_output_file.write('*' * 52 + "\n")
            voronoi_output_file.write("\n")
        
        # Save the SSP templates used in the fitting together with their respectives weights (weight_values_nonzero/np.sum(weight_values))
        
        age_values, metal_values, weight_values = get_age_metal_weights(sps, light_weights)
        
        # Get indices of values with nonzero weight
        nonzero_indices = weight_values > 0
        
        # Filter age, metallicity, and weight values
        age_values_nonzero = age_values[nonzero_indices]
        metal_values_nonzero = metal_values[nonzero_indices]
        weight_values_nonzero = weight_values[nonzero_indices]/np.sum(weight_values) # Normalized weights
        
        # Combine age, metallicity, and weight arrays into a single 2D array
        sp_data_info = np.column_stack((age_values_nonzero, metal_values_nonzero, weight_values_nonzero))
        
        # Write header
        with open(voronoi_output_file_name, 'a') as voronoi_output_file:
            voronoi_output_file.write("Templates used for the fitting:\n")
            voronoi_output_file.write("\n")
            voronoi_output_file.write("{:<20} {:<20} {:<20}\n".format("Age(Gyr)", "Metallicity", "Weight"))
        
        # Write data
        for age, metal, weight in sp_data_info:
            with open(voronoi_output_file_name, 'a') as voronoi_output_file:
                voronoi_output_file.write("{:<20.8f} {:<20.8f} {:<20.8f}\n".format(age, metal, weight))
        with open(voronoi_output_file_name, 'a') as voronoi_output_file:
            voronoi_output_file.write('*' * 52 + "\n")
        
        print("-"*50)
        print("Age values [Gyr]",age_values_nonzero)
        print("Metallicity values [M/H]",metal_values_nonzero)
        print("Weights [Normalized]",weight_values_nonzero)
        print("-"*50)
        
    # Correction of sigma in case the instrumental dispersion is smaller than the dispersion of the SSP models
    if FWHM_gal < sps.fwhm_tem[0]:
        corrected_sigbin = correct_sigma(FWHM_gal, sps.fwhm_tem[0], sigbin, s.lambda_c)
        corrected_sigbin_error = sigbin*sigbin_error/corrected_sigbin
    else:
        corrected_sigbin = sigbin
        corrected_sigbin_error = sigbin_error
        
    
    
    residuals_results_FITS = os.path.join(args.output_dir, f"{base_filename}_residuals{suffix}.fits")
    bestfits_results_FITS = os.path.join(args.output_dir, f"{base_filename}_bestfit{suffix}.fits")
    galaxy_results_FITS = os.path.join(args.output_dir, f"{base_filename}_galaxy{suffix}.fits")
    
    # Call the function to save the results as a FITS file
    keywords_to_update = {'CRVAL1': crval3, 'CDELT1': cdelt3}
    save_results_as_fits(residulas_rebin.data, spectra_filename, residuals_results_FITS, keywords_to_update)
    save_results_as_fits(bestfits_rebin.data, spectra_filename, bestfits_results_FITS, keywords_to_update)
    save_results_as_fits(galaxy_rebin.data, spectra_filename, galaxy_results_FITS, keywords_to_update)

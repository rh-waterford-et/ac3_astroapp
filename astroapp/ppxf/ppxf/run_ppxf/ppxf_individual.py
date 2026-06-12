#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Analyze individual spectrum with pPXF (robust goodpixels handling)
"""

import os
from astropy.io import fits
import numpy as np
import matplotlib.pyplot as plt
from os import path

from ppxf.ppxf import ppxf, robust_sigma
import ppxf.ppxf_util as util
import ppxf.sps_util as lib

from astropy import units as u
from specutils import Spectrum1D
from specutils.manipulation import (
    FluxConservingResampler,
    LinearInterpolatedResampler,
    SplineInterpolatedResampler,
)

import argparse

# --------------------------
# CLI
# --------------------------
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
# Helpers
# ==========================
def _sanitize_1d(arr, fill_value=0.0):
    arr = np.asarray(arr, dtype=float).copy()
    finite = np.isfinite(arr)
    if finite.all():
        return arr
    arr[~finite] = fill_value
    idx = np.arange(arr.size)
    finite = np.isfinite(arr)
    if finite.sum() >= 2:
        arr = np.interp(idx, idx[finite], arr[finite])
    arr[~np.isfinite(arr)] = fill_value
    return arr

# ==========================
# Function Definitions
# ==========================
class read_individual_spectrum(object):
    def __init__(self, spectra_filename, wave_range):
        filename = spectra_filename
        print("=" * 80)
        print("Analyzing file:", filename)
        print("=" * 80)
        hdu = fits.open(filename)
        head = hdu[0].header
        spectrum = hdu[0].data

        vph_parameters = {
            'LR-U': (0.672, 4051), 'LR-B': (0.792, 4800), 'LR-V': (0.937, 5695),
            'LR-R': (1.106, 6747), 'LR-I': (1.308, 7991), 'LR-Z': (1.455, 8900),
            'MR-U': (0.326, 4104), 'MR-UB': (0.358, 4431), 'MR-B': (0.395, 4814),
            'MR-G': (0.433, 5213), 'MR-V': (0.476, 5667), 'MR-VR': (0.522, 6170),
            'MR-R': (0.558, 6563), 'MR-RI': (0.608, 7115), 'MR-I': (0.666, 7767),
            'MR-Z': (0.796, 9262), 'HR-R': (0.355, 6646), 'HR-I': (0.462, 8634)
        }

        try:
            red = head['VPH']
            print("=" * 80)
            print("Observation VPH:", red)
            print("=" * 80)
            FWHM_VPH, lambda_c = vph_parameters.get(red, (0.937, 5695))
        except KeyError:
            instrument = head.get('INSTRUME', 'UNKNOWN')
            print("=" * 80)
            print("Instrument observed with:", instrument)
            print("=" * 80)
            if instrument == 'MaNGA':
                FWHM_VPH, lambda_c = 2.0, 7000
            elif instrument == 'MUSE':
                FWHM_VPH, lambda_c = 2.4, 7050
            else:
                FWHM_VPH, lambda_c = 0.937, 5695

        npix = spectrum.shape[0]
        wave = head['CRVAL1'] + head['CDELT1']*np.arange(npix)

        w = (wave > wave_range[0]) & (wave < wave_range[1])
        wave = wave[w]
        spectrum = spectrum[w]
        spectrum = np.maximum(spectrum, 0)

        c = 299792.458  # km/s
        velscale = c*np.diff(np.log(wave[-2:]))
        lam_range_temp = [np.min(wave), np.max(wave)]
        spectrum, ln_lam_gal, velscale = util.log_rebin(lam_range_temp, spectrum, velscale=velscale)

        spectrum = _sanitize_1d(spectrum)

        self.original_data = spectrum.copy()
        self.header = head
        self.spectrum = spectrum
        self.ln_lam_gal = ln_lam_gal
        self.lam_gal = wave
        self.fwhm_gal = FWHM_VPH
        self.lambda_c = lambda_c
        hdu.close()

def clip_outliers(galaxy, bestfit, goodpixels):
    while True:
        if goodpixels.size == 0:
            return goodpixels
        scale = galaxy[goodpixels] @ bestfit[goodpixels] / np.sum(bestfit[goodpixels]**2)
        resid = scale*bestfit[goodpixels] - galaxy[goodpixels]
        err = robust_sigma(resid, zero=1)
        ok_old = goodpixels
        goodpixels = np.flatnonzero(np.abs(bestfit - galaxy) < 3*err)
        if np.array_equal(goodpixels, ok_old):
            break
    return goodpixels

def fit_and_clean(templates, galaxy, velscale, start, goodpixels0, lam, lam_temp, plot_title,
                  min_pixels_second_pass=20):
    """
    Run pPXF, clip at 3σ, and re-run only if the cleaned mask is not empty.
    If the second pass cannot proceed, return the first solution.
    """
    galaxy = _sanitize_1d(galaxy)
    goodpixels = np.asarray(goodpixels0, int)
    goodpixels = goodpixels[(goodpixels >= 0) & (goodpixels < galaxy.size)]

    print('##############################################################')
    # First pass
    pp1 = ppxf(templates, galaxy, np.ones_like(galaxy), velscale, start,
               moments=4, degree=4, mdegree=4, lam=lam, lam_temp=lam_temp,
               goodpixels=goodpixels, plot_title=plot_title, reddening=2)

    # Clip and intersect with original
    gp_clean = clip_outliers(galaxy, pp1.bestfit, goodpixels)
    gp_clean = np.intersect1d(gp_clean, np.asarray(goodpixels0, int))
    gp_clean = gp_clean[(gp_clean >= 0) & (gp_clean < galaxy.size)]

    # If the cleaned mask is too small/empty, keep first pass
    if gp_clean.size < min_pixels_second_pass:
        optimal_template = templates @ pp1.weights
        return pp1, optimal_template

    # Second pass (guarded)
    try:
        pp2 = ppxf(templates, galaxy, np.ones_like(galaxy), velscale, start,
                   moments=4, degree=4, mdegree=4, lam=lam, lam_temp=lam_temp,
                   goodpixels=gp_clean, plot_title=plot_title, reddening=2)
        optimal_template = templates @ pp2.weights
        return pp2, optimal_template
    except Exception as e:
        print("Warning: second pPXF pass failed, returning first pass. Reason:", repr(e))
        optimal_template = templates @ pp1.weights
        return pp1, optimal_template

def create_mask(lam, exclude_ranges, apply_redshift, z_galaxy):
    mask = np.ones_like(lam, dtype=bool)
    for i, (start, end) in enumerate(exclude_ranges):
        observed_start = start * (1 + z_galaxy) if apply_redshift[i] else start
        observed_end   = end   * (1 + z_galaxy) if apply_redshift[i] else end
        mask &= (lam < observed_start) | (lam > observed_end)
    return mask

def read_mask_info(mask_file):
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
    with fits.open(original_file) as hdul:
        header = hdul[0].header
    if keywords:
        for key, value in keywords.items():
            header[key] = value
    new_hdu = fits.PrimaryHDU(data=result_data, header=header)
    new_hdu.writeto(new_filename, overwrite=True)

def get_age_metal_weights(self, weights):
    age_values, metal_values, weight_values = [], [], []
    for i in range(self.age_grid.shape[0]):
        for j in range(self.age_grid.shape[1]):
            age_values.append(self.age_grid[i, j])
            metal_values.append(self.metal_grid[i, j])
            weight_values.append(weights[i, j])
    return np.array(age_values), np.array(metal_values), np.array(weight_values)

def correct_sigma(fwhm_gal, fwhm_template, sigma_ppxf, central_wavelength):
    c = 299792.458  # km/s
    sig = 2*np.sqrt(2*np.log(2))
    sigma_gal      = (fwhm_gal      / sig) * (c / central_wavelength)
    sigma_template = (fwhm_template / sig) * (c / central_wavelength)
    corrected_sigma = np.sqrt(sigma_ppxf**2 - (sigma_gal**2 - sigma_template**2))
    return corrected_sigma

def rebin_spectrum(original_spectrum, original_wavelength, final_wavelength, method):
    input_spec = Spectrum1D(spectral_axis=original_wavelength * u.AA,
                            flux=original_spectrum * (u.erg / (u.cm**2 * u.s * u.AA)))
    if method == 'flux_conserving':
        resampler = FluxConservingResampler()
    elif method == 'linear':
        resampler = LinearInterpolatedResampler()
    elif method == 'spline':
        resampler = SplineInterpolatedResampler()
    else:
        raise ValueError("Invalid method. Choose from 'flux_conserving', 'linear', or 'spline'.")
    rebinned_spectrum = resampler(input_spec, final_wavelength * u.AA)
    return rebinned_spectrum

# ==========================
# Main Script
# ==========================
if args.filenames_file:
    with open(args.filenames_file, 'r') as file:
        spectra_filenames = [line.strip() for line in file.readlines()]
else:
    spectra_filenames = args.filenames

first_spectra_filename = spectra_filenames[0]

if args.wave_range is None:
    with fits.open(first_spectra_filename) as hdul:
        header = hdul[0].header
        if 'WAVLIMF1' in header and 'WAVLIMF2' in header:
            wave_range = [header['WAVLIMF1'], header['WAVLIMF2']]
        else:
            crval3 = header['CRVAL1']
            cdelt3 = header['CDELT1']
            npix   = header['NAXIS1']
            wave_range = [crval3, crval3 + (npix - 1) * cdelt3]
else:
    wave_range = args.wave_range

print("=" * 80)
print("Using wavelength range:", wave_range)
print("=" * 80)

s = read_individual_spectrum(first_spectra_filename, wave_range)

c_kms    = 299792.458
velscale = c_kms*np.diff(s.ln_lam_gal[:2])[0]

sps_name = args.sps_name
ppxf_dir = path.dirname(path.realpath(lib.__file__))
basename = f"spectra_{sps_name}_9.0.npz"
filename = path.join(ppxf_dir, 'sps_models', basename)

FWHM_gal = s.fwhm_gal
sps = lib.sps_lib(filename, velscale, FWHM_gal, norm_range=[5300, 5950])
stars_templates, ln_lam_temp = sps.templates, sps.ln_lam_temp

reg_dim = stars_templates.shape[1:]
stars_templates = stars_templates.reshape(stars_templates.shape[0], -1)
stars_templates /= np.median(stars_templates)
regul_err = 0.01

z = args.redshift
print("=" * 80)
print("Galaxy redshift:", z)
print("=" * 80)
vel0  = c_kms*np.log(1 + z)
sigma = args.velocity_dispersion
print("Velocity dispersion initial guess:", sigma, "km/s")
print("=" * 80)
start = [vel0, sigma]

lam_range_temp = np.exp(ln_lam_temp[[0, -1]])
goodpixels0 = util.determine_goodpixels(s.ln_lam_gal, lam_range_temp, z, width=1000)

lam_gal = np.exp(s.ln_lam_gal)
crval3  = lam_gal[0]
cdelt3  = (lam_gal[-1] - lam_gal[0]) / lam_gal.size
naxis3  = lam_gal.size
final_wavelength = crval3 + cdelt3 * np.arange(naxis3)

if args.mask_file:
    complete_array = np.arange(lam_gal.shape[0])
    exclude_ranges, apply_redshift = read_mask_info(args.mask_file)
    mask = create_mask(lam_gal, exclude_ranges, apply_redshift, z)
    goodpixels_masked = complete_array[mask]
else:
    goodpixels_masked = goodpixels0

# ==========================
# pPXF loop
# ==========================
for spectra_filename in spectra_filenames:

    s = read_individual_spectrum(spectra_filename, wave_range)

    base_filename = os.path.basename(spectra_filename).replace('.fits', '')
    suffix = f"_{args.suffix}" if args.suffix else ''
    voronoi_output_file_name = os.path.join(args.output_dir, f"{base_filename}_kinematics_and_stellar_pops_info{suffix}.txt")

    galaxy = _sanitize_1d(s.spectrum)
    if not np.isfinite(galaxy).all():
        print("Spectrum still contains non-finite values after cleaning. Skipping:", spectra_filename)
        continue

    gp = np.asarray(goodpixels_masked, int)
    gp = gp[(gp >= 0) & (gp < galaxy.size)]
    if gp.size:
        gp = gp[np.isfinite(galaxy[gp])]

    # Require a reasonable number of pixels for a stable fit
    if (galaxy != 0).any() and gp.size > 50:
        plot_title = 'pPXF fitting'
        pp, bestfit_template = fit_and_clean(stars_templates, galaxy, velscale, start, gp, lam_gal, sps.lam_temp, plot_title)
        velbin, sigbin, h3bin, h4bin = pp.sol
        velbin_error, sigbin_error, h3bin_error, h4bin_error = pp.error*np.sqrt(pp.chi2)
        optimal_templates = bestfit_template
        attbin = pp.reddening

        # Plot (non-empty)
        if hasattr(pp, "plot_mine"):
            plt.figure(figsize=(9, 4))
            pp.plot_mine()
            fig = plt.gcf()
        else:
            fig, ax = plt.subplots(figsize=(9, 4))
            pp.plot(ax=ax)
            ax.tick_params(which='both', direction='in', length=5, width=1)
            ax.xaxis.set_tick_params(labelsize=10)
            ax.yaxis.set_tick_params(labelsize=10)
        plt.title(plot_title, fontsize=12)
        fig.savefig(
            os.path.join(args.output_dir, f"{base_filename}_pPXF_fitting{suffix}.pdf"),
            dpi=600, bbox_inches='tight'
        )
        plt.close(fig)

        residuals = galaxy - pp.bestfit
        residulas_rebin = rebin_spectrum(residuals, lam_gal, final_wavelength, 'flux_conserving')
        bestfits_rebin  = rebin_spectrum(pp.bestfit, lam_gal, final_wavelength, 'flux_conserving')
        galaxy_rebin    = rebin_spectrum(galaxy, lam_gal, final_wavelength, 'flux_conserving')

        with open(voronoi_output_file_name, 'w') as f:
            f.write("Kinematics results:\n\n")
            f.write("{:<20} {:<20} {:<30} {:<30} {:<15} {:<15} {:<15} {:<15}\n".format(
                "Velocity(km/s)", "Error(Velocity)", "Velocity_Dispersion(km/s)", "Error(Velocity_Dispersion)", "h3", "Error(h3)", "h4", "Error(h4)"))
            f.write("{:<20.8f} {:<20.8f} {:<30.8f} {:<30.8f} {:<15.8f} {:<15.8f} {:<15.8f} {:<15.8f}\n".format(
                velbin, velbin_error, sigbin, sigbin_error, h3bin, h3bin_error, h4bin, h4bin_error))
            f.write('*' * 162 + "\n\n")

        light_weights = pp.weights.reshape(reg_dim)
        lg_age_bin, metalbin = sps.mean_age_metal(light_weights)

        try:
            M_L_ratio_V = sps.mass_to_light(light_weights, band='V', redshift=args.redshift)
            ml_str = f"{M_L_ratio_V:.8f}"
        except Exception as e:
            print("Warning: could not compute M/L(V). Reason:", repr(e))
            M_L_ratio_V = np.nan
            ml_str = "NaN"

        with open(voronoi_output_file_name, 'a') as f:
            f.write("Stellar populations results:\n\n")
            f.write("{:<20} {:<20} {:<20} {:<15}\n".format("Weighted_Age(Gyr)", "Weighted_Metallicity", "M*/L(V_band)", "A_V"))
            f.write("{:<20.8f} {:<20.8f} {:<20} {:<15.8f}\n".format((10**lg_age_bin)/1e9, metalbin, ml_str, attbin))
            f.write('*' * 73 + "\n\n")

        age_values, metal_values, weight_values = get_age_metal_weights(sps, light_weights)
        nz = weight_values > 0
        age_values_nonzero    = age_values[nz]
        metal_values_nonzero  = metal_values[nz]
        weight_values_nonzero = weight_values[nz] / np.sum(weight_values)

        with open(voronoi_output_file_name, 'a') as f:
            f.write("Templates used for the fitting:\n\n")
            f.write("{:<20} {:<20} {:<20}\n".format("Age(Gyr)", "Metallicity", "Weight"))
            for age, metal, weight in zip(age_values_nonzero, metal_values_nonzero, weight_values_nonzero):
                f.write("{:<20.8f} {:<20.8f} {:<20.8f}\n".format(age, metal, weight))
            f.write('*' * 52 + "\n")

        print("-"*50)
        print("Age values [Gyr]", age_values_nonzero)
        print("Metallicity values [M/H]", metal_values_nonzero)
        print("Weights [Normalized]", weight_values_nonzero)
        print("-"*50)

    else:
        print("Empty spectrum or too few goodpixels. Skipping:", spectra_filename)
        continue

    if FWHM_gal < sps.fwhm_tem[0]:
        corrected_sigbin = correct_sigma(FWHM_gal, sps.fwhm_tem[0], sigbin, s.lambda_c)
        corrected_sigbin_error = sigbin*sigbin_error/ corrected_sigbin
    else:
        corrected_sigbin = sigbin
        corrected_sigbin_error = sigbin_error

    residuals_results_FITS = os.path.join(args.output_dir, f"{base_filename}_residuals{suffix}.fits")
    bestfits_results_FITS  = os.path.join(args.output_dir, f"{base_filename}_bestfit{suffix}.fits")
    galaxy_results_FITS    = os.path.join(args.output_dir, f"{base_filename}_galaxy{suffix}.fits")

    keywords_to_update = {'CRVAL1': crval3, 'CDELT1': cdelt3}
    save_results_as_fits(residulas_rebin.data, spectra_filename, residuals_results_FITS, keywords_to_update)
    save_results_as_fits(bestfits_rebin.data,  spectra_filename, bestfits_results_FITS,  keywords_to_update)
    save_results_as_fits(galaxy_rebin.data,    spectra_filename, galaxy_results_FITS,    keywords_to_update)

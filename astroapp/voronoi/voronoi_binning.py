import os
from astropy.io import fits
import numpy as np
import matplotlib
matplotlib.use('Agg')  # Use non-interactive backend for headless environments
import matplotlib.pyplot as plt
# from os import path
from matplotlib.backends.backend_pdf import PdfPages
from scipy.interpolate import LSQUnivariateSpline
from matplotlib.ticker import AutoMinorLocator
import sys

# from ppxf.ppxf import ppxf, robust_sigma
# import ppxf.ppxf_util as util
# import ppxf.sps_util as lib
from vorbin.voronoi_2d_binning import voronoi_2d_binning
# from plotbin.display_bins import display_bins

import argparse

parser = argparse.ArgumentParser(description='Script for performing Voronoi binning on astronomical data cubes')
parser.add_argument('filename', type=str, help='Path to the spectra file to be analyzed')
parser.add_argument('-pix1', '--pix1', metavar='Initial pixel', help='Initial pixel', type=int)
parser.add_argument('-pix2', '--pix2', metavar='Final pixel', help='Final pixel', type=int)
parser.add_argument('-wl1', '--wavelength-start', metavar='Initial wavelength', help='Initial wavelength', type=float)
parser.add_argument('-wl2', '--wavelength-end', metavar='Final wavelength', help='Final wavelength', type=float)
parser.add_argument('-z', '--redshift', metavar='REDSHIFT', default=0.0, help='Redshift of the source', type=float)
parser.add_argument('-az', '--apply-redshift', default=False, action="store_true", help='Set it True if you want to shift your wl1 and wl2 values')
parser.add_argument('--mask-file', help='Path to the file containing the wavelength intervals you do not want to take into account')
parser.add_argument('--output-dir', type=str, default='./', metavar='OUTPUT_DIR',
                    help='Directory where the output FITS files will be saved (default: current directory)')
parser.add_argument('--suffix', type=str, default='', help='Optional label to append to the output file names')
parser.add_argument('-p', '--plot', default=False, action="store_true", help='Fitting plots (default inactive)')
parser.add_argument("--knots-number", type=int, help="Number of knots to use in the spline fitting for estimating residuals. Default is 40.", default=40)
parser.add_argument('--sn-method', type=str, default='spline', choices=['spline', 'brightest_spaxel', 'signal_square_root'],
                    help='Method to estimate the signal-to-noise ratio. Default method is "spline"')
parser.add_argument('-minsn', '--min-sn', metavar='Minimum Signal/Noise', default=0, help='Minimum Signal/Noise desired in output data (default = 0)', type=float)
parser.add_argument('-sn', '--sn', metavar='Signal/Noise', default=40, help='Signal/Noise desired in output data (default = 40)', type=float)
parser.add_argument('--generate-individual-spectra', action='store_true', help='Generate individual FITS files for each Voronoi cell')
parser.add_argument('--instrument', choices=['megara', 'manga', 'muse'], required=True, help='Instrument used to obtain the data cube')
parser.add_argument('--split-cube-into-spectra', action='store_true', help='If set, splits the original data cube into individual FITS spectra for each spaxel, saves cell_numbers and voronoi_output maps, and exits without S/N computation or Voronoi binning.')

args = parser.parse_args()

# ==========================
# Function Definitions
# ==========================

def read_megara_cube(spectra_filename):
    """
    Read MEGARA cube, log rebin it and compute coordinates of each spaxel.
    """
    print("=" * 80)
    print(f"Analyzing file: {spectra_filename}")
    print("=" * 80)
    hdu = fits.open(spectra_filename)
    head = hdu[0].header
    cube = hdu[0].data   # cube.shape = (4300, nx, ny)
    wave = head['CRVAL3'] + head['CDELT3']*np.arange(head['NAXIS3'])
    spectra_cgs = cube*1e20 / (3.33564095e4*(wave[:, np.newaxis, np.newaxis]**2))   ## Units 10**-20 erg cm-2 s-1 Angstrom-1

    return {
        'data': cube,
        'header': head,
        'data_cgs': spectra_cgs,
        'lam_gal': wave,
        'pixlimf1': head['PIXLIMF1'],
        'pixlimf2': head['PIXLIMF2'],
        # 'pixelsize': head['CDELT2']*3600
        'pixelsize': None
    }

def read_manga_cube(spectra_filename):
    """
    Read MANGA cube, log rebin it and compute coordinates of each spaxel.
    """

    print("=" * 80)
    print(f"Analyzing file: {spectra_filename}")
    print("=" * 80)
    hdu = fits.open(spectra_filename)
    head = hdu[1].header
    cube = hdu[1].data   # cube.shape = (4300, nx, ny)
    wave = head['CRVAL3'] + head['CD3_3']*np.arange(head['NAXIS3'])
    spectra_cgs = cube ## Units 10**-17 erg cm-2 s-1 Angstrom-1

    return {
        'data': cube,
        'data_cgs': spectra_cgs,
        'header': head,
        'lam_gal': wave,
        # 'pixelsize': head['CD2_2']*3600
        'pixelsize': None
    }

def read_muse_cube(spectra_filename):
    """
    Read MUSE cube, log rebin it and compute coordinates of each spaxel.
    """

    print("=" * 80)
    print(f"Analyzing file: {spectra_filename}")
    print("=" * 80)
    hdu = fits.open(spectra_filename)
    # Read header from HDU[1] where wavelength keywords are located
    head = hdu[1].header
    cube = hdu[1].data   # cube.shape = (4300, nx, ny)
    wave = head['CRVAL3'] + head['CD3_3']*np.arange(head['NAXIS3'])
    spectra_cgs = cube ## Units 10**(-16)*erg/s/cm**2/Angstrom

    return {
        'data': cube,
        'data_cgs': spectra_cgs,
        'header': head,
        'lam_gal': wave,
        'pixelsize': head['CD2_2']*3600
    }

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


def calculate_valid_pixels(result, wavelength_start=None, wavelength_end=None, apply_redshift=False, redshift=0.0, mask_file=None):
    """
    Calculate the valid pixels for S/N calculation based on the provided wavelength intervals and redshift parameters.

    Parameters:
    result (dict): Dictionary containing the data, including the array of galaxy wavelengths ('lam_gal').
    wavelength_start (float, optional): The starting wavelength. Defaults to None.
    wavelength_end (float, optional): The ending wavelength. Defaults to None.
    apply_redshift (bool, optional): Whether to apply redshift to the wavelengths. Defaults to False.
    redshift (float, optional): The redshift value to apply. Defaults to 0.0.
    mask_file (str, optional): Path to the file containing good wavelength intervals. Defaults to None.

    Returns:
    valid_pixels (array): The array of valid pixels for S/N calculation.
    """
    
    lam_gal = result['lam_gal']
    
    # Set the initial pixel (pix_i) and final pixel (pix_f) based on user input or header information.

    pix1 = None
    pix2 = None

    if pix1 is not None:
        initial_pixel = pix1
    else:
        try:
            initial_pixel = result['pixlimf1']
        except KeyError:
            initial_pixel = 0
    if pix2 is not None:
        final_pixel = pix2
    else:
        try:
            final_pixel = result['pixlimf2']
        except KeyError:
            final_pixel = -1
    
    # Initialize the list to store the good wavelength intervals
    good_wavelength_intervals = []

    # Add the starting wavelength to the intervals
    if wavelength_start is not None:
        if apply_redshift == False:
            good_wavelength_intervals.append(wavelength_start)
        else:
            good_wavelength_intervals.append(wavelength_start * (1 + redshift))
    else:
        good_wavelength_intervals.append(lam_gal[initial_pixel])  # Assuming initial_pixel is the first element

    # Check if a mask file is provided as input
    if mask_file:
        # Read the mask information
        exclude_ranges, apply_redshift_flags = read_mask_info(mask_file)
        # Iterate over the ranges and redshift flags
        for (start, end), apply_redshift_mask in zip(exclude_ranges, apply_redshift_flags):
            if apply_redshift_mask:
                good_wavelength_intervals.append(start * (1 + redshift))
                good_wavelength_intervals.append(end * (1 + redshift))
            else:
                good_wavelength_intervals.append(start)
                good_wavelength_intervals.append(end)
    else:
        # If no mask file is provided, handle it differently or raise an error
        print("No mask file provided.")

    # Remove intervals greater than wavelength_end if specified
    if wavelength_end is not None:
        good_wavelength_intervals = [w for w in good_wavelength_intervals if w <= wavelength_end]
        if apply_redshift == False:
            good_wavelength_intervals.append(wavelength_end)
        else:
            good_wavelength_intervals.append(wavelength_end * (1 + redshift))
    else:
        good_wavelength_intervals.append(lam_gal[final_pixel])  # Assuming final_pixel is the last element

    # Ensure the number of intervals is even by adding an extra interval if necessary
    if len(good_wavelength_intervals) % 2 != 0:
        good_wavelength_intervals.append(good_wavelength_intervals[-1]+1)

    # Convert the wavelength intervals to pixel intervals (good_pixel_intervals)
    good_pixel_intervals = []
    last_index = 0
    for wavelength in good_wavelength_intervals:
        for j in range(last_index, len(lam_gal)):
            if wavelength == lam_gal[j] or wavelength < lam_gal[j]:
                good_pixel_intervals.append(j)
                last_index = j
                break

    # Define the pixels to include in the S/N calculation (valid_pixels)
    valid_pixels = ()
    for i in np.arange(0, len(good_pixel_intervals), step=2):
        valid_pixels = np.append(valid_pixels, np.arange(good_pixel_intervals[i], good_pixel_intervals[i + 1] + 1))
    valid_pixels = np.int64(valid_pixels)
    
    valid_pixels = sorted(set(valid_pixels))
    return valid_pixels

def signal_to_noise_estimation_spline(result, valid_pixels, knots_number, lam_gal, plot=False):
    """
    Estimate the signal-to-noise ratio for each pixel in the data and generate plots if required.

    Parameters:
    result (dict): Dictionary containing the data and header information.
    sn_fitting_plots (str): Path to save the PDF file with the plots.
    valid_pixels (array): Array of valid pixel indices.
    knots (array): Array of knot positions for spline fitting.
    lam_gal (array): Array of observed wavelengths.
    plot (bool): Whether to generate and save plots. Default is False.

    Returns:
    sn_coordinates (ndarray): Array containing the signal-to-noise ratio and coordinates.
    """
    
    if plot:
        sn_fitting_plots = os.path.join(args.output_dir, f"{base_filename}_sn_fitting_plots{suffix}.pdf")
        pdf_file = PdfPages(sn_fitting_plots)
        
    # factor_aa = 1/np.sqrt(result['header']['CDELT3'])  # Convert from s/n per pixel to s/n per angstrom
    nrows = result['data'].shape[1]
    ncols = result['data'].shape[2]
    sn_coordinates = np.zeros([nrows*ncols, 4])
    knots = np.linspace(valid_pixels[0], valid_pixels[-1], knots_number)
    print("=" * 80)
    print(f'Number of knots used for splines: {knots_number}')
    print("=" * 80)
    
    # Plot central spaxel fitting for checking the quality of the fitting
    # Polynomial fitting to calculate residuals
    spline = LSQUnivariateSpline(valid_pixels, result['data'][valid_pixels, nrows//2, ncols//2], t=knots[1:-1])  # Exclude the extremes
    clean_spectra = spline(valid_pixels)
    residuals = result['data'][valid_pixels, nrows//2, ncols//2] - clean_spectra
    std_deviation = np.std(residuals, ddof=0)
    
    plt.rc('xtick', direction='in')
    fig = plt.figure(figsize=(8, 3.7))
    plt.clf()
    frame1 = fig.add_axes((.1, .3, .8, .63))
    plt.title(f'Spaxel coordinates x={ncols//2}, y={nrows//2}')
    plt.plot(lam_gal[valid_pixels], result['data'][valid_pixels, nrows//2, ncols//2], 'b-', linewidth=0.25, label='Input spectrum')
    plt.plot(lam_gal[valid_pixels], clean_spectra, '--', color='orange', linewidth=1, label='Polyfit fitting')
    ax = plt.gca()
    ax.xaxis.set_tick_params(length=5, width=1, labelsize=0)
    ax.xaxis.set_minor_locator(AutoMinorLocator())
    ax.yaxis.set_tick_params(length=5, width=1, labelsize=12)
    ax.yaxis.set_minor_locator(AutoMinorLocator())
    plt.setp(ax.spines.values(), linewidth=1, zorder=100)
    plt.legend(frameon=False)
    frame2 = fig.add_axes((.1, .15, .8, .14))
    plt.plot(lam_gal[valid_pixels], residuals, 'g', label='Residuals', linewidth=0.25)
    plt.hlines(0, min(lam_gal[valid_pixels]), max(lam_gal[valid_pixels]), linestyles='dashed', linewidth=0.25, colors='black', zorder=100)
    plt.xlabel("Observed wavelength [$\\mathregular{\\AA}$]", fontsize=12)
    plt.rc('axes', linewidth=1.5)
    ax = plt.gca()
    ax.xaxis.set_tick_params(length=5, width=1, labelsize=12)
    ax.yaxis.set_tick_params(length=5, width=1, labelsize=12)
    ax.xaxis.set_ticks_position('both')
    plt.rc('xtick', direction='in')
    plt.setp(ax.spines.values(), linewidth=1, zorder=100)
    plt.subplots_adjust(left=0.1, bottom=0.2, right=0.9, top=0.99)
    ax.xaxis.set_minor_locator(AutoMinorLocator())
    ax.yaxis.set_minor_locator(AutoMinorLocator())
    # plt.show()
    
    
    for i in range(nrows):
        for j in range(ncols):
            index = i * ncols + j
            sys.stdout.write(f"\rWorking on spaxel {index+1}/{nrows*ncols}")
            sys.stdout.flush()
            sn_coordinates[index, 0] = j + 1
            sn_coordinates[index, 1] = i + 1
            # sn_coordinates[index, 2] = np.median(result['data'][valid_pixels, i, j]) * factor_aa  # Signal per angstrom
            sn_coordinates[index, 2] = np.median(result['data'][valid_pixels, i, j])  # Signal

            # Polynomial fitting to calculate residuals
            spline = LSQUnivariateSpline(valid_pixels, result['data'][valid_pixels, i, j], t=knots[1:-1])  # Exclude the extremes
            clean_spectra = spline(valid_pixels)
            residuals = result['data'][valid_pixels, i, j] - clean_spectra
            std_deviation = np.std(residuals, ddof=0)

            if plot:
                plt.rc('xtick', direction='in')
                fig = plt.figure(figsize=(8, 3.7))
                plt.clf()
                frame1 = fig.add_axes((.1, .3, .8, .63))
                plt.title(f'Spaxel coordinates {i, j}')
                plt.plot(lam_gal[valid_pixels], result['data'][valid_pixels, i, j], 'b-', linewidth=0.25, label='Input spectrum')
                plt.plot(lam_gal[valid_pixels], clean_spectra, '--', color='orange', linewidth=1, label='Polyfit fitting')
                ax = plt.gca()
                ax.xaxis.set_tick_params(length=5, width=1, labelsize=0)
                ax.xaxis.set_minor_locator(AutoMinorLocator())
                ax.yaxis.set_tick_params(length=5, width=1, labelsize=12)
                ax.yaxis.set_minor_locator(AutoMinorLocator())
                plt.setp(ax.spines.values(), linewidth=1, zorder=100)
                plt.legend(frameon=False)
                frame2 = fig.add_axes((.1, .15, .8, .14))
                plt.plot(lam_gal[valid_pixels], residuals, 'g', label='Residuals', linewidth=0.25)
                plt.hlines(0, min(lam_gal[valid_pixels]), max(lam_gal[valid_pixels]), linestyles='dashed', linewidth=0.25, colors='black', zorder=100)
                plt.xlabel("Observed wavelength [$\\mathregular{\\AA}$]", fontsize=12)
                plt.rc('axes', linewidth=1.5)
                ax = plt.gca()
                ax.xaxis.set_tick_params(length=5, width=1, labelsize=12)
                ax.yaxis.set_tick_params(length=5, width=1, labelsize=12)
                ax.xaxis.set_ticks_position('both')
                plt.rc('xtick', direction='in')
                plt.setp(ax.spines.values(), linewidth=1, zorder=100)
                plt.subplots_adjust(left=0.1, bottom=0.2, right=0.9, top=0.99)
                ax.xaxis.set_minor_locator(AutoMinorLocator())
                ax.yaxis.set_minor_locator(AutoMinorLocator())
                pdf_file.savefig(fig)  # Save the figure to the PDF file
                plt.close(fig)

            if sn_coordinates[index, 2] < 1e-10:
                sn_coordinates[index, 2] = 0
                sn_coordinates[index, 3] = 1
            else:
                sn_coordinates[index, 3] = std_deviation  # Noise
    if plot:
        pdf_file.close()
    print('\n')
    # Calculate signal-to-noise ratio and save to FITS file
    sn_ratio = sn_coordinates[:, 2] / sn_coordinates[:, 3]
    
    # Reshape the S/N ratio array to match the original data cube dimensions
    sn_ratio_cube = sn_ratio.reshape((nrows, ncols))
    signal_cube = sn_coordinates[:, 2].reshape((nrows, ncols))
    noise_cube = sn_coordinates[:, 3].reshape((nrows, ncols))
    
    # Create a new FITS HDU (Header/Data Unit) object
    hdu = fits.PrimaryHDU(sn_ratio_cube)
    hdu_signal = fits.PrimaryHDU(signal_cube)
    hdu_noise = fits.PrimaryHDU(noise_cube)
    
    # Create a FITS HDUList to contain the HDU object
    hdulist = fits.HDUList([hdu])
    hdulist_signal = fits.HDUList([hdu_signal])
    hdulist_noise = fits.HDUList([hdu_noise])
    
    # Save the HDUList to a new FITS file
    sn_fits_file = os.path.join(args.output_dir, f"{base_filename}_snr_map{suffix}.fits")
    signal_fits_file = os.path.join(args.output_dir, f"{base_filename}_signal_map{suffix}.fits")
    noise_fits_file = os.path.join(args.output_dir, f"{base_filename}_noise_map{suffix}.fits")
    
    hdulist.writeto(sn_fits_file, overwrite=True)
    # hdulist_signal.writeto(signal_fits_file, overwrite=True)
    # hdulist_noise.writeto(noise_fits_file, overwrite=True)
    return sn_coordinates

def signal_to_noise_estimation_brightest_spaxel(result, valid_pixels, knots_number, lam_gal, plot=False):
    """
    Estimate the signal-to-noise ratio for each pixel in the data scaling its value from the s/n of the brightest spaxel.
    First, the signal-to-noise ratio is calculated for the brightest spaxel, and then this value is used to calculate the
    signal-to-noise ratio for the rest of the spaxels by scaling it with the square root of the brightness ratio between
    each spaxel and the brightest spaxel.

    Parameters:
    result (dict): Dictionary containing the data and header information.
    sn_fitting_plots (str): Path to save the PDF file with the plots.
    valid_pixels (array): Array of valid pixel indices.
    knots (array): Array of knot positions for spline fitting.
    lam_gal (array): Array of observed wavelengths.
    plot (bool): Whether to generate and save plots. Default is False.

    Returns:
    sn_coordinates (ndarray): Array containing the signal-to-noise ratio and coordinates.
    """
        
    # factor_aa = 1/np.sqrt(result['header']['CDELT3'])  # Convert from s/n per pixel to s/n per angstrom
    nrows = result['data'].shape[1]
    ncols = result['data'].shape[2]
    sn_coordinates = np.zeros([nrows*ncols, 4])
    
    # Define the number of knots for the spline fitting
    knots = np.linspace(valid_pixels[0], valid_pixels[-1], knots_number)
    print("=" * 80)
    print(f'Number of knots used for splines: {knots_number}')
    print("=" * 80)
    
    # Find the brightest spaxel
    # Sum the values along the wavelength axis for each spaxel
    brightness = np.sum(result['data'], axis=0)

    # Find the index of the brightest spaxel
    max_brightness_index = np.unravel_index(np.argmax(brightness, axis=None), brightness.shape)
    
    # Polynomial fitting to calculate the signal-to-noise ratio of the brightest spaxel
    spline = LSQUnivariateSpline(valid_pixels, result['data'][valid_pixels, max_brightness_index[0], max_brightness_index[1]], t=knots[1:-1])  # Exclude the extremes
    clean_spectra = spline(valid_pixels)
    signal_brightest_spaxel = np.median(clean_spectra)
    residuals_brightest_spaxel = result['data'][valid_pixels, max_brightness_index[0], max_brightness_index[1]] - clean_spectra
    std_deviation_brightest_spaxel = np.std(residuals_brightest_spaxel, ddof=0)
    sn_brightest_spaxel = signal_brightest_spaxel/std_deviation_brightest_spaxel
    
    # Plot the fitting for checking its quality
    plt.rc('xtick', direction='in')
    fig = plt.figure(figsize=(8, 3.7))
    plt.clf()
    frame1 = fig.add_axes((.1, .3, .8, .63))
    plt.title(f'Spaxel coordinates x={max_brightness_index[1]}, y={max_brightness_index[0]}')
    plt.plot(lam_gal[valid_pixels], result['data'][valid_pixels, max_brightness_index[0], max_brightness_index[1]], 'b-', linewidth=0.25, label='Input spectrum')
    plt.plot(lam_gal[valid_pixels], clean_spectra, '--', color='orange', linewidth=1, label='Polyfit fitting')
    ax = plt.gca()
    ax.xaxis.set_tick_params(length=5, width=1, labelsize=0)
    ax.xaxis.set_minor_locator(AutoMinorLocator())
    ax.yaxis.set_tick_params(length=5, width=1, labelsize=12)
    ax.yaxis.set_minor_locator(AutoMinorLocator())
    plt.setp(ax.spines.values(), linewidth=1, zorder=100)
    plt.legend(frameon=False)
    frame2 = fig.add_axes((.1, .15, .8, .14))
    plt.plot(lam_gal[valid_pixels], residuals_brightest_spaxel, 'g', label='Residuals', linewidth=0.25)
    plt.hlines(0, min(lam_gal[valid_pixels]), max(lam_gal[valid_pixels]), linestyles='dashed', linewidth=0.25, colors='black', zorder=100)
    plt.xlabel("Observed wavelength [$\\mathregular{\\AA}$]", fontsize=12)
    plt.rc('axes', linewidth=1.5)
    ax = plt.gca()
    ax.xaxis.set_tick_params(length=5, width=1, labelsize=12)
    ax.yaxis.set_tick_params(length=5, width=1, labelsize=12)
    ax.xaxis.set_ticks_position('both')
    plt.rc('xtick', direction='in')
    plt.setp(ax.spines.values(), linewidth=1, zorder=100)
    plt.subplots_adjust(left=0.1, bottom=0.2, right=0.9, top=0.99)
    ax.xaxis.set_minor_locator(AutoMinorLocator())
    ax.yaxis.set_minor_locator(AutoMinorLocator())
    if plot:
        sn_fitting_plots = os.path.join(args.output_dir, f"{base_filename}_sn_fitting_plots{suffix}.pdf")
        plt.savefig(sn_fitting_plots, dpi=600)
    plt.show()
    
    # Calculation of the brightness ratio to scale de sn of the brightest spaxel with the rest of the spaxels
    brightness_ratio = brightness/brightness[max_brightness_index[0], max_brightness_index[1]]
    sn_scale = np.sqrt(brightness_ratio)
    
    
    # Array with the signal-to-noise ratio in cube format
    sn_map_data = sn_brightest_spaxel*sn_scale
    
    
    # Fill the sn_coordinates array with the signal-to-noise ratio as the signal and the noise as 1
    index = 0
    for i in range(nrows):
        for j in range(ncols):
            sys.stdout.write(f"\rWorking on spaxel {index+1}/{nrows*ncols}")
            sys.stdout.flush()
            
            sn_coordinates[index, 0] = j
            sn_coordinates[index, 1] = i
            if sn_map_data[i,j] < 1e-10:
                sn_coordinates[index, 2] = 0
            else:
                sn_coordinates[index, 2] = sn_map_data[i,j]
            sn_coordinates[index, 3] = 1
            index += 1

    
    # Reshape the S/N ratio array to match the original data cube dimensions
    sn_ratio_cube = sn_map_data
    
    # Create a new FITS HDU (Header/Data Unit) object
    hdu = fits.PrimaryHDU(sn_ratio_cube)
    
    # Create a FITS HDUList to contain the HDU object
    hdulist = fits.HDUList([hdu])
    
    # Save the HDUList to a new FITS file
    sn_fits_file = os.path.join(args.output_dir, f"{base_filename}_snr_map{suffix}.fits")
    
    hdulist.writeto(sn_fits_file, overwrite=True)
    return sn_coordinates

def signal_to_noise_estimation_signal_square_root(result, valid_pixels, lam_gal):
    """
    Estimate the signal-to-noise ratio for each pixel in the data using the method described by Cappellari.
    This method calculates the signal-to-noise ratio by taking the median signal value for each spaxel and
    using the square root of this value as the noise estimate.

    Parameters:
    result (dict): Dictionary containing the data and header information.
    valid_pixels (array): Array of valid pixel indices.
    lam_gal (array): Array of observed wavelengths.

    Returns:
    sn_coordinates (ndarray): Array containing the signal-to-noise ratio and coordinates.
    """
    
    nrows = result['data'].shape[1]
    ncols = result['data'].shape[2]
    sn_coordinates = np.zeros([nrows*ncols, 4])
    
    # Fill the sn_coordinates array with the signal-to-noise ratio as the signal and the noise as 1
    index = 0
    for i in range(nrows):
        for j in range(ncols):
            sys.stdout.write(f"\rWorking on spaxel {index+1}/{nrows*ncols}")
            sys.stdout.flush()
            
            sn_coordinates[index, 0] = j
            sn_coordinates[index, 1] = i
            sn_coordinates[index, 2] = np.median(result['data'][valid_pixels,i,j]) # Signal
            if sn_coordinates[index, 2] < 1e-10:
                sn_coordinates[index, 2] = 0
                sn_coordinates[index, 3] = 1
            else:
                sn_coordinates[index, 3] = np.sqrt(np.median(result['data'][valid_pixels,i,j])) # Noise
            index += 1
    
    # Calculate signal-to-noise ratio and save to FITS file
    sn_ratio = sn_coordinates[:, 2] / sn_coordinates[:, 3]
    
    # Reshape the S/N ratio array to match the original data cube dimensions
    sn_ratio_cube = sn_ratio.reshape((nrows, ncols))

    # Create a new FITS HDU (Header/Data Unit) object
    hdu = fits.PrimaryHDU(sn_ratio_cube)
    
    # Create a FITS HDUList to contain the HDU object
    hdulist = fits.HDUList([hdu])
    
    # Save the HDUList to a new FITS file
    sn_fits_file = os.path.join(args.output_dir, f"{base_filename}_snr_map{suffix}.fits")
    
    hdulist.writeto(sn_fits_file, overwrite=True)
    
    return sn_coordinates

def filter_by_signal_to_noise(sn_coordinates, min_sn):
    """
    Filter the sn_coordinates array to remove values with a signal-to-noise ratio below the specified minimum.

    Parameters:
    min_sn (float): The minimum signal-to-noise ratio threshold.
    sn_coordinates (ndarray): The input array containing the signal-to-noise ratio and other data.

    Returns:
    ndarray: The filtered sn_coordinates array.
    """
    # Calculate the signal-to-noise ratio
    sn_ratio = sn_coordinates[:, 2]/sn_coordinates[:, 3]
    
    # Create a boolean mask where the condition is met
    mask = sn_ratio >= min_sn
    
    # Use the mask to filter the array
    filtered_sn_coordinates = sn_coordinates[mask]
    
    return filtered_sn_coordinates


def generate_voronoi_cubes(data_cube, voronoi_bins, header, generate_individual_spectra=False):
    # Get the dimensions of the data cube
    nz, ny, nx = data_cube.shape
    
    # Create dictionaries to store the summed spectra and counts for each Voronoi cell
    voronoi_cells = {}
    voronoi_counts = {}
    
    # Create a 2D image where each pixel value is the Voronoi cell number, initialized with NaN
    voronoi_image = np.full((ny, nx), np.nan)
    
    # Total number of spaxels
    total_spaxels = len(voronoi_bins)
    
    # Iterate over each spaxel in the Voronoi data
    for index, (x, y, cell) in enumerate(voronoi_bins):
        if cell not in voronoi_cells:
            voronoi_cells[cell] = np.zeros(nz)
            voronoi_counts[cell] = 0
        
        voronoi_cells[cell] += data_cube[:, y-1, x-1]
        voronoi_counts[cell] += 1
        voronoi_image[y-1, x-1] = cell
    
    # Create an empty data cube for the Voronoi cells
    voronoi_cube = np.zeros((nz, ny, nx))
    
    # Assign the averaged spectra to the Voronoi cube
    for x, y, cell in voronoi_bins:
        
        # Update the progress bar
        # progress_bar(index, total_spaxels)
        
        if cell == -1:
            voronoi_cube[:, y-1, x-1] = np.nan
        else:
            voronoi_cube[:, y-1, x-1] = voronoi_cells[cell] / voronoi_counts[cell]
    
    # Save the Voronoi cube to a new FITS file with the same header as the original cube
    hdu_voronoi = fits.PrimaryHDU(data=voronoi_cube, header=header)
    voronoi_results_FITS = os.path.join(args.output_dir, f"{base_filename}_voronoi_binned_sn_{target_sn}{suffix}.fits")
    hdu_voronoi.writeto(voronoi_results_FITS, overwrite=True)

    # Save the Voronoi cell number image to a new FITS file
    hdu_voronoi_image = fits.PrimaryHDU(data=voronoi_image, header=header)
    voronoi_image_FITS = os.path.join(args.output_dir, f"{base_filename}_voronoi_cell_numbers_sn_{target_sn}{suffix}.fits")
    hdu_voronoi_image.writeto(voronoi_image_FITS, overwrite=True)
    
    if generate_individual_spectra:
        # Create a subdirectory for individual Voronoi cell spectra
        individual_spectra_dir = os.path.join(args.output_dir, f"{base_filename}_individual_voronoi_spectra_sn_{target_sn}{suffix}")
        os.makedirs(individual_spectra_dir, exist_ok=True)
            
        # Save individual FITS files for each Voronoi cell
        for cell, spectrum in voronoi_cells.items():
            averaged_spectrum = spectrum / voronoi_counts[cell]
            hdu_cell = fits.PrimaryHDU(data=averaged_spectrum)
            
            # Update header with specific information
            try:
                hdu_cell.header['INSTRUME'] = header['INSTRUME']
            except KeyError:
                try:
                    hdu_cell.header['INSTRUME'] = 'MUSE'
                except KeyError:
                    pass

            try:
                hdu_cell.header['OBJECT'] = header['OBJECT']
            except KeyError:
                pass
            try:
                hdu_cell.header['NAXIS1'] = header['NAXIS3']
            except KeyError:
                pass
            try:
                hdu_cell.header['CRVAL1'] = header['CRVAL3']
            except KeyError:
                pass
            try:
                hdu_cell.header['CDELT1'] = header['CDELT3']
            except KeyError:
                try:
                    hdu_cell.header['CDELT1'] = header['CD3_3']
                except KeyError:
                    pass
            try:
                hdu_cell.header['VPH'] = header['VPH']
            except KeyError:
                pass
            
            hdu_cell.header['CRPIX1'] = 1
            hdu_cell.header['CTYPE1'] = 'Wavelength'
            hdu_cell.header['CUNIT1'] = 'Angstrom'
            if args.instrument == 'muse':
                hdu_cell.header['BUNIT'] = '10**-16 erg cm-2 s-1 Angstrom-1'
            else:
                hdu_cell.header['BUNIT'] = '10**-17 erg cm-2 s-1 Angstrom-1'
            hdu_cell.header['CELL'] = cell
            try:
                hdu_cell.header['WAVLIMF1'] = header['WAVLIMF1']
            except KeyError:
                pass
            try:
                hdu_cell.header['WAVLIMF2'] = header['WAVLIMF2']
            except KeyError:
                pass
            try:
                hdu_cell.header['PLATEIFU'] = header['PLATEIFU']
            except KeyError:
                pass

            cell_filename = os.path.join(individual_spectra_dir, f"{base_filename}_voronoi_cell_{cell}{suffix}.fits")
            hdu_cell.writeto(cell_filename, overwrite=True)
    
    return voronoi_cube

    
# def progress_bar(current, total):
#     """
#     Displays a progress bar in the console.

#     Parameters:
#     current (int): The current progress step.
#     total (int): The total number of steps.

#     The progress bar updates in place, showing the percentage of completion.
#     """
#     # Calculate the percentage of completion
#     percent = (current + 1) / total * 100
    
#     # Create the progress bar string
#     bar = '#' * int(percent // 2) + '-' * (50 - int(percent // 2))
    
#     # Write the progress bar to the console
#     sys.stdout.write(f"\r[{bar}] {percent:.2f}%")
#     sys.stdout.flush()

def export_original_as_cells(result, base_filename, suffix):
    """
    Treat each original spaxel (x, y) of the data cube as an independent Voronoi cell.
    - Assign zero-based cell IDs (0..N-1) in row-major order.
    - Save a 2D FITS image with cell numbers ('*_voronoi_cell_numbers*.fits').
    - Save a text table with (x, y, cell) as '*_voronoi_output*.txt' (x,y remain 1-based for compatibility).
    - Export one 1D FITS spectrum per cell using '<base>_voronoi_cell_<CELL>.fits' naming.
    No S/N estimation or Voronoi binning is performed.
    """
    data_cube = result['data']              # Shape: (nz, ny, nx)
    header    = result['header']
    nz, ny, nx = data_cube.shape

    # Output directory, consistent with Voronoi-style individual spectra naming
    out_dir = os.path.join(args.output_dir, f"{base_filename}_individual_voronoi_spectra{suffix}")
    os.makedirs(out_dir, exist_ok=True)

    # Spectral axis information from the cube header
    cd1 = header.get('CDELT3', header.get('CD3_3', None))
    crval1 = header.get('CRVAL3', None)

    # Optional metadata
    instrume = header.get('INSTRUME', None)
    objname  = header.get('OBJECT', None)

    # Intensity units by instrument (aligned with your other outputs)
    if args.instrument == 'muse':
        bunit = '10**-16 erg cm-2 s-1 Angstrom-1'
    else:
        bunit = '10**-17 erg cm-2 s-1 Angstrom-1'

    # Build zero-based cell numbering and (x, y, cell) rows
    # Note: x,y are kept 1-based in the output table for compatibility with existing tools.
    cell_numbers = np.arange(ny * nx, dtype=np.int32).reshape(ny, nx)  # 0..(ny*nx-1), row-major
    rows = []
    total = ny * nx

    for y in range(ny):
        for x in range(nx):
            rows.append((x + 1, y + 1, int(cell_numbers[y, x])))

    # Save the cell number map as FITS (2D image)
    hdu_img = fits.PrimaryHDU(data=cell_numbers, header=header)
    cell_numbers_path = os.path.join(args.output_dir, f"{base_filename}_voronoi_cell_numbers{suffix}.fits")
    hdu_img.writeto(cell_numbers_path, overwrite=True)

    # Save (x, y, cell) as voronoi_output text file
    voronoi_output_file = os.path.join(args.output_dir, f"{base_filename}_voronoi_output{suffix}.txt")
    np.savetxt(voronoi_output_file, np.asarray(rows, dtype=float), fmt=b'%10.6f %10.6f %8i')

    # Export one 1D FITS spectrum per cell using Voronoi-like naming
    idx = 0
    for y in range(ny):
        for x in range(nx):
            idx += 1
            sys.stdout.write(f"\rExporting cell {idx}/{total} (x={x+1}, y={y+1}, cell={cell_numbers[y,x]})")
            sys.stdout.flush()

            spectrum = data_cube[:, y, x].astype(np.float32, copy=False)
            hdu = fits.PrimaryHDU(data=spectrum)
            hdr = hdu.header

            # Define 1D spectral axis keywords
            hdr['NAXIS']  = 1
            hdr['NAXIS1'] = nz
            hdr['CRPIX1'] = 1
            if crval1 is not None:
                hdr['CRVAL1'] = crval1
            if cd1 is not None:
                hdr['CDELT1'] = cd1
            hdr['CTYPE1'] = 'Wavelength'
            hdr['CUNIT1'] = 'Angstrom'
            hdr['BUNIT']  = bunit

            # Metadata and coordinates (X,Y remain 1-based for compatibility)
            if instrume is not None:
                hdr['INSTRUME'] = instrume
            if objname is not None:
                hdr['OBJECT']   = objname
            hdr['X'] = x + 1
            hdr['Y'] = y + 1
            hdr['CELL'] = int(cell_numbers[y, x])   # zero-based cell ID

            # Copy optional header keywords when present
            for k in ('WAVLIMF1', 'WAVLIMF2', 'VPH', 'PLATEIFU'):
                if k in header:
                    hdr[k] = header[k]

            # File name: <base>_voronoi_cell_<CELL>.fits  (CELL is zero-based)
            fname = f"{base_filename}_voronoi_cell_{int(cell_numbers[y,x])}{suffix}.fits"
            hdu.writeto(os.path.join(out_dir, fname), overwrite=True)

    print("\nExport completed.")


# ==========================
# Main Script
# ==========================


# Read the data cube based on the specified instrument
if args.instrument == 'megara':
    result = read_megara_cube(args.filename)
elif args.instrument == 'manga':
    result = read_manga_cube(args.filename)
elif args.instrument == 'muse':
    result = read_muse_cube(args.filename)

base_filename = os.path.basename(args.filename).replace('.fits', '')
suffix = f"_{args.suffix}" if args.suffix else ''

# Direct export mode: split the cube into individual spectra and exit
if args.split_cube_into_spectra:
    export_original_as_cells(result, base_filename, suffix)
    sys.exit(0)


# Calculate the valid pixels for S/N calculation based on the provided wavelength intervals, mask file and redshift parameters
valid_pixels = calculate_valid_pixels(result, wavelength_start=args.wavelength_start, wavelength_end=args.wavelength_end,
                                      apply_redshift=args.apply_redshift, redshift=args.redshift, mask_file=args.mask_file)


# Select and execute the appropriate method to estimate the signal-to-noise ratio based on the provided argument.
# Optionally, generate and save plots if specified.
if args.sn_method == 'spline':
    sn_coordinates = signal_to_noise_estimation_spline(result, valid_pixels, args.knots_number, result['lam_gal'], plot=args.plot)
elif args.sn_method == 'brightest_spaxel':
    sn_coordinates = signal_to_noise_estimation_brightest_spaxel(result, valid_pixels, args.knots_number, result['lam_gal'], plot=args.plot)
elif args.sn_method == 'signal_square_root':
    sn_coordinates = signal_to_noise_estimation_signal_square_root(result, valid_pixels, result['lam_gal'])

# Filter the sn_coordinates array to remove entries with a signal-to-noise ratio below the specified minimum.
# This ensures that only data points with a sufficient signal-to-noise ratio are retained for further analysis.
filtered_sn_coordinates = filter_by_signal_to_noise(sn_coordinates=sn_coordinates, min_sn=args.min_sn)

# Save the filtered_sn_coordinates to a new txt file
voronoi_input_file = os.path.join(args.output_dir, f"{base_filename}_voronoi_input{suffix}.txt")
np.savetxt(voronoi_input_file, filtered_sn_coordinates, fmt = '%.10f')


# Extract the x and y coordinates, signal, and noise from the filtered signal-to-noise coordinates array.
x, y, signal, noise = filtered_sn_coordinates[:,0], filtered_sn_coordinates[:,1], filtered_sn_coordinates[:,2], filtered_sn_coordinates[:,3]

# Set the target signal-to-noise ratio for the Voronoi binning.
target_sn = args.sn

# Define the path to save the Voronoi binning plot.
voronoi_binning_plot = os.path.join(args.output_dir, f"{base_filename}_voronoi_binning_sn_{target_sn}{suffix}.pdf")

# Run the Voronoi binning routine with the specified target signal-to-noise ratio.
# This routine groups the data points into bins to achieve the desired signal-to-noise ratio.
print(f"Starting Voronoi binning with {len(x)} data points, target S/N={target_sn}, pixelsize={result['pixelsize']}")
print("This may take several minutes...")
sys.stdout.flush()

binNum, xNode, yNode, xBar, yBar, sn, nPixels, scale = voronoi_2d_binning(x, y, signal, noise, target_sn, plot=True, quiet=1, pixelsize=result['pixelsize'])

print(f"Voronoi binning completed. Created {len(np.unique(binNum))} bins.")
sys.stdout.flush()

# Save the Voronoi binning plot to the specified file.
plt.savefig(voronoi_binning_plot, dpi=600)


voronoi_output_file = os.path.join(args.output_dir, f"{base_filename}_voronoi_output{suffix}.txt")
np.savetxt(voronoi_output_file, np.column_stack([x, y, binNum]),
           fmt='%10.6f %10.6f %8i')

# Create index matrices
indices_x, indices_y = np.meshgrid(np.arange(result['data'].shape[1]), np.arange(result['data'].shape[2]), indexing='ij')
# Flatten the index matrices
x_flat = indices_x.flatten()
y_flat = indices_y.flatten()
# voronoi_bins = np.column_stack((x_flat, y_flat, bin_num))
voronoi_bins = np.column_stack((x.astype(int), y.astype(int), binNum))


# Generate the Voronoi cube by averaging the spectra of spaxels within each Voronoi cell
voronoi_cube_data = generate_voronoi_cubes(result['data'], voronoi_bins, result['header'], generate_individual_spectra = args.generate_individual_spectra)


# plt.plot(lam_gal[valid_pixels], result['data'][valid_pixels,25,25],'b-', linewidth = 0.25, label='Input spectrum')
# plt.show()

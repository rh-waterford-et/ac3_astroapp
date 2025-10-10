package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/uc3/experiment-tool/pkg/config"
	"github.com/uc3/experiment-tool/pkg/dataset"
	"github.com/uc3/experiment-tool/pkg/orchestrator"
)

// NewStartCmd creates the start command for experiments
func NewStartCmd(cfg *config.ExperimentConfig) *cobra.Command {
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start an autoscaling experiment",
		Long: `Start an autoscaling experiment with the specified configuration.

This will process datasets with the configured processor count while
collecting performance metrics for training data generation.`,
		Run: func(cmd *cobra.Command, args []string) {
			startExperiment(cfg)
		},
	}

	// Start command flags
	startCmd.Flags().StringP("name", "n", "", "experiment name")
	startCmd.Flags().IntP("processor-count", "", 0, "number of processor pods")
	startCmd.Flags().StringP("dataset-path", "", "", "local path to dataset directory")
	startCmd.Flags().StringP("processor-type", "", "", "processor type (starlight or ppxf)")
	startCmd.Flags().StringP("output-dir", "o", "", "output directory for experiment data")
	startCmd.Flags().BoolP("upload-dataset", "", true, "upload dataset to S3 before experiment")

	// Bind flags to viper
	viper.BindPFlag("name", startCmd.Flags().Lookup("name"))
	viper.BindPFlag("scaling.processor_count", startCmd.Flags().Lookup("processor-count"))
	viper.BindPFlag("workload.dataset", startCmd.Flags().Lookup("dataset-path"))
	viper.BindPFlag("workload.processor_type", startCmd.Flags().Lookup("processor-type"))
	viper.BindPFlag("metrics.output_directory", startCmd.Flags().Lookup("output-dir"))
	viper.BindPFlag("upload-dataset", startCmd.Flags().Lookup("upload-dataset"))

	return startCmd
}

func startExperiment(cfg *config.ExperimentConfig) {
	fmt.Printf("Starting UC3 Autoscaling Experiment: %s\n", cfg.Name)

	// Create experiment controller (always use multi-dataset)
	datasets := cfg.Workload.GetDatasets()
	fmt.Printf("Running experiment with %d dataset(s)\n", len(datasets))

	controller, err := orchestrator.NewMultiDatasetController(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create controller: %v\n", err)
		os.Exit(1)
	}
	defer controller.Close()

	// Run the experiment
	err = controller.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Experiment failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Experiment completed successfully!\n")
}

// NewStartCmdWithConfigLoader creates the start command that loads config dynamically
func NewStartCmdWithConfigLoader(cfgFile *string) *cobra.Command {
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start an autoscaling experiment",
		Long: `Start an autoscaling experiment with the specified configuration.

This will process datasets with the configured processor count while
collecting performance metrics for training data generation.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load config with the provided config file
			cfg, err := config.LoadConfig(*cfgFile)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			// Start the experiment with the loaded config
			return startExperimentWithConfig(cfg)
		},
	}

	// Start command flags
	startCmd.Flags().StringP("name", "n", "", "experiment name")
	startCmd.Flags().IntP("processor-count", "", 0, "number of processor pods")
	startCmd.Flags().StringP("dataset-path", "", "", "local path to dataset directory")
	startCmd.Flags().StringP("processor-type", "", "", "processor type (starlight or ppxf)")
	startCmd.Flags().StringP("output-dir", "o", "", "output directory for experiment data")
	startCmd.Flags().BoolP("upload-dataset", "", true, "upload dataset to S3 before experiment")

	// Bind flags to viper
	viper.BindPFlag("name", startCmd.Flags().Lookup("name"))
	viper.BindPFlag("scaling.processor_count", startCmd.Flags().Lookup("processor-count"))
	viper.BindPFlag("workload.dataset", startCmd.Flags().Lookup("dataset-path"))
	viper.BindPFlag("workload.processor_type", startCmd.Flags().Lookup("processor-type"))
	viper.BindPFlag("metrics.output_directory", startCmd.Flags().Lookup("output-dir"))
	viper.BindPFlag("upload-dataset", startCmd.Flags().Lookup("upload-dataset"))

	return startCmd
}

func startExperimentWithConfig(cfg *config.ExperimentConfig) error {
	fmt.Printf("Starting UC3 Autoscaling Experiment: %s\n", cfg.Name)

	// Always use multi-dataset controller (handles single dataset too)
	datasets := cfg.Workload.GetDatasets()
	fmt.Printf("Running experiment with %d dataset(s)\n", len(datasets))

	controller, err := orchestrator.NewMultiDatasetController(cfg)
	if err != nil {
		return fmt.Errorf("failed to create controller: %w", err)
	}
	defer controller.Close()

	// Run the experiment
	err = controller.Run()
	if err != nil {
		return fmt.Errorf("experiment failed: %w", err)
	}

	fmt.Printf("Experiment completed successfully!\n")
	return nil
}

// runDatasetUpload handles dataset upload and returns the dataset name
func runDatasetUpload(cfg *config.ExperimentConfig, localPath string) string {
	// Create temporary dataset manager for scanning
	tempDM, err := dataset.NewDatasetManager("temp", cfg.Workload.ProcessorType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating dataset manager: %v\n", err)
		os.Exit(1)
	}

	// Scan local dataset to extract dataset name
	localDataset, err := tempDM.ScanLocalDataset(localPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning dataset: %v\n", err)
		os.Exit(1)
	}

	if len(localDataset.Files) == 0 {
		fmt.Fprintf(os.Stderr, "No files found for processor type '%s' in %s\n", cfg.Workload.ProcessorType, localPath)
		os.Exit(1)
	}

	// Use extracted dataset name or fallback
	datasetName := localDataset.DatasetName
	if datasetName == "" {
		datasetName = "unknown-dataset"
	}

	fmt.Printf("  Dataset identified as: %s\n", datasetName)
	fmt.Printf("  Found %d files, uploading...\n", len(localDataset.Files))

	// Create final dataset manager with correct dataset name
	dm, err := dataset.NewDatasetManager(datasetName, cfg.Workload.ProcessorType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating dataset manager: %v\n", err)
		os.Exit(1)
	}

	// Upload to S3
	err = dm.UploadDataset(localDataset)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Upload failed: %v\n", err)
		os.Exit(1)
	}

	return datasetName
}

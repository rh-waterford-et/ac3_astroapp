package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/uc3/experiment-tool/pkg/config"
	"github.com/uc3/experiment-tool/pkg/dataset"
	"github.com/uc3/experiment-tool/pkg/scaler"
)

// NewStartCmd creates the start command for experiments
func NewStartCmd(cfg *config.ExperimentConfig) *cobra.Command {
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start an autoscaling experiment",
		Long: `Start an autoscaling experiment with the specified configuration.

This will begin systematic testing of different processor counts while
collecting performance metrics for training data generation.`,
		Run: func(cmd *cobra.Command, args []string) {
			startExperiment(cfg)
		},
	}

	// Start command flags
	startCmd.Flags().StringP("name", "n", "", "experiment name")
	startCmd.Flags().DurationP("duration", "d", 0, "experiment duration (e.g., 30m, 1h)")
	startCmd.Flags().IntP("min-processors", "", 0, "minimum number of processors")
	startCmd.Flags().IntP("max-processors", "", 0, "maximum number of processors")
	startCmd.Flags().StringP("dataset-path", "", "", "local path to dataset directory")
	startCmd.Flags().StringP("processor-type", "", "", "processor type (starlight or ppxf)")
	startCmd.Flags().StringP("output-dir", "o", "", "output directory for experiment data")
	startCmd.Flags().BoolP("upload-dataset", "", true, "upload dataset to S3 before experiment")

	// Bind flags to viper
	viper.BindPFlag("name", startCmd.Flags().Lookup("name"))
	viper.BindPFlag("duration", startCmd.Flags().Lookup("duration"))
	viper.BindPFlag("scaling.min_processors", startCmd.Flags().Lookup("min-processors"))
	viper.BindPFlag("scaling.max_processors", startCmd.Flags().Lookup("max-processors"))
	viper.BindPFlag("workload.dataset", startCmd.Flags().Lookup("dataset-path"))
	viper.BindPFlag("workload.processor_type", startCmd.Flags().Lookup("processor-type"))
	viper.BindPFlag("metrics.output_directory", startCmd.Flags().Lookup("output-dir"))
	viper.BindPFlag("upload-dataset", startCmd.Flags().Lookup("upload-dataset"))

	return startCmd
}

func startExperiment(cfg *config.ExperimentConfig) {
	fmt.Printf("Starting UC3 Autoscaling Experiment: %s\n", cfg.Name)
	fmt.Printf("Duration: %s\n", cfg.Duration)
	fmt.Printf("Processor Range: %d-%d\n", cfg.Scaling.MinProcessors, cfg.Scaling.MaxProcessors)
	fmt.Printf("Dataset: %s\n", cfg.Workload.Dataset)
	fmt.Printf("Output Directory: %s\n", cfg.Metrics.OutputDirectory)
	fmt.Printf("Processor Type: %s\n", cfg.Workload.ProcessorType)

	// Step 1: Upload dataset if path provided
	datasetPath := viper.GetString("dataset-path")
	uploadDataset := viper.GetBool("upload-dataset")

	var datasetName string
	if datasetPath != "" && uploadDataset {
		fmt.Printf("\nStep 1: Uploading dataset from %s\n", datasetPath)
		datasetName = runDatasetUpload(cfg, datasetPath)
	} else {
		// Use dataset from config if no path provided
		datasetName = cfg.Workload.Dataset
		if datasetName == "" {
			fmt.Fprintf(os.Stderr, "Error: No dataset specified. Use --dataset-path or set workload.dataset in config\n")
			os.Exit(1)
		}
		fmt.Printf("\nStep 1: Using existing dataset: %s\n", datasetName)
	}

	// Step 2: Set initial processor count
	fmt.Printf("\nStep 2: Setting initial processor count to %d\n", cfg.Scaling.MinProcessors)
	err := setProcessorCount(cfg, cfg.Scaling.MinProcessors)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set processor count: %v\n", err)
		os.Exit(1)
	}

	// Step 3: Trigger processing
	fmt.Printf("\nStep 3: Triggering UC3 processing for dataset: %s\n", datasetName)
	err = triggerProcessing(cfg, datasetName, cfg.Workload.ProcessorType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to trigger processing: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  Processing triggered successfully\n")

	fmt.Printf("\nExperiment started successfully!\n")
	fmt.Printf("Metrics collection and scaling logic will be implemented in the next phase.\n")
	fmt.Printf("Monitor processing progress in the UC3 dashboard or logs.\n")
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

// setProcessorCount sets the Kubernetes deployment replica count
func setProcessorCount(cfg *config.ExperimentConfig, replicas int) error {
	// Create scaler
	scalerConfig := scaler.ScalerConfig{
		KubeConfig:     cfg.Infrastructure.KubeConfig,
		Namespace:      cfg.Infrastructure.Namespace,
		DeploymentName: cfg.Infrastructure.DeploymentName,
		Timeout:        30 * time.Second,
	}

	k8sScaler, err := scaler.NewKubernetesScaler(scalerConfig)
	if err != nil {
		return fmt.Errorf("failed to create scaler: %w", err)
	}

	// Validate cluster connection
	ctx := context.Background()
	if err := k8sScaler.ValidateClusterConnection(ctx); err != nil {
		return fmt.Errorf("cluster connection validation failed: %w", err)
	}

	// Get current scale before scaling
	currentReplicas, err := k8sScaler.GetCurrentScale(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current scale: %w", err)
	}

	// Scale deployment
	err = k8sScaler.Scale(ctx, int32(replicas))
	if err != nil {
		return fmt.Errorf("failed to scale deployment: %w", err)
	}

	fmt.Printf("  Scaled %s to %d replicas (was %d)\n",
		cfg.Infrastructure.DeploymentName, replicas, currentReplicas)

	return nil
}

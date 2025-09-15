package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/uc3/experiment-tool/pkg/api"
	"github.com/uc3/experiment-tool/pkg/config"
	"github.com/uc3/experiment-tool/pkg/dataset"
)

// NewDatasetCmd creates the dataset command and its subcommands
func NewDatasetCmd(cfg *config.ExperimentConfig) *cobra.Command {
	datasetCmd := &cobra.Command{
		Use:   "dataset",
		Short: "Dataset management commands",
		Long: `Manage local datasets and S3 uploads for experiments.

This command helps you scan local datasets, test S3 connectivity, and upload files
for processing by the UC3 system.`,
	}

	datasetScanCmd := &cobra.Command{
		Use:   "scan [path]",
		Short: "Scan local directory for dataset files",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			datasetScan(cfg, args[0])
		},
	}

	datasetTestCmd := &cobra.Command{
		Use:   "test-s3",
		Short: "Test S3 connection with UC3 credentials",
		Run: func(cmd *cobra.Command, args []string) {
			datasetTestS3(cfg)
		},
	}

	datasetUploadCmd := &cobra.Command{
		Use:   "upload [path]",
		Short: "Upload local dataset to S3 and optionally trigger processing",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			trigger, _ := cmd.Flags().GetBool("trigger")
			datasetUpload(cfg, args[0], trigger)
		},
	}

	datasetTriggerCmd := &cobra.Command{
		Use:   "trigger [dataset-name]",
		Short: "Trigger UC3 processing for a dataset",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			datasetTrigger(cfg, args[0])
		},
	}

	// Add trigger flag to upload command
	datasetUploadCmd.Flags().BoolP("trigger", "t", false, "trigger processing after upload")

	datasetCmd.AddCommand(datasetScanCmd)
	datasetCmd.AddCommand(datasetTestCmd)
	datasetCmd.AddCommand(datasetUploadCmd)
	datasetCmd.AddCommand(datasetTriggerCmd)

	return datasetCmd
}

func datasetScan(cfg *config.ExperimentConfig, localPath string) {
	fmt.Printf("Scanning local dataset: %s\n", localPath)

	// Create dataset manager
	dm, err := dataset.NewDatasetManager("test-scan", cfg.Workload.ProcessorType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating dataset manager: %v\n", err)
		os.Exit(1)
	}

	// Scan local dataset
	localDataset, err := dm.ScanLocalDataset(localPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning dataset: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nDataset scan results:\n")
	fmt.Printf("  Path: %s\n", localDataset.Path)
	fmt.Printf("  Dataset name: %s\n", localDataset.DatasetName)
	fmt.Printf("  Files found: %d\n", len(localDataset.Files))
	fmt.Printf("  Total size: %.2f MB\n", float64(localDataset.TotalSize)/(1024*1024))

	if len(localDataset.Files) > 0 {
		fmt.Printf("\nFirst 5 files:\n")
		for i, file := range localDataset.Files {
			if i >= 5 {
				fmt.Printf("  ... and %d more files\n", len(localDataset.Files)-5)
				break
			}
			fmt.Printf("  %s\n", file)
		}
	}
}

func datasetTestS3(cfg *config.ExperimentConfig) {
	fmt.Printf("Testing S3 connection with UC3 credentials...\n")

	// Create dataset manager
	dm, err := dataset.NewDatasetManager("connection-test", cfg.Workload.ProcessorType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating dataset manager: %v\n", err)
		os.Exit(1)
	}

	// Test S3 connection
	err = dm.TestS3Connection()
	if err != nil {
		fmt.Fprintf(os.Stderr, "S3 connection failed: %v\n", err)
		fmt.Printf("\nTroubleshooting:\n")
		fmt.Printf("  • Ensure UC3 S3 environment variables are set:\n")
		fmt.Printf("    - AWS_ACCESS_KEY_ID\n")
		fmt.Printf("    - AWS_SECRET_ACCESS_KEY\n")
		fmt.Printf("    - S3_ENDPOINT\n")
		fmt.Printf("    - S3_REGION\n")
		fmt.Printf("    - S3_BUCKET_NAME\n")
		os.Exit(1)
	}
}

func datasetUpload(cfg *config.ExperimentConfig, localPath string, trigger bool) {
	fmt.Printf("Uploading dataset: %s\n", localPath)

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
		fmt.Printf("No files found for processor type '%s' in %s\n", cfg.Workload.ProcessorType, localPath)
		return
	}

	// Use extracted dataset name or fallback
	datasetName := localDataset.DatasetName
	if datasetName == "" {
		datasetName = "unknown-dataset"
	}

	fmt.Printf("Dataset identified as: %s\n", datasetName)
	fmt.Printf("Found %d files, proceeding with upload...\n", len(localDataset.Files))

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

	// Trigger processing if requested
	if trigger {
		fmt.Printf("\nTriggering processing for dataset: %s\n", datasetName)
		err = triggerProcessing(cfg, datasetName, cfg.Workload.ProcessorType)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to trigger processing: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("  Processing triggered successfully\n")
	}
}

func datasetTrigger(cfg *config.ExperimentConfig, datasetName string) {
	fmt.Printf("Triggering processing for dataset: %s\n", datasetName)

	err := triggerProcessing(cfg, datasetName, cfg.Workload.ProcessorType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to trigger processing: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Processing triggered successfully\n")
}

func triggerProcessing(cfg *config.ExperimentConfig, datasetName, processorType string) error {
	// Create UC3 API client
	client := api.NewUC3Client(cfg.Infrastructure.UC3APIBaseURL)

	// Test connection first
	err := client.TestConnection()
	if err != nil {
		return fmt.Errorf("failed to connect to UC3 API: %w", err)
	}

	// Trigger processing
	return client.TriggerProcessing(datasetName, processorType)
}

package main

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

var (
	cfgFile string
	cfg     *config.ExperimentConfig
)

var rootCmd = &cobra.Command{
	Use:   "uc3-experiment",
	Short: "UC3 Autoscaling Experiment Tool",
	Long: `UC3 Autoscaling Experiment Tool generates training data for autoscaling AI models.
	
This tool systematically varies processor counts, workload patterns, and job characteristics
while collecting detailed performance metrics from the UC3 astronomical application.`,
	Version: "0.1.0",
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("UC3 Experiment Tool v%s\n", rootCmd.Version)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current experiment status",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("No experiment currently running")
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management commands",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Run: func(cmd *cobra.Command, args []string) {
		showConfig()
	},
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration file",
	Run: func(cmd *cobra.Command, args []string) {
		validateConfig()
	},
}

var scaleCmd = &cobra.Command{
	Use:   "scale",
	Short: "Kubernetes scaling operations",
	Long: `Scale the UC3 processor deployment up or down.
	
This command allows you to manually scale the processor deployment
for testing or to prepare for experiments.`,
}

var scaleSetCmd = &cobra.Command{
	Use:   "set [replicas]",
	Short: "Set the number of processor replicas",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		scaleSet(args[0])
	},
}

var scaleInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show current deployment information",
	Run: func(cmd *cobra.Command, args []string) {
		scaleInfo()
	},
}

var scaleTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test cluster connection and permissions",
	Run: func(cmd *cobra.Command, args []string) {
		scaleTest()
	},
}

var datasetCmd = &cobra.Command{
	Use:   "dataset",
	Short: "Dataset management commands",
	Long: `Manage local datasets and S3 uploads for experiments.
	
These commands help you prepare local astronomical datasets
for processing by the UC3 system.`,
}

var datasetScanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "Scan local directory for dataset files",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		datasetScan(args[0])
	},
}

var datasetTestCmd = &cobra.Command{
	Use:   "test-s3",
	Short: "Test S3 connection with UC3 credentials",
	Run: func(cmd *cobra.Command, args []string) {
		datasetTestS3()
	},
}

var datasetUploadCmd = &cobra.Command{
	Use:   "upload [path]",
	Short: "Upload local dataset to S3 (test only)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		datasetUpload(args[0])
	},
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start an autoscaling experiment",
	Long: `Start an autoscaling experiment with the specified configuration.
	
This will begin systematic testing of different processor counts while
collecting performance metrics for training data generation.`,
	Run: func(cmd *cobra.Command, args []string) {
		startExperiment()
	},
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./configs/defaults/experiment.yaml)")

	// Start command flags
	startCmd.Flags().StringP("name", "n", "", "experiment name")
	startCmd.Flags().DurationP("duration", "d", 0, "experiment duration (e.g., 30m, 1h)")
	startCmd.Flags().IntP("min-processors", "", 0, "minimum number of processors")
	startCmd.Flags().IntP("max-processors", "", 0, "maximum number of processors")
	startCmd.Flags().StringP("dataset-path", "", "", "local path to dataset directory")
	startCmd.Flags().StringP("processor-type", "", "", "processor type (starlight or ppxf)")
	startCmd.Flags().StringP("output-dir", "o", "", "output directory for experiment data")
	startCmd.Flags().BoolP("upload-dataset", "", true, "upload dataset to S3 before experiment")

	// Scale command flags (defined after scaleSetCmd is created)
	// These will be added in the init function after scaleSetCmd is defined

	// Bind flags to viper
	viper.BindPFlag("name", startCmd.Flags().Lookup("name"))
	viper.BindPFlag("duration", startCmd.Flags().Lookup("duration"))
	viper.BindPFlag("scaling.min_processors", startCmd.Flags().Lookup("min-processors"))
	viper.BindPFlag("scaling.max_processors", startCmd.Flags().Lookup("max-processors"))
	viper.BindPFlag("workload.dataset", startCmd.Flags().Lookup("dataset"))
	viper.BindPFlag("workload.processor_type", startCmd.Flags().Lookup("processor-type"))
	viper.BindPFlag("metrics.output_directory", startCmd.Flags().Lookup("output-dir"))

	// Add subcommands
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configValidateCmd)

	scaleCmd.AddCommand(scaleSetCmd)
	scaleCmd.AddCommand(scaleInfoCmd)
	scaleCmd.AddCommand(scaleTestCmd)

	datasetCmd.AddCommand(datasetScanCmd)
	datasetCmd.AddCommand(datasetTestCmd)
	datasetCmd.AddCommand(datasetUploadCmd)

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(scaleCmd)
	rootCmd.AddCommand(datasetCmd)
	rootCmd.AddCommand(startCmd)
}

// initConfig reads in config file and ENV variables
func initConfig() {
	var err error
	cfg, err = config.LoadConfig(cfgFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}
}

func showConfig() {
	fmt.Printf("Current Configuration:\n")
	fmt.Printf("=====================\n\n")

	fmt.Printf("Experiment:\n")
	fmt.Printf("  Name: %s\n", cfg.Name)
	fmt.Printf("  Duration: %s\n", cfg.Duration)
	fmt.Printf("  Description: %s\n\n", cfg.Description)

	fmt.Printf("Scaling:\n")
	fmt.Printf("  Min Processors: %d\n", cfg.Scaling.MinProcessors)
	fmt.Printf("  Max Processors: %d\n", cfg.Scaling.MaxProcessors)
	fmt.Printf("  Scale Steps: %v\n", cfg.Scaling.ScaleSteps)
	fmt.Printf("  Stabilize Time: %s\n\n", cfg.Scaling.StabilizeTime)

	fmt.Printf("Workload:\n")
	fmt.Printf("  Dataset: %s\n", cfg.Workload.Dataset)
	fmt.Printf("  Processor Type: %s\n", cfg.Workload.ProcessorType)
	fmt.Printf("  Batch Size: %d\n", cfg.Workload.BatchSize)
	fmt.Printf("  Submission Rate: %s\n\n", cfg.Workload.SubmissionRate)

	fmt.Printf("Metrics:\n")
	fmt.Printf("  Collection Interval: %s\n", cfg.Metrics.CollectionInterval)
	fmt.Printf("  Export Format: %s\n", cfg.Metrics.ExportFormat)
	fmt.Printf("  Output Directory: %s\n\n", cfg.Metrics.OutputDirectory)

	fmt.Printf("Infrastructure:\n")
	fmt.Printf("  Namespace: %s\n", cfg.Infrastructure.Namespace)
	fmt.Printf("  Deployment: %s\n", cfg.Infrastructure.DeploymentName)
	fmt.Printf("  UC3 API: %s\n", cfg.Infrastructure.UC3APIBaseURL)
	fmt.Printf("  Redis: %s:%d\n", cfg.Infrastructure.RedisHost, cfg.Infrastructure.RedisPort)
}

func validateConfig() {
	fmt.Printf("Validating configuration...\n")

	// Configuration is already validated in LoadConfig
	fmt.Printf("✓ Configuration is valid\n")

	// Show key settings
	fmt.Printf("\nKey Settings:\n")
	fmt.Printf("  Processors: %d-%d\n", cfg.Scaling.MinProcessors, cfg.Scaling.MaxProcessors)
	fmt.Printf("  Duration: %s\n", cfg.Duration)
	fmt.Printf("  Dataset: %s\n", cfg.Workload.Dataset)
	fmt.Printf("  Output: %s\n", cfg.Metrics.OutputDirectory)
}

func createScaler() (*scaler.KubernetesScaler, error) {
	scalerConfig := scaler.ScalerConfig{
		Namespace:      cfg.Infrastructure.Namespace,
		DeploymentName: cfg.Infrastructure.DeploymentName,
		KubeConfig:     cfg.Infrastructure.KubeConfig,
		Timeout:        5 * time.Minute,
	}

	k8sScaler, err := scaler.NewKubernetesScaler(scalerConfig)
	if err != nil {
		return nil, err
	}

	// Validate cluster connection
	ctx := context.Background()
	if err := k8sScaler.ValidateClusterConnection(ctx); err != nil {
		return nil, fmt.Errorf("cluster connection validation failed: %w", err)
	}

	return k8sScaler, nil
}

func scaleSet(replicasStr string) {
	// Parse replicas
	var replicas int32
	if _, err := fmt.Sscanf(replicasStr, "%d", &replicas); err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid replica count '%s': %v\n", replicasStr, err)
		os.Exit(1)
	}

	// Validate replicas
	if replicas < 0 {
		fmt.Fprintf(os.Stderr, "Error: replica count cannot be negative\n")
		os.Exit(1)
	}
	if replicas > 50 {
		fmt.Fprintf(os.Stderr, "Error: replica count should not exceed 50 for safety\n")
		os.Exit(1)
	}

	// Create scaler
	k8sScaler, err := createScaler()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating Kubernetes scaler: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Get current scale
	currentReplicas, err := k8sScaler.GetCurrentScale(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current scale: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Scaling deployment %s/%s from %d to %d replicas...\n",
		cfg.Infrastructure.Namespace, cfg.Infrastructure.DeploymentName, currentReplicas, replicas)

	// Perform scaling
	result := k8sScaler.ScaleWithResult(ctx, replicas)
	if !result.Success {
		fmt.Fprintf(os.Stderr, "Error scaling deployment: %v\n", result.Error)
		os.Exit(1)
	}

	fmt.Printf("✓ Successfully scaled from %d to %d replicas in %v\n",
		result.PreviousReplicas, result.NewReplicas, result.Duration)

	// Note: Wait functionality will be added in next iteration
	// For now, scaling operation is complete
}

func scaleInfo() {
	// Create scaler
	k8sScaler, err := createScaler()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating Kubernetes scaler: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Get deployment info
	info, err := k8sScaler.GetDeploymentInfo(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting deployment info: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Deployment Information:\n")
	fmt.Printf("======================\n\n")
	fmt.Printf("Name: %s\n", info.Name)
	fmt.Printf("Namespace: %s\n", info.Namespace)
	fmt.Printf("Desired Replicas: %d\n", info.Replicas)
	fmt.Printf("Ready Replicas: %d\n", info.Ready)
	fmt.Printf("Available Replicas: %d\n", info.Available)

	if len(info.Labels) > 0 {
		fmt.Printf("\nLabels:\n")
		for key, value := range info.Labels {
			fmt.Printf("  %s: %s\n", key, value)
		}
	}
}

func scaleTest() {
	fmt.Printf("Testing cluster connection and permissions...\n")
	fmt.Printf("Target: %s/%s\n\n", cfg.Infrastructure.Namespace, cfg.Infrastructure.DeploymentName)

	// Test cluster connection
	fmt.Printf("1. Testing cluster connection...")
	k8sScaler, err := createScaler()
	if err != nil {
		fmt.Printf(" FAILED\n")
		fmt.Fprintf(os.Stderr, "\nError: %v\n\n", err)

		// Provide helpful guidance
		fmt.Printf("To fix this issue:\n")
		fmt.Printf("   • Run 'oc login' to authenticate with your OpenShift cluster\n")
		fmt.Printf("   • Or run 'kubectl config current-context' to check your Kubernetes context\n")
		fmt.Printf("   • Ensure you have access to the '%s' namespace\n", cfg.Infrastructure.Namespace)
		os.Exit(1)
	}
	fmt.Printf(" SUCCESS\n")

	// Test deployment access
	fmt.Printf("2. Testing deployment access...")
	ctx := context.Background()
	info, err := k8sScaler.GetDeploymentInfo(ctx)
	if err != nil {
		fmt.Printf(" FAILED\n")
		fmt.Fprintf(os.Stderr, "\nError: %v\n\n", err)

		fmt.Printf("💡 Possible issues:\n")
		fmt.Printf("   • Deployment '%s' doesn't exist in namespace '%s'\n", cfg.Infrastructure.DeploymentName, cfg.Infrastructure.Namespace)
		fmt.Printf("   • You don't have permission to access this namespace\n")
		fmt.Printf("   • Check available deployments: oc get deployments -n %s\n", cfg.Infrastructure.Namespace)
		os.Exit(1)
	}
	fmt.Printf(" SUCCESS\n")

	// Show deployment info
	fmt.Printf("\nAll tests passed! Deployment details:\n")
	fmt.Printf("   Name: %s\n", info.Name)
	fmt.Printf("   Namespace: %s\n", info.Namespace)
	fmt.Printf("   Current Replicas: %d\n", info.Replicas)
	fmt.Printf("   Ready Replicas: %d\n", info.Ready)
	fmt.Printf("   Available Replicas: %d\n", info.Available)

	fmt.Printf("\nReady to perform scaling operations!\n")
	fmt.Printf("   Try: ./uc3-experiment scale set 2\n")
}

func datasetScan(localPath string) {
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

func datasetTestS3() {
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
		fmt.Printf("\nTo fix this issue:\n")
		fmt.Printf("  • Ensure UC3 S3 environment variables are set:\n")
		fmt.Printf("    - AWS_ACCESS_KEY_ID\n")
		fmt.Printf("    - AWS_SECRET_ACCESS_KEY\n")
		fmt.Printf("    - S3_ENDPOINT\n")
		fmt.Printf("    - S3_REGION\n")
		fmt.Printf("    - S3_BUCKET_NAME\n")
		os.Exit(1)
	}
}

func datasetUpload(localPath string) {
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
}

func startExperiment() {
	fmt.Printf("Starting UC3 Autoscaling Experiment: %s\n", cfg.Name)
	fmt.Printf("Duration: %s\n", cfg.Duration)
	fmt.Printf("Processor Range: %d-%d\n", cfg.Scaling.MinProcessors, cfg.Scaling.MaxProcessors)
	fmt.Printf("Dataset: %s\n", cfg.Workload.Dataset)
	fmt.Printf("Output Directory: %s\n", cfg.Metrics.OutputDirectory)

	fmt.Printf("\nExperiment orchestration not yet implemented\n")
	fmt.Printf("This will be implemented in the next phase.\n")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

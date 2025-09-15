package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/uc3/experiment-tool/pkg/config"
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
	startCmd.Flags().StringP("dataset", "", "", "dataset to process")
	startCmd.Flags().StringP("processor-type", "", "", "processor type (starlight or ppxf)")
	startCmd.Flags().StringP("output-dir", "o", "", "output directory for experiment data")

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

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(configCmd)
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

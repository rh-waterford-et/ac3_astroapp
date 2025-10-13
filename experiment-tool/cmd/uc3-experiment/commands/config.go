package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/uc3/experiment-tool/pkg/config"
)

// NewConfigCmd creates the config command and its subcommands
func NewConfigCmd(cfg *config.ExperimentConfig) *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management commands",
	}

	configShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load config from file
			configFile := viper.GetString("config")
			if configFile == "" {
				configFile = "./configs/defaults/experiment.yaml"
			}

			loadedCfg, err := config.LoadConfig(configFile)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			showConfig(loadedCfg)
			return nil
		},
	}

	configValidateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load config from file
			configFile := viper.GetString("config")
			if configFile == "" {
				configFile = "./configs/defaults/experiment.yaml"
			}

			loadedCfg, err := config.LoadConfig(configFile)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			validateConfig(loadedCfg)
			return nil
		},
	}

	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configValidateCmd)

	return configCmd
}

func showConfig(cfg *config.ExperimentConfig) {
	fmt.Printf("UC3 Experiment Configuration:\n")
	fmt.Printf("=============================\n\n")

	fmt.Printf("Experiment Details:\n")
	fmt.Printf("  Name: %s\n", cfg.Name)
	fmt.Printf("  Description: %s\n", cfg.Description)

	fmt.Printf("\nScaling Configuration:\n")
	fmt.Printf("  Processor Count: %d\n", cfg.Scaling.ProcessorCount)
	fmt.Printf("  Stabilize Time: %s\n", cfg.Scaling.StabilizeTime)

	fmt.Printf("\nWorkload Configuration:\n")
	if cfg.Workload.IsMultiDataset() {
		fmt.Printf("  Mode: Multi-Dataset\n")
		fmt.Printf("  Datasets: %d\n", len(cfg.Workload.Datasets))
		for i, ds := range cfg.Workload.Datasets {
			fmt.Printf("    %d. %s (%s)\n", i+1, ds.Name, ds.ProcessorType)
		}
		fmt.Printf("  Failure Strategy: %s\n", cfg.Workload.GetFailureStrategy())
		fmt.Printf("  Dataset Start Interval: %s\n", cfg.Workload.GetDatasetStartInterval())
	} else {
		fmt.Printf("  Mode: Single-Dataset\n")
		fmt.Printf("  Dataset: %s\n", cfg.Workload.Dataset)
		fmt.Printf("  Processor Type: %s\n", cfg.Workload.ProcessorType)
	}

	fmt.Printf("\nInfrastructure:\n")
	fmt.Printf("  Namespace: %s\n", cfg.Infrastructure.Namespace)
	fmt.Printf("  Deployment: %s\n", cfg.Infrastructure.DeploymentName)
	fmt.Printf("  UC3 API: %s\n", cfg.Infrastructure.UC3APIBaseURL)

	fmt.Printf("\nMetrics:\n")
	fmt.Printf("  Output Directory: %s\n", cfg.Metrics.OutputDirectory)
}

func validateConfig(cfg *config.ExperimentConfig) {
	fmt.Printf("Validating experiment configuration...\n\n")

	valid := true

	// Validate experiment basics
	if cfg.Name == "" {
		fmt.Printf("❌ Missing experiment name\n")
		valid = false
	} else {
		fmt.Printf("✅ Experiment name: %s\n", cfg.Name)
	}

	// Validate scaling configuration
	if cfg.Scaling.ProcessorCount <= 0 {
		fmt.Printf("❌ Invalid processor count: %d (must be > 0)\n", cfg.Scaling.ProcessorCount)
		valid = false
	} else if cfg.Scaling.ProcessorCount > 50 {
		fmt.Printf("❌ Processor count %d exceeds safety limit of 50\n", cfg.Scaling.ProcessorCount)
		valid = false
	} else {
		fmt.Printf("✅ Processor count: %d\n", cfg.Scaling.ProcessorCount)
	}

	// Validate infrastructure
	if cfg.Infrastructure.Namespace == "" {
		fmt.Printf("❌ Missing Kubernetes namespace\n")
		valid = false
	} else {
		fmt.Printf("✅ Namespace: %s\n", cfg.Infrastructure.Namespace)
	}

	if cfg.Infrastructure.DeploymentName == "" {
		fmt.Printf("❌ Missing deployment name\n")
		valid = false
	} else {
		fmt.Printf("✅ Deployment: %s\n", cfg.Infrastructure.DeploymentName)
	}

	fmt.Printf("\n")
	if valid {
		fmt.Printf("🎉 Configuration is valid!\n")
	} else {
		fmt.Printf("💥 Configuration has errors. Please fix them before running experiments.\n")
		os.Exit(1)
	}
}

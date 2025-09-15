package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
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
		Run: func(cmd *cobra.Command, args []string) {
			showConfig(cfg)
		},
	}

	configValidateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration file",
		Run: func(cmd *cobra.Command, args []string) {
			validateConfig(cfg)
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
	fmt.Printf("  Duration: %s\n", cfg.Duration)
	fmt.Printf("  Description: %s\n", cfg.Description)

	fmt.Printf("\nScaling Configuration:\n")
	fmt.Printf("  Processor Range: %d-%d\n", cfg.Scaling.MinProcessors, cfg.Scaling.MaxProcessors)
	fmt.Printf("  Scale Steps: %v\n", cfg.Scaling.ScaleSteps)
	fmt.Printf("  Stabilize Time: %s\n", cfg.Scaling.StabilizeTime)

	fmt.Printf("\nWorkload Configuration:\n")
	fmt.Printf("  Dataset: %s\n", cfg.Workload.Dataset)
	fmt.Printf("  Processor Type: %s\n", cfg.Workload.ProcessorType)
	fmt.Printf("  Batch Size: %d\n", cfg.Workload.BatchSize)

	fmt.Printf("\nInfrastructure:\n")
	fmt.Printf("  Namespace: %s\n", cfg.Infrastructure.Namespace)
	fmt.Printf("  Deployment: %s\n", cfg.Infrastructure.DeploymentName)
	fmt.Printf("  UC3 API: %s\n", cfg.Infrastructure.UC3APIBaseURL)

	fmt.Printf("\nMetrics:\n")
	fmt.Printf("  Collection Interval: %s\n", cfg.Metrics.CollectionInterval)
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

	if cfg.Duration <= 0 {
		fmt.Printf("❌ Invalid duration: %s\n", cfg.Duration)
		valid = false
	} else {
		fmt.Printf("✅ Duration: %s\n", cfg.Duration)
	}

	// Validate scaling configuration
	if cfg.Scaling.MinProcessors <= 0 {
		fmt.Printf("❌ Invalid min processors: %d\n", cfg.Scaling.MinProcessors)
		valid = false
	} else {
		fmt.Printf("✅ Min processors: %d\n", cfg.Scaling.MinProcessors)
	}

	if cfg.Scaling.MaxProcessors < cfg.Scaling.MinProcessors {
		fmt.Printf("❌ Max processors (%d) less than min processors (%d)\n",
			cfg.Scaling.MaxProcessors, cfg.Scaling.MinProcessors)
		valid = false
	} else {
		fmt.Printf("✅ Max processors: %d\n", cfg.Scaling.MaxProcessors)
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

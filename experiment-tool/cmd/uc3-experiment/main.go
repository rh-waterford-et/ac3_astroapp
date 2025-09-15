package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/uc3/experiment-tool/cmd/uc3-experiment/commands"
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

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./configs/defaults/experiment.yaml)")

	// Add basic commands that don't depend on config
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(statusCmd)
}

func main() {
	// Load config
	var err error
	cfg, err = config.LoadConfig(cfgFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Add commands that depend on config
	rootCmd.AddCommand(commands.NewConfigCmd(cfg))
	rootCmd.AddCommand(commands.NewScaleCmd(cfg))
	rootCmd.AddCommand(commands.NewDatasetCmd(cfg))
	rootCmd.AddCommand(commands.NewStartCmd(cfg))
	rootCmd.AddCommand(commands.NewMetricsCmd())

	// Execute
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

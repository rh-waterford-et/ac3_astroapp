package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/uc3/experiment-tool/pkg/config"
	"github.com/uc3/experiment-tool/pkg/scaler"
)

// NewScaleCmd creates the scale command and its subcommands
func NewScaleCmd(cfg *config.ExperimentConfig) *cobra.Command {
	scaleCmd := &cobra.Command{
		Use:   "scale",
		Short: "Kubernetes scaling operations",
		Long: `Scale the UC3 processor deployment up or down.

This command allows you to manually scale the processor deployment
for testing or to prepare for experiments.`,
	}

	scaleSetCmd := &cobra.Command{
		Use:   "set [replicas]",
		Short: "Set the number of processor replicas",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			scaleSet(cfg, args[0])
		},
	}

	scaleInfoCmd := &cobra.Command{
		Use:   "info",
		Short: "Show current deployment information",
		Run: func(cmd *cobra.Command, args []string) {
			scaleInfo(cfg)
		},
	}

	scaleTestCmd := &cobra.Command{
		Use:   "test",
		Short: "Test Kubernetes connectivity and permissions",
		Run: func(cmd *cobra.Command, args []string) {
			scaleTest(cfg)
		},
	}

	scaleCmd.AddCommand(scaleSetCmd)
	scaleCmd.AddCommand(scaleInfoCmd)
	scaleCmd.AddCommand(scaleTestCmd)

	return scaleCmd
}

func createScaler(cfg *config.ExperimentConfig) (scaler.Scaler, error) {
	scalerConfig := scaler.ScalerConfig{
		KubeConfig:     cfg.Infrastructure.KubeConfig,
		Namespace:      cfg.Infrastructure.Namespace,
		DeploymentName: cfg.Infrastructure.DeploymentName,
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

func scaleSet(cfg *config.ExperimentConfig, replicasStr string) {
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
	const maxSafeReplicas = 50
	if replicas > maxSafeReplicas {
		fmt.Fprintf(os.Stderr, "Error: replica count should not exceed %d for safety\n", maxSafeReplicas)
		os.Exit(1)
	}

	fmt.Printf("Scaling %s deployment to %d replicas...\n", cfg.Infrastructure.DeploymentName, replicas)

	k8sScaler, err := createScaler(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating Kubernetes scaler: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Get current scale before scaling
	currentReplicas, err := k8sScaler.GetCurrentScale(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current scale: %v\n", err)
		os.Exit(1)
	}

	// Scale deployment
	err = k8sScaler.Scale(ctx, replicas)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scaling deployment: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully scaled %s from %d to %d replicas\n",
		cfg.Infrastructure.DeploymentName, currentReplicas, replicas)
}

func scaleInfo(cfg *config.ExperimentConfig) {
	fmt.Printf("Getting deployment information for %s...\n", cfg.Infrastructure.DeploymentName)

	k8sScaler, err := createScaler(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating Kubernetes scaler: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	info, err := k8sScaler.GetDeploymentInfo(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting deployment info: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nDeployment Information:\n")
	fmt.Printf("=======================\n")
	fmt.Printf("Name: %s\n", info.Name)
	fmt.Printf("Namespace: %s\n", info.Namespace)
	fmt.Printf("Replicas: %d (desired) / %d (ready) / %d (available)\n", info.Replicas, info.Ready, info.Available)

	if len(info.Labels) > 0 {
		fmt.Printf("Labels:\n")
		for key, value := range info.Labels {
			fmt.Printf("  %s: %s\n", key, value)
		}
	}
}

func scaleTest(cfg *config.ExperimentConfig) {
	fmt.Printf("Testing Kubernetes connectivity and permissions...\n\n")

	fmt.Printf("1. Testing cluster connection...")
	k8sScaler, err := createScaler(cfg)
	if err != nil {
		fmt.Printf("\n")
		fmt.Printf("\nConnection failed: %v\n", err)
		fmt.Printf("\nTroubleshooting:\n")
		fmt.Printf("  • Ensure you're logged into the cluster: oc login\n")
		fmt.Printf("  • Check your kubeconfig: kubectl config current-context\n")
		fmt.Printf("  • Verify cluster access: oc whoami\n")
		os.Exit(1)
	}
	fmt.Printf("\n")

	fmt.Printf("2. Testing deployment access...")
	ctx := context.Background()
	info, err := k8sScaler.GetDeploymentInfo(ctx)
	if err != nil {
		fmt.Printf("\n")
		fmt.Printf("\nDeployment access failed: %v\n", err)
		fmt.Printf("\nTroubleshooting:\n")
		fmt.Printf("  • Check deployment exists: oc get deployment %s -n %s\n",
			cfg.Infrastructure.DeploymentName, cfg.Infrastructure.Namespace)
		fmt.Printf("  • Verify permissions: oc auth can-i get deployments -n %s\n", cfg.Infrastructure.Namespace)
		os.Exit(1)
	}
	fmt.Printf("\n")

	fmt.Printf("\nAll tests passed!\n")
	fmt.Printf("Connected to deployment: %s/%s (%d replicas)\n",
		info.Namespace, info.Name, info.Replicas)
	fmt.Printf("\nYou can now scale the deployment:\n")
	fmt.Printf("   Try: ./uc3-experiment scale set 2\n")
}

package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/uc3/experiment-tool/pkg/collector"
)

// NewMetricsCmd creates the metrics command and its subcommands
func NewMetricsCmd() *cobra.Command {
	metricsCmd := &cobra.Command{
		Use:   "metrics",
		Short: "Redis metrics collection and testing",
		Long:  "Commands for testing Redis connectivity and collecting UC3 metrics data",
	}

	metricsCmd.AddCommand(newMetricsTestRedisCmd())
	metricsCmd.AddCommand(newMetricsCollectCmd())

	return metricsCmd
}

func newMetricsTestRedisCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test-redis",
		Short: "Test Redis connectivity",
		Long:  "Verify connection to UC3's Redis instance using environment variables",
		RunE:  metricsTestRedis,
	}
}

func newMetricsCollectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "collect",
		Short: "Collect current metrics snapshot",
		Long:  "Capture current system state and active job metrics from Redis",
		RunE:  metricsCollect,
	}
}

func metricsTestRedis(cmd *cobra.Command, args []string) error {
	fmt.Println("🔍 Testing Redis connection...")

	redisCollector, err := collector.NewRedisCollector()
	if err != nil {
		return fmt.Errorf("❌ Failed to create Redis collector: %w", err)
	}
	defer redisCollector.Close()

	if err := redisCollector.TestConnection(); err != nil {
		return fmt.Errorf("❌ Redis connection test failed: %w", err)
	}

	fmt.Println("✅ Redis connection successful!")

	// Test basic functionality
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	activeBatches, err := redisCollector.GetActiveBatches(ctx)
	if err != nil {
		return fmt.Errorf("❌ Failed to get active batches: %w", err)
	}

	fmt.Printf("📊 Found %d active batches in Redis\n", len(activeBatches))
	if len(activeBatches) > 0 {
		fmt.Println("   Active batch IDs:")
		for i, batchID := range activeBatches {
			if i < 5 { // Show first 5
				fmt.Printf("   - %s\n", batchID)
			} else if i == 5 {
				fmt.Printf("   ... and %d more\n", len(activeBatches)-5)
				break
			}
		}
	}

	return nil
}

func metricsCollect(cmd *cobra.Command, args []string) error {
	fmt.Println("📊 Collecting current metrics snapshot...")

	redisCollector, err := collector.NewRedisCollector()
	if err != nil {
		return fmt.Errorf("❌ Failed to create Redis collector: %w", err)
	}
	defer redisCollector.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get system snapshot (assuming 1 processor for now)
	snapshot, err := redisCollector.GetSystemSnapshot(ctx, 1)
	if err != nil {
		return fmt.Errorf("❌ Failed to get system snapshot: %w", err)
	}

	// Calculate live metrics
	liveMetrics := redisCollector.CalculateLiveMetrics(snapshot, "manual-collection")

	// Display results
	fmt.Printf("\n📈 System Snapshot (%s):\n", snapshot.Timestamp.Format("15:04:05"))
	fmt.Printf("   Active Batches: %d\n", len(snapshot.ActiveBatches))
	fmt.Printf("   Total Jobs: %d\n", len(snapshot.JobMetrics))
	fmt.Printf("   Active Jobs: %d\n", liveMetrics.ActiveJobs)
	fmt.Printf("   Completed Jobs: %d\n", liveMetrics.CompletedJobs)
	fmt.Printf("   Avg Processing Time: %.2f seconds\n", liveMetrics.AvgProcessingTime)
	fmt.Printf("   Estimated Throughput: %.2f jobs/min\n", liveMetrics.Throughput)

	if len(snapshot.ActiveBatches) > 0 {
		fmt.Println("\n🔍 Sample Batch Details:")
		batchID := snapshot.ActiveBatches[0]

		batchMetrics, err := redisCollector.GetBatchSummary(ctx, batchID, 1, "manual-collection")
		if err != nil {
			fmt.Printf("   ⚠️  Could not get batch summary for %s: %v\n", batchID, err)
		} else {
			fmt.Printf("   Batch ID: %s\n", batchMetrics.BatchID)
			fmt.Printf("   Jobs: %d/%d complete\n", batchMetrics.CompleteJobCount, batchMetrics.JobCount)
			fmt.Printf("   Avg Queue Time: %v\n", batchMetrics.AvgQueueTime.Round(time.Millisecond))
			fmt.Printf("   Avg Processing Time: %v\n", batchMetrics.AvgProcessingTime.Round(time.Millisecond))
			fmt.Printf("   Total Size: %.2f MB\n", batchMetrics.TotalSizeMB)
		}
	}

	return nil
}

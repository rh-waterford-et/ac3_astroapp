package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/podmetrics"
)

func main() {
	log.Println("Starting Pod Metrics Exporter...")

	// Create pod metrics exporter
	exporter, err := podmetrics.NewPodMetricsExporter()
	if err != nil {
		log.Fatalf("Failed to create pod metrics exporter: %v", err)
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal, stopping...")
		cancel()
	}()

	// Start the exporter
	if err := exporter.Start(ctx); err != nil {
		log.Fatalf("Failed to start pod metrics exporter: %v", err)
	}
}

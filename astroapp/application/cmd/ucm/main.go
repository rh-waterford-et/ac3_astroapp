package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/common"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/metrics"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/queue"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/receiver"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/s3bucket"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/server"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/watcher"
)

const usage = `
	usage: 
	ucm watcher <producer|processor>  
	ucm consumer <producer|processor> 
	ucm server

	command line examples:

	# execute producer watcher
	ucm watcher producer

	# execute processor receiver
	ucm consumer processor

	# start HTTP server for file uploads
	ucm server

	# execute aggregator
	ucm aggregator
`

func main() {

	if len(os.Args) < 2 {
		fmt.Println(usage)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "watcher":
		if len(os.Args) < 3 {
			fmt.Println(usage)
			os.Exit(1)
		}
		err := LaunchProducer(os.Args[2])
		if err != nil {
			os.Exit(1)
		}
	case "consumer":
		if len(os.Args) < 3 {
			fmt.Println(usage)
			os.Exit(1)
		}
		err := LaunchReceiver(os.Args[2])
		if err != nil {
			os.Exit(1)
		}
	case "server":
		LaunchServer()
	case "aggregator":
		LaunchAggregator()
	default:
		fmt.Println(usage)
		os.Exit(1)
	}
}

func LaunchServer() {
	log.Printf("------------------ Starting HTTP Server ---------------------")

	server := server.NewServer()
	server.Start()
}

func LaunchReceiver(side string) error {
	log.Printf(side)
	log.Printf("------------------ Starting Receiver() ---------------------")
	// Initialize directories first
	utils := &common.Utils{}

	if side == "processor" {
		if err := utils.EnsureDirectoriesExist(); err != nil {
			log.Fatalf("Directory initialization failed: %v", err)
		}
	}
	// Create RabbitMQ connection
	queue, err := queue.NewRabbitMQConnection()
	if err != nil {
		log.Fatalf("Failed to create RabbitMQ connection: %v", err)
	}
	s3bucket := s3bucket.NewS3Bucket()

	// Initialize Redis client
	var redisClient *metrics.RedisClient
	if os.Getenv("REDIS_HOST") != "" {
		redisClient, err = metrics.NewRedisConnection()
		if err != nil {
			log.Printf("Failed to connect to Redis: %v", err)
			log.Printf("Continuing without Redis metrics tracking")
		} else {
			log.Printf("Connected to Redis for metrics tracking")
		}
	}

	receiver := receiver.NewReceiver(queue, utils, s3bucket, redisClient)
	receiver.Start(side)

	return nil
}

func LaunchProducer(side string) error {
	log.Printf(side)
	utils := &common.Utils{}

	log.Printf("------------------ Starting Watcher() ---------------------")

	queue, err := queue.NewRabbitMQConnection()
	if err != nil {
		log.Fatalf("Failed to create RabbitMQ connection: %v", err)
	}

	if side == "producer" {
		bucket := s3bucket.NewS3Bucket()
		err := utils.EnsureBucketDirectoriesExist(bucket)
		if err != nil {
			log.Fatalf("Failed to ensure bucket directories: %v", err)
		}

	}
	// Initialize Redis client for producer if needed
	var redis *metrics.RedisClient
	if os.Getenv("REDIS_HOST") != "" {
		redis, err = metrics.NewRedisConnection()
		if err != nil {
			log.Printf("Failed to connect to Redis: %v", err)
			log.Printf("Continuing without Redis metrics tracking")
		} else {
			log.Printf("Connected to Redis for metrics tracking")
		}
	}
	appRunner := &watcher.Watcher{}
	if side == "processor" {
		if err := utils.EnsureDirectoriesExist(); err != nil {
			log.Fatalf("Directory initialization failed: %v", err)
		}
		appRunner.RunProcessor(side, utils, queue, redis)
	} else {
		// Producer side starts HTTP server to receive processing triggers
		log.Println("Producer watcher starting HTTP trigger server on :8081...")

		http.HandleFunc("/trigger-processing", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			var triggerData struct {
				Dataset   string `json:"dataset"`
				Processor string `json:"processor"`
			}

			if err := json.NewDecoder(r.Body).Decode(&triggerData); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}

			jobName := triggerData.Dataset
			processorType := triggerData.Processor
			appType := strings.ToUpper(processorType)

			log.Printf("HTTP trigger received: job=%s, processor=%s", jobName, processorType)

			// Run the watcher for this specific job
			go func() {
				appRunner.RunProducer(appType, jobName, side, utils, queue, redis)
				log.Printf("Completed processing for job: %s", jobName)
			}()

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "triggered"})
		})

		http.HandleFunc("/trigger-single-file", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			var triggerData struct {
				Dataset   string `json:"dataset"`
				FileName  string `json:"fileName"`
				Processor string `json:"processor"`
			}

			if err := json.NewDecoder(r.Body).Decode(&triggerData); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}

			jobName := triggerData.Dataset
			fileName := triggerData.FileName
			processorType := triggerData.Processor
			appType := strings.ToUpper(processorType)

			log.Printf("HTTP single file trigger received: job=%s, file=%s, processor=%s",
				jobName, fileName, processorType)

			// Run the watcher for this specific file
			go func() {
				appRunner.RunForSingleFile(appType, jobName, fileName, side, utils, queue, redis)
				log.Printf("Completed processing for file: %s in job: %s", fileName, jobName)
			}()

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "triggered"})
		})

		log.Fatal(http.ListenAndServe(":8081", nil))
	}

	return nil
}

func LaunchAggregator() {
	log.Printf("------------------ Starting Aggregator() ---------------------")
	redisClient, err := metrics.NewRedisConnection()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	metricsStore := metrics.NewMetricsStore(redisClient, 168*time.Hour)
	aggregationService := metrics.NewAggregationService(metricsStore, 5*time.Minute)

	// Create RabbitMQ connection for queue length monitoring
	rabbitMQ, err := queue.NewRabbitMQConnection()
	if err != nil {
		log.Printf("Warning: Failed to connect to RabbitMQ for queue monitoring: %v", err)
		log.Printf("Queue length monitoring will be disabled")
	} else {
		aggregationService.SetQueue(rabbitMQ)
		log.Printf("✓ RabbitMQ connection established for queue length monitoring")
	}

	log.Printf("🔄 Starting Prometheus /metrics endpoint")
	go func() {
		if err := metrics.StartMetricsServer(":9090", metricsStore); err != nil {
			log.Fatalf("Metrics server failed: %v", err)
		}
	}()
	time.Sleep(2 * time.Second) // Give server time to start and register metrics
	log.Printf("🔄 Started Prometheus /metrics endpoint on :9090")

	// Start aggregation service in background (only on processor side to avoid duplication)
	if aggregationService != nil {
		ctx := context.Background()

		// Start queue length monitor in a separate goroutine (polls every 10 seconds)
		go func() {
			// Small delay to ensure metrics server is fully ready
			time.Sleep(1 * time.Second)
			aggregationService.RunQueueLengthMonitor(ctx)
		}()
		log.Printf("🔄 Started queue length monitor (10-second intervals)")

		go func() {
			time.Sleep(30 * time.Second)
			// Run aggregation every 5 minutes
			aggregationService.Run(ctx)
		}()
		log.Printf("🔄 Started batch metrics aggregation service (5-minute intervals)")
		for {
			time.Sleep(1 * time.Hour)
		}
	}
}

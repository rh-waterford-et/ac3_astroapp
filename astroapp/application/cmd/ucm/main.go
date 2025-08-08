package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

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

	if err := utils.EnsureDirectoriesExist(); err != nil {
		log.Fatalf("Directory initialization failed: %v", err)
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

			batchName := triggerData.Dataset
			processorType := triggerData.Processor
			appType := strings.ToUpper(processorType)

			log.Printf("HTTP trigger received: batch=%s, processor=%s", batchName, processorType)

			// Run the watcher for this specific batch
			go func() {
				appRunner.RunForBatch(appType, batchName, side, utils, queue, redis)
				log.Printf("Completed processing for batch: %s", batchName)
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

			batchName := triggerData.Dataset
			fileName := triggerData.FileName
			processorType := triggerData.Processor
			appType := strings.ToUpper(processorType)

			log.Printf("HTTP single file trigger received: batch=%s, file=%s, processor=%s",
				batchName, fileName, processorType)

			// Run the watcher for this specific file
			go func() {
				appRunner.RunForSingleFile(appType, batchName, fileName, side, utils, queue, redis)
				log.Printf("Completed processing for file: %s in batch: %s", fileName, batchName)
			}()

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "triggered"})
		})

		log.Fatal(http.ListenAndServe(":8081", nil))
	}

	return nil
}

package main

import (
	"fmt"
	"log"
	"os"
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

	apps := []string{"PPXF", "STARLIGHT", "STECKMAP"}

	appRunner := &watcher.Watcher{}
	for {
		for _, app := range apps {
			appRunner.Run(app, side, utils, queue)
		}
		log.Println("Checking for new files...")
		time.Sleep(10 * time.Second)
	}
}

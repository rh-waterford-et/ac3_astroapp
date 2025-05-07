package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/rh-waterford-et/ac3_astroapp/pkg/common"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/queue"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/receiver"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/s3bucket"
	"github.com/rh-waterford-et/ac3_astroapp/pkg/watcher"
)

const usage = `
	usage: 
	ucm watcher <producer|processor>  
	ucm consumer <producer|processor> 

	command line examples:

	# execute producer watcher
	ucm watcher producer

	# execute processor receiver
	ucm consumer processor
`

func main() {

	if len(os.Args) <= 2 {
		fmt.Println(usage)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "watcher":
		err := LaunchProducer(os.Args[2])
		if err != nil {
			os.Exit(1)
		}
	case "consumer":
		err := LaunchReceiver(os.Args[2])
		if err != nil {
			os.Exit(1)
		}
	}
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
	receiver := receiver.NewReceiver(queue, utils, s3bucket)
	receiver.Start(side)

	return nil
}

func LaunchProducer(side string) error {
	log.Printf(side)
	utils := &common.Utils{}

	log.Printf("------------------ Starting Watcher() ---------------------")
	if err := utils.EnsureDirectoriesExist(); err != nil {
		log.Fatalf("Directory initialization failed: %v", err)
	}

	apps := []string{"PPFX", "STARLIGHT", "STECKMAP"}

	appRunner := &watcher.Watcher{}
	for {
		for _, app := range apps {
			appRunner.Run(app, side, utils)
		}
		log.Println("Checking for new files...")
		time.Sleep(10 * time.Second)
	}
}

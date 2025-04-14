package common

import (
	"fmt"
	"log"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type QueueHandler struct {
	conn      *amqp.Connection
	channel   *amqp.Channel
	consumers map[string]ConsumerInterface
}

func NewQueueHandler() (*QueueHandler, error) {
	username := os.Getenv("RABBITMQ_USER")
	password := os.Getenv("RABBITMQ_PASSWORD")
	host := os.Getenv("RABBITMQ_HOST")
	port := os.Getenv("RABBITMQ_PORT")
	url := fmt.Sprintf("amqp://%s:%s@%s:%s/", username, password, host, port)

	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %v", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %v", err)
	}

	return &QueueHandler{
		conn:      conn,
		channel:   ch,
		consumers: make(map[string]ConsumerInterface),
	}, nil
}

func (q *QueueHandler) Close() {
	if q.channel != nil {
		q.channel.Close()
	}
	if q.conn != nil {
		q.conn.Close()
	}
}

func (q *QueueHandler) AddConsumer(queueName string, consumer ConsumerInterface) error {
	_, err := q.channel.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %v", err)
	}

	q.consumers[queueName] = consumer
	return nil
}

func (q *QueueHandler) StartConsuming() error {
	if err := q.channel.Qos(1, 0, false); err != nil {
		return fmt.Errorf("failed to set QoS: %v", err)
	}

	for queueName, consumer := range q.consumers {
		go q.consumeQueue(queueName, consumer)
	}

	return nil
}

func (q *QueueHandler) consumeQueue(queueName string, consumer ConsumerInterface) {
	for {
		// Generate a unique consumer tag
		consumerTag := fmt.Sprintf("consumer-%s-%d", queueName, time.Now().UnixNano())

		msgs, err := q.channel.Consume(
			queueName,   // queue
			consumerTag, // consumer
			false,       // auto-ack
			false,       // exclusive
			false,       // no-local
			false,       // no-wait
			nil,         // args
		)
		if err != nil {
			log.Printf("Failed to consume from queue %s: %v", queueName, err)
			time.Sleep(5 * time.Second)
			continue
		}

		log.Printf("Started consuming from queue %s", queueName)

		for d := range msgs {
			if err := consumer.ProcessMessage(d); err != nil {
				log.Printf("Error processing message: %v", err)
				d.Nack(false, true) // Requeue on failure
			} else {
				d.Ack(false)
			}
		}

		log.Printf("Stopped consuming from queue %s, reconnecting...", queueName)
		time.Sleep(1 * time.Second)
	}
}

package common

import (
	"context"
	"fmt"
	"log"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
)

type QueueInterface interface {
	Connect() error
	Close() error
	DeclareQueue(name string) error
	Publish(ctx context.Context, queueName string, body []byte, headers amqp.Table) error
	Consume(queueName string, consumerTag string) (<-chan amqp.Delivery, error)
	CancelConsumer(consumerTag string) error
	InspectQueue(name string) (amqp.Queue, error)
	SetQoS(prefetchCount int) error
}

type Queues struct {
	conn *amqp.Connection
	ch   *amqp.Channel
	url  string
}

func NewRabbitMQConnection() (QueueInterface, error) {
	username := os.Getenv("RABBITMQ_USER")
	password := os.Getenv("RABBITMQ_PASSWORD")
	host := os.Getenv("RABBITMQ_HOST")
	port := os.Getenv("RABBITMQ_PORT")
	url := fmt.Sprintf("amqp://%s:%s@%s:%s/", username, password, host, port)

	q := &Queues{url: url}
	log.Printf("Connecting to RabbitMQ at %s", url)
	err := q.Connect()
	if err != nil {
		return nil, err
	}
	return q, nil
}

func (q *Queues) Connect() error {
	var err error
	q.conn, err = amqp.Dial(q.url)
	if err != nil {
		return fmt.Errorf("connect error: %w", err)
	}

	q.ch, err = q.conn.Channel()
	if err != nil {
		return fmt.Errorf("channel error: %w", err)
	}
	return nil
}

func (q *Queues) Close() error {
	if err := q.ch.Close(); err != nil {
		return err
	}
	return q.conn.Close()
}

func (q *Queues) DeclareQueue(name string) error {
	_, err := q.ch.QueueDeclare(
		name,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,
	)
	return err
}

func (q *Queues) Publish(ctx context.Context, queueName string, body []byte, headers amqp.Table) error {
	return q.ch.PublishWithContext(ctx,
		"",        // exchange
		queueName, // routing key
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
			Headers:     headers,
		})
}

func (q *Queues) Consume(queueName string, consumerTag string) (<-chan amqp.Delivery, error) {
	return q.ch.Consume(
		queueName,
		consumerTag,
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
}

func (q *Queues) CancelConsumer(consumerTag string) error {
	return q.ch.Cancel(consumerTag, false)
}

func (q *Queues) InspectQueue(name string) (amqp.Queue, error) {
	return q.ch.QueueInspect(name)
}

func (q *Queues) SetQoS(prefetchCount int) error {
	return q.ch.Qos(prefetchCount, 0, false)
}

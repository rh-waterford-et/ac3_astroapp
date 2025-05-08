package queue

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

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

func NewRabbitMQConnection() (*Queues, error) {
	// TODO: Consider some fallback - maybe use parameters
	// and then check envars
	username := os.Getenv("RABBITMQ_USER")
	password := os.Getenv("RABBITMQ_PASSWORD")
	host := os.Getenv("RABBITMQ_HOST")
	port := os.Getenv("RABBITMQ_PORT")
	hostPort := net.JoinHostPort(host, port)
	url := fmt.Sprintf("amqp://%s:%s@%s/", username, password, hostPort)

	q := &Queues{url: url}
	log.Printf("Connecting to RabbitMQ at %s", url)
	err := q.Connect()
	if err != nil {
		return &Queues{}, fmt.Errorf("%w", err)
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
		return fmt.Errorf("%w", err)
	}
	err := q.conn.Close()
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
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
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

func (q *Queues) Publish(ctx context.Context, queueName string, body []byte, headers amqp.Table) error {
	err := q.ch.PublishWithContext(ctx,
		"",        // exchange
		queueName, // routing key
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
			Headers:     headers,
			Timestamp:   time.Now(),
		})
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

func (q *Queues) Consume(queueName string, consumerTag string) (<-chan amqp.Delivery, error) {
	ch, err := q.ch.Consume(
		queueName,
		consumerTag,
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
	if err != nil {
		return ch, fmt.Errorf("%w", err)
	}
	return ch, nil
}

func (q *Queues) CancelConsumer(consumerTag string) error {
	return q.ch.Cancel(consumerTag, false)
}

func (q *Queues) InspectQueue(name string) (amqp.Queue, error) {
	queue, err := q.ch.QueueDeclarePassive(
		name,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,
	)

	if err != nil {
		return amqp.Queue{}, fmt.Errorf("%w", err)
	}
	return queue, nil

}

func (q *Queues) SetQoS(prefetchCount int) error {
	err := q.ch.Qos(prefetchCount, 0, false)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

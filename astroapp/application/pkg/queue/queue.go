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
	GetOne(queueName string) (amqp.Delivery, bool, error)
}

type Queues struct {
	conn           *amqp.Connection
	ch             *amqp.Channel
	url            string
	reconnecting   bool
	lastReconnect  time.Time
	reconnectDelay time.Duration
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

	q := &Queues{
		url:            url,
		reconnectDelay: 2 * time.Second,
	}
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

	q.lastReconnect = time.Now()
	log.Printf("✓ RabbitMQ connection established")
	return nil
}

// isConnected checks if connection and channel are healthy
func (q *Queues) isConnected() bool {
	return q.conn != nil && !q.conn.IsClosed() && q.ch != nil
}

// ensureConnection checks connection health and reconnects if needed
func (q *Queues) ensureConnection() error {
	if q.isConnected() {
		return nil
	}

	// Prevent concurrent reconnection attempts
	if q.reconnecting {
		return fmt.Errorf("reconnection already in progress")
	}

	// Rate limit reconnection attempts
	if time.Since(q.lastReconnect) < q.reconnectDelay {
		return fmt.Errorf("reconnection rate limited, last attempt was %v ago", time.Since(q.lastReconnect))
	}

	q.reconnecting = true
	defer func() { q.reconnecting = false }()

	log.Printf("⚠ RabbitMQ connection lost, attempting to reconnect...")

	// Close existing connections if any
	if q.ch != nil {
		q.ch.Close()
	}
	if q.conn != nil {
		q.conn.Close()
	}

	// Attempt reconnection with retries
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := q.Connect()
		if err == nil {
			log.Printf("✓ RabbitMQ reconnection successful after %d attempt(s)", attempt)
			return nil
		}

		if attempt < maxRetries {
			waitTime := time.Duration(attempt) * q.reconnectDelay
			log.Printf("⚠ Reconnection attempt %d/%d failed: %v. Retrying in %v...", attempt, maxRetries, err, waitTime)
			time.Sleep(waitTime)
		}
	}

	return fmt.Errorf("failed to reconnect to RabbitMQ after %d attempts", maxRetries)
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
	if err := q.ensureConnection(); err != nil {
		return fmt.Errorf("connection check failed: %w", err)
	}

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
	if err := q.ensureConnection(); err != nil {
		return fmt.Errorf("connection check failed: %w", err)
	}

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
	if err := q.ensureConnection(); err != nil {
		return nil, fmt.Errorf("connection check failed: %w", err)
	}

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
	if err := q.ensureConnection(); err != nil {
		return amqp.Queue{}, fmt.Errorf("connection check failed: %w", err)
	}

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
	if err := q.ensureConnection(); err != nil {
		return fmt.Errorf("connection check failed: %w", err)
	}

	err := q.ch.Qos(prefetchCount, 0, false)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

func (q *Queues) GetOne(queueName string) (amqp.Delivery, bool, error) {
	if err := q.ensureConnection(); err != nil {
		return amqp.Delivery{}, false, fmt.Errorf("connection check failed: %w", err)
	}

	msg, ok, err := q.ch.Get(
		queueName,
		false, // autoAck = false
	)
	return msg, ok, err
}

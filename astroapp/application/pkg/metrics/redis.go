package metrics

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	client *redis.Client
	url    string
}

func NewRedisConnection() (*RedisClient, error) {
	host := os.Getenv("REDIS_HOST")
	port := os.Getenv("REDIS_PORT")
	password := os.Getenv("REDIS_PASSWORD")
	hostPort := net.JoinHostPort(host, port)

	rc := &RedisClient{
		url: hostPort,
		client: redis.NewClient(&redis.Options{
			Addr:     hostPort,
			Password: password,
			DB:       0, // default DB
		}),
	}

	log.Printf("Connecting to Redis at %s", hostPort)
	err := rc.Connect()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}
	return rc, nil
}

func (rc *RedisClient) Connect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := rc.client.Ping(ctx).Result()
	return err
}

func (rc *RedisClient) Close() error {
	return rc.client.Close()
}

func (rc *RedisClient) Ping(ctx context.Context) error {
	_, err := rc.client.Ping(ctx).Result()
	return err
}

func (rc *RedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return rc.client.Set(ctx, key, value, expiration).Err()
}

func (rc *RedisClient) Get(ctx context.Context, key string) (string, error) {
	return rc.client.Get(ctx, key).Result()
}

func (rc *RedisClient) HSet(ctx context.Context, key string, values ...interface{}) error {
	return rc.client.HSet(ctx, key, values...).Err()
}

func (rc *RedisClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return rc.client.HGetAll(ctx, key).Result()
}

func (rc *RedisClient) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return rc.client.Expire(ctx, key, expiration).Err()
}

func (rc *RedisClient) Pipeline() redis.Pipeliner {
	return rc.client.Pipeline()
}

func (rc *RedisClient) Keys(ctx context.Context, pattern string) ([]string, error) {
	return rc.client.Keys(ctx, pattern).Result()
}

func (rc *RedisClient) Exists(ctx context.Context, key string) (int64, error) {
	return rc.client.Exists(ctx, key).Result()
}

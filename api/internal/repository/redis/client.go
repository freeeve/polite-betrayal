package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Client wraps the Redis client for game state operations.
type Client struct {
	rdb *redis.Client
}

// NewClient creates a Redis client from a connection URL.
func NewClient(redisURL string) (*Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}
	rdb := redis.NewClient(opts)
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return &Client{rdb: rdb}, nil
}

// NewClientFromPool wraps an existing redis.Client for use in tests.
func NewClientFromPool(rdb *redis.Client) *Client {
	return &Client{rdb: rdb}
}

// Close closes the Redis connection.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Underlying returns the raw redis client for keyspace notifications.
func (c *Client) Underlying() *redis.Client {
	return c.rdb
}

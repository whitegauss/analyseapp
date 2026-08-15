package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache is the real Cache implementation, backed by Redis.
type RedisCache struct {
	Client *redis.Client
}

func (c *RedisCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	val, err := c.Client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return val, true, nil
}

func (c *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.Client.Set(ctx, key, value, ttl).Err()
}

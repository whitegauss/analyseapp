package cache

import (
	"context"
	"errors"
	"strings"
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

// DeleteByPrefix implements Cache. The prefix is matched literally: it is
// escaped before being appended to the SCAN pattern, so a metacharacter in it
// names itself rather than a character class. An empty prefix is rejected —
// as a pattern it would be MATCH "*", which silently empties the database.
func (c *RedisCache) DeleteByPrefix(ctx context.Context, prefix string) error {
	if prefix == "" {
		return ErrEmptyPrefix
	}
	var keys []string
	iter := c.Client.Scan(ctx, 0, quoteGlob(prefix)+"*", 0).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	return c.Client.Del(ctx, keys...).Err()
}

// globMeta are the characters Redis's pattern matcher reads as syntax rather
// than as themselves.
const globMeta = `*?[]\`

// quoteGlob escapes every glob metacharacter in s so that Redis matches it
// literally.
func quoteGlob(s string) string {
	if !strings.ContainsAny(s, globMeta) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	for _, r := range s {
		if strings.ContainsRune(globMeta, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

package rediskey

import (
	"context"
	"errors"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4/pgxpool"
)

// cachedCount returns the value cached under key, or runs query against
// Postgres and caches the result for ttl when the key is missing. A failure to
// write the cache is not an error; the count itself is still valid.
func cachedCount(ctx context.Context, client *redis.Client, pool *pgxpool.Pool, key, query string, ttl time.Duration) (int64, error) {
	value, err := client.Get(ctx, key).Int64()
	if err == nil {
		return value, nil
	}
	if !errors.Is(err, redis.Nil) {
		return 0, err
	}
	if err = pool.QueryRow(ctx, query).Scan(&value); err != nil {
		return 0, err
	}
	client.Set(ctx, key, value, ttl)
	return value, nil
}

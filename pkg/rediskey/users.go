package rediskey

import (
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4/pgxpool"
	"time"
)

const TotalUsersExpiration = time.Minute * 5

// CountTotalUsers is shared by the bot's /info command and the HTTP API so both
// report the same cached figure.
func CountTotalUsers(ctx context.Context, client *redis.Client, pool *pgxpool.Pool) (int64, error) {
	return cachedCount(ctx, client, pool, TotalUsers, "SELECT COUNT(*) FROM users", TotalUsersExpiration)
}

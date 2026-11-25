package rediskey

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
)

const TotalGuildsExpiration = time.Minute * 5

// CountGuilds returns the number of guilds the bot has ever been added to, including those it has since left or been
// removed from. The bot's /info command and the HTTP API share the cached figure.
func CountGuilds(ctx context.Context, client *redis.Client, pool *pgxpool.Pool) (int64, error) {
	return cachedCount(ctx, client, pool, TotalGuilds, "SELECT COUNT(*) FROM guilds", TotalGuildsExpiration)
}

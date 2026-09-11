package rediskey

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4/pgxpool"
	"log"
	"time"
)

const TotalGameExpiration = time.Minute * 5

// ActiveGameTimeoutSeconds is shared by bot liveness tracking and API counts.
const ActiveGameTimeoutSeconds = 900

// CountTotalGames is shared by the bot's /info command and the HTTP API so both
// report the same cached figure.
func CountTotalGames(ctx context.Context, client *redis.Client, pool *pgxpool.Pool) (int64, error) {
	return cachedCount(ctx, client, pool, TotalGames, "SELECT COUNT(*) FROM games WHERE start_time != -1 AND end_time != -1", TotalGameExpiration)
}

// CountActiveGames counts games that reported activity within the last secs seconds.
func CountActiveGames(ctx context.Context, client *redis.Client, secs int64) (int64, error) {
	now := time.Now()
	before := now.Add(-(time.Second * time.Duration(secs)))
	return client.ZCount(ctx, ActiveGamesZSet, fmt.Sprintf("%d", before.Unix()), fmt.Sprintf("%d", now.Unix())).Result()
}

func GetActiveGames(ctx context.Context, client *redis.Client, secs int64) int64 {
	count, err := CountActiveGames(ctx, client, secs)
	if err != nil {
		log.Println(err)
		return 0
	}
	return count
}

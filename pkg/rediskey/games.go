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

func GetTotalGames(ctx context.Context, client *redis.Client) int64 {
	v, err := client.Get(ctx, TotalGames).Int64()
	if err == nil {
		return v
	}
	return NotFound
}

func GetActiveGames(ctx context.Context, client *redis.Client, secs int64) int64 {
	now := time.Now()
	before := now.Add(-(time.Second * time.Duration(secs)))
	count, err := client.ZCount(ctx, ActiveGamesZSet, fmt.Sprintf("%d", before.Unix()), fmt.Sprintf("%d", now.Unix())).Result()
	if err != nil {
		log.Println(err)
		return 0
	}
	return count
}

func RefreshTotalGames(ctx context.Context, client *redis.Client, pool *pgxpool.Pool) int64 {
	v := queryTotalGames(ctx, pool)
	if v != NotFound {
		err := client.Set(ctx, TotalGames, v, TotalGameExpiration).Err()
		if err != nil {
			log.Println(err)
		}
	}
	return v
}

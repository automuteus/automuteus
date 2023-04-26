package redis

import (
	"context"
	"fmt"
	"github.com/automuteus/automuteus/v8/pkg/storage"
	"log"
	"time"
)

const (
	TotalGameExpiration      = time.Minute * 5
	NewGameRateLimitDuration = 3000 * time.Millisecond
)

func (redisDriver Driver) GetTotalGames(ctx context.Context) int64 {
	v, err := redisDriver.client.Get(ctx, TotalGames).Int64()
	if err == nil {
		return v
	}
	return NotFound
}

func (redisDriver Driver) GetActiveGames(ctx context.Context, secs int64) int64 {
	now := time.Now()
	before := now.Add(-(time.Second * time.Duration(secs)))
	count, err := redisDriver.client.ZCount(ctx, ActiveGamesZSet, fmt.Sprintf("%d", before.Unix()), fmt.Sprintf("%d", now.Unix())).Result()
	if err != nil {
		log.Println(err)
		return 0
	}
	return count
}

func (redisDriver Driver) RefreshTotalGames(ctx context.Context, psql storage.PsqlInterface) int64 {
	v := psql.QueryTotalGames(ctx)
	if v != NotFound {
		err := redisDriver.client.Set(ctx, TotalGames, v, TotalGameExpiration).Err()
		if err != nil {
			log.Println(err)
		}
	}
	return v
}

package redis

import (
	"context"
	"log"
)

func (redisDriver *Driver) GetGuildCounter(ctx context.Context) int64 {
	count, err := redisDriver.client.SCard(ctx, TotalGuildsSet).Result()
	if err != nil {
		log.Println(err)
		return 0
	}
	return count
}

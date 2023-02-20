package rediskey

import (
	"context"
	"github.com/go-redis/redis/v8"
	"log"
)

func GetGuildCounter(ctx context.Context, client *redis.Client) int64 {
	count, err := client.SCard(ctx, TotalGuildsSet).Result()
	if err != nil {
		log.Println(err)
		return 0
	}
	return count
}

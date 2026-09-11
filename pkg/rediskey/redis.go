package rediskey

import (
	"context"
	"github.com/go-redis/redis/v8"
	"log"
)

// CountGuilds returns the number of guilds the bot has ever been added to.
func CountGuilds(ctx context.Context, client *redis.Client) (int64, error) {
	return client.SCard(ctx, TotalGuildsSet).Result()
}

func GetGuildCounter(ctx context.Context, client *redis.Client) int64 {
	count, err := CountGuilds(ctx, client)
	if err != nil {
		log.Println(err)
		return 0
	}
	return count
}

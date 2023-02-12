package rediskey

import (
	"context"
	"github.com/go-redis/redis/v8"
	"log"
)

func GetVersionAndCommit(ctx context.Context, client *redis.Client) (string, string) {
	v, err := client.Get(ctx, Version).Result()
	if err != nil {
		log.Println(err)
	}
	c, err := client.Get(ctx, Commit).Result()
	if err != nil {
		log.Println(err)
	}
	return v, c
}

func SetVersionAndCommit(ctx context.Context, client *redis.Client, version, commit string) {
	err := client.Set(ctx, Version, version, 0).Err()
	if err != nil {
		log.Println(err)
	}

	err = client.Set(ctx, Commit, commit, 0).Err()
	if err != nil {
		log.Println(err)
	}
}

func GetGuildCounter(ctx context.Context, client *redis.Client) int64 {
	count, err := client.SCard(ctx, TotalGuildsSet).Result()
	if err != nil {
		log.Println(err)
		return 0
	}
	return count
}

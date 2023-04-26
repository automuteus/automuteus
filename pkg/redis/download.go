package redis

import (
	"context"
	"github.com/go-redis/redis/v8"
	"log"
	"time"
)

const GuildDownloadCooldown = 24 * time.Hour

func GuildDownloadCategoryCooldownKey(guildID, category string) string {
	return "automuteus:ratelimit:download:guild:" + guildID + ":category:" + category
}

func (redisDriver *Driver) MarkDownloadCategoryCooldown(guildID, category string) {
	err := redisDriver.client.Set(context.Background(), GuildDownloadCategoryCooldownKey(guildID, category), "", GuildDownloadCooldown).Err()
	if err != nil {
		log.Println(err)
	}
}

func (redisDriver *Driver) GetDownloadCategoryCooldown(guildID, category string) (time.Duration, error) {
	v, err := redisDriver.client.TTL(context.Background(), GuildDownloadCategoryCooldownKey(guildID, category)).Result()
	if err == redis.Nil {
		return 0, nil
	} else if err != nil {
		log.Println(err)
		return -1, err
	}
	return v, nil
}

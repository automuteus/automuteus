package rediskey

import (
	"context"
	"errors"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4/pgxpool"
	"log"
	"time"
)

const TotalUsersExpiration = time.Minute * 5

// CountTotalUsers is shared by the bot's /info command and the HTTP API so both
// report the same cached figure.
func CountTotalUsers(ctx context.Context, client *redis.Client, pool *pgxpool.Pool) (int64, error) {
	return cachedCount(ctx, client, pool, TotalUsers, "SELECT COUNT(*) FROM users", TotalUsersExpiration)
}

func GetCachedUserInfo(ctx context.Context, client *redis.Client, userID, guildID string) string {
	user, err := client.Get(ctx, CachedUserInfoOnGuild(userID, guildID)).Result()
	if errors.Is(err, redis.Nil) {
		return ""
	}
	if err != nil {
		log.Println(err)
		return ""
	}
	return user
}

const CachedUserDataExpiration = time.Hour * 12

func SetCachedUserInfo(ctx context.Context, client *redis.Client, userID, guildID, userData string) error {
	return client.Set(ctx, CachedUserInfoOnGuild(userID, guildID), userData, CachedUserDataExpiration).Err()
}

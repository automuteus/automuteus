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

const NotFound = -1

func GetTotalUsers(ctx context.Context, client *redis.Client) int64 {
	v, err := client.Get(ctx, TotalUsers).Int64()
	if err == nil {
		return v
	}
	return NotFound
}

func RefreshTotalUsers(ctx context.Context, client *redis.Client, pool *pgxpool.Pool) int64 {
	v := queryTotalUsers(ctx, pool)
	if v != NotFound {
		err := client.Set(ctx, TotalUsers, v, TotalUsersExpiration).Err()
		if err != nil {
			log.Println(err)
		}
	}
	return v
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

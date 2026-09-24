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

// GetCachedUserInfos fetches the cached info of many users of one guild in a single round trip. Users with no
// cached record are absent from the result; a Redis failure is returned so the caller can decide whether names
// are optional.
func GetCachedUserInfos(ctx context.Context, client *redis.Client, guildID string, userIDs []string) (map[string]string, error) {
	infos := make(map[string]string, len(userIDs))
	if len(userIDs) == 0 {
		return infos, nil
	}
	keys := make([]string, len(userIDs))
	for i, id := range userIDs {
		keys[i] = CachedUserInfoOnGuild(id, guildID)
	}
	values, err := client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	for i, v := range values {
		if s, ok := v.(string); ok && s != "" {
			infos[userIDs[i]] = s
		}
	}
	return infos, nil
}

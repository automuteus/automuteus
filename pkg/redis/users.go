package redis

import (
	"context"
	"errors"
	"fmt"
	"github.com/automuteus/automuteus/v8/pkg/storage"
	"github.com/go-redis/redis/v8"
	"log"
	"time"
)

const TotalUsersExpiration = time.Minute * 5

const NotFound = -1

const GlobalUserRateLimitDuration = 1 * time.Second

// when a user exceeds the threshold, they're ignored for this long
const SoftbanDuration = 5 * time.Minute

// how many violations before a softban
const SoftbanThreshold = 3

// how far back the bot should look for violations. Softban is invoked by violations>threshold in this amt of time
const SoftbanExpiration = 10 * time.Minute

const CachedUserDataExpiration = time.Hour * 12

func (redisDriver *Driver) GetTotalUsers(ctx context.Context) int64 {
	v, err := redisDriver.client.Get(ctx, TotalUsers).Int64()
	if err == nil {
		return v
	}
	return NotFound
}

func (redisDriver *Driver) RefreshTotalUsers(ctx context.Context, psql storage.PsqlInterface) int64 {
	v := psql.QueryTotalUsers(ctx)
	if v != NotFound {
		err := redisDriver.client.Set(ctx, TotalUsers, v, TotalUsersExpiration).Err()
		if err != nil {
			log.Println(err)
		}
	}
	return v
}

func (redisDriver *Driver) GetCachedUserInfo(ctx context.Context, userID, guildID string) string {
	user, err := redisDriver.client.Get(ctx, CachedUserInfoOnGuild(userID, guildID)).Result()
	if errors.Is(err, redis.Nil) {
		return ""
	}
	if err != nil {
		log.Println(err)
		return ""
	}
	return user
}

func (redisDriver *Driver) SetCachedUserInfo(ctx context.Context, userID, guildID, userData string) error {
	return redisDriver.client.Set(ctx, CachedUserInfoOnGuild(userID, guildID), userData, CachedUserDataExpiration).Err()
}

func (redisDriver *Driver) MarkUserRateLimit(userID, cmdType string, ttl time.Duration) {
	err := redisDriver.client.Set(context.Background(), UserRateLimitGeneral(userID), "", GlobalUserRateLimitDuration).Err()
	if err != nil {
		log.Println(err)
	}

	if cmdType != "" && ttl > 0 {
		err = redisDriver.client.Set(context.Background(), UserRateLimitSpecific(userID, cmdType), "", ttl).Err()
		if err != nil {
			log.Println(err)
		}
	}
}

func (redisDriver *Driver) IncrementRateLimitExceed(userID string) bool {
	t := time.Now().Unix()
	_, err := redisDriver.client.ZAdd(context.Background(), UserSoftbanCount(userID), &redis.Z{
		Score:  float64(t),
		Member: float64(t),
	}).Result()
	if err != nil {
		log.Println(err)
	}

	beforeStr := fmt.Sprintf("%d", time.Now().Add(-SoftbanExpiration).Unix())

	count, err := redisDriver.client.ZCount(context.Background(), UserSoftbanCount(userID),
		beforeStr,
		fmt.Sprintf("%d", t),
	).Result()
	if err != nil {
		log.Println(err)
	}
	if count > SoftbanThreshold {
		redisDriver.softbanUser(userID)
		return true
	}

	go redisDriver.client.ZRemRangeByScore(context.Background(), UserSoftbanCount(userID), "-inf", beforeStr)

	return false
}

func (redisDriver *Driver) softbanUser(userID string) {
	err := redisDriver.client.Set(context.Background(), UserSoftban(userID), "", SoftbanDuration).Err()
	if err != nil {
		log.Println(err)
	}
}

func (redisDriver *Driver) IsUserBanned(userID string) bool {
	v, err := redisDriver.client.Exists(context.Background(), UserSoftban(userID)).Result()
	if err != nil {
		log.Println(err)
		return false
	}
	return v == 1 // =1 means the user is present, and thus rate-limited
}

func (redisDriver *Driver) IsUserRateLimitedGeneral(userID string) bool {
	v, err := redisDriver.client.Exists(context.Background(), UserRateLimitGeneral(userID)).Result()
	if err != nil {
		log.Println(err)
		return false
	}
	return v == 1 // =1 means the user is present, and thus rate-limited
}

func (redisDriver *Driver) IsUserRateLimitedSpecific(userID string, cmdType string) bool {
	v, err := redisDriver.client.Exists(context.Background(), UserRateLimitSpecific(userID, cmdType)).Result()
	if err != nil {
		log.Println(err)
		return false
	}
	return v == 1 // =1 means the user is present, and thus rate-limited
}

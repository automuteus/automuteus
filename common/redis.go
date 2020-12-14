package common

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"log"
	"time"
)

const GlobalUserRateLimitDuration = 1 * time.Second

const NewGameRateLimitDuration = 3000 * time.Millisecond

const ReactionRateLimitDuration = 3000 * time.Millisecond

// when a user exceeds the threshold, they're ignored for this long
const SoftbanDuration = 5 * time.Minute

// how many violations before a softban
const SoftbanThreshold = 3

// how far back the bot should look for violations. Softban is invoked by violations>threshold in this amt of time
const SoftbanExpiration = 10 * time.Minute

func UserRateLimitGeneralKey(userID string) string {
	return "automuteus:ratelimit:user:" + userID
}

func UserRateLimitSpecificKey(userID, cmdType string) string {
	return "automuteus:ratelimit:user:" + cmdType + ":" + userID
}

func UserSoftbanKey(userID string) string {
	return "automuteus:ratelimit:softban:user:" + userID
}

func UserSoftbanCountKey(userID string) string {
	return "automuteus:ratelimit:softban:count:user:" + userID
}

func MarkUserRateLimit(client *redis.Client, userID, cmdType string, ttl time.Duration) {
	err := client.Set(context.Background(), UserRateLimitGeneralKey(userID), "", GlobalUserRateLimitDuration).Err()
	if err != nil {
		log.Println(err)
	}

	if cmdType != "" && ttl > 0 {
		err = client.Set(context.Background(), UserRateLimitSpecificKey(userID, cmdType), "", ttl).Err()
		if err != nil {
			log.Println(err)
		}
	}
}

func IncrementRateLimitExceed(client *redis.Client, userID string) bool {
	t := time.Now().Unix()
	_, err := client.ZAdd(context.Background(), UserSoftbanCountKey(userID), &redis.Z{
		Score:  float64(t),
		Member: float64(t),
	}).Result()
	if err != nil {
		log.Println(err)
	}

	beforeStr := fmt.Sprintf("%d", time.Now().Add(-SoftbanExpiration).Unix())

	count, err := client.ZCount(context.Background(), UserSoftbanCountKey(userID),
		beforeStr,
		fmt.Sprintf("%d", t),
	).Result()
	if err != nil {
		log.Println(err)
	}
	if count > SoftbanThreshold {
		softbanUser(client, userID)
		return true
	}

	go client.ZRemRangeByScore(context.Background(), UserSoftbanCountKey(userID), "-inf", beforeStr)

	return false
}

func softbanUser(client *redis.Client, userID string) {
	err := client.Set(context.Background(), UserSoftbanKey(userID), "", SoftbanDuration).Err()
	if err != nil {
		log.Println(err)
	}
}

func IsUserBanned(client *redis.Client, userID string) bool {
	v, err := client.Exists(context.Background(), UserSoftbanKey(userID)).Result()
	if err != nil {
		log.Println(err)
		return false
	}
	return v == 1 // =1 means the user is present, and thus rate-limited
}

func IsUserRateLimitedGeneral(client *redis.Client, userID string) bool {
	v, err := client.Exists(context.Background(), UserRateLimitGeneralKey(userID)).Result()
	if err != nil {
		log.Println(err)
		return false
	}
	return v == 1 // =1 means the user is present, and thus rate-limited
}

func IsUserRateLimitedSpecific(client *redis.Client, userID string, cmdType string) bool {
	v, err := client.Exists(context.Background(), UserRateLimitSpecificKey(userID, cmdType)).Result()
	if err != nil {
		log.Println(err)
		return false
	}
	return v == 1 // =1 means the user is present, and thus rate-limited
}

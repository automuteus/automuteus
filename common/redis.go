package common

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"log"
	"time"
)

const GlobalUserRateLimitSecs = 1

const NewGameRateLimitms = 3000

//when a user exceeds the threshold, they're ignored for this long
const SoftbanMinutes = 5

//how many violations before a softban
const SoftbanThreshold = 3

//how far back the bot should look for violations. Softban is invoked by violations>threshold in this amt of time
const SoftbanExpMinutes = 10

func VersionKey() string {
	return "automuteus:version"
}

func CommitKey() string {
	return "automuteus:commit"
}

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

func SetVersionAndCommit(client *redis.Client, version, commit string) {
	err := client.Set(context.Background(), VersionKey(), version, 0).Err()
	if err != nil {
		log.Println(err)
	}

	err = client.Set(context.Background(), CommitKey(), commit, 0).Err()
	if err != nil {
		log.Println(err)
	}
}

func GetVersionAndCommit(client *redis.Client) (string, string) {
	v, err := client.Get(context.Background(), VersionKey()).Result()
	if err != nil {
		log.Println(err)
	}

	c, err := client.Get(context.Background(), CommitKey()).Result()
	if err != nil {
		log.Println(err)
	}
	return v, c
}

func TotalGuildsKey() string {
	return "automuteus:count:guilds"
}

func MarkUserRateLimit(client *redis.Client, userID, cmdType string, ttlMS int64) {
	err := client.Set(context.Background(), UserRateLimitGeneralKey(userID), "", time.Second*GlobalUserRateLimitSecs).Err()
	if err != nil {
		log.Println(err)
	}

	if cmdType != "" && ttlMS > 0 {
		err = client.Set(context.Background(), UserRateLimitSpecificKey(userID, cmdType), "", time.Millisecond*time.Duration(ttlMS)).Err()
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

	beforeStr := fmt.Sprintf("%d", time.Now().Add(-time.Minute*SoftbanExpMinutes).Unix())

	count, err := client.ZCount(context.Background(), UserSoftbanCountKey(userID),
		beforeStr,
		fmt.Sprintf("%d", t),
	).Result()
	if count > SoftbanThreshold {
		softbanUser(client, userID)
		return true
	}

	go client.ZRemRangeByScore(context.Background(), UserSoftbanCountKey(userID), "-inf", beforeStr)

	return false
}

func softbanUser(client *redis.Client, userID string) {
	err := client.Set(context.Background(), UserSoftbanKey(userID), "", time.Minute*SoftbanMinutes).Err()
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
	return v == 1 //=1 means the user is present, and thus rate-limited
}

func IsUserRateLimitedGeneral(client *redis.Client, userID string) bool {
	v, err := client.Exists(context.Background(), UserRateLimitGeneralKey(userID)).Result()
	if err != nil {
		log.Println(err)
		return false
	}
	return v == 1 //=1 means the user is present, and thus rate-limited
}

func IsUserRateLimitedSpecific(client *redis.Client, userID string, cmdType string) bool {
	v, err := client.Exists(context.Background(), UserRateLimitSpecificKey(userID, cmdType)).Result()
	if err != nil {
		log.Println(err)
		return false
	}
	return v == 1 //=1 means the user is present, and thus rate-limited
}

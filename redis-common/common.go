package redis_common

import (
	"context"
	"github.com/go-redis/redis/v8"
	"log"
)

func VersionKey() string {
	return "automuteus:version"
}

func CommitKey() string {
	return "automuteus:commit"
}

func MatchIDKey() string {
	return "automuteus:match:counter"
}

func GetAndIncrementMatchID(client *redis.Client) int64 {
	num, err := client.Incr(context.Background(), MatchIDKey()).Result()
	if err != nil {
		log.Println(err)
	}
	return num
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

func TotalGuildsKey(version string) string {
	return "automuteus:count:guilds:version-" + version
}

func GetGuildCounter(client *redis.Client, version string) int64 {
	count, err := client.SCard(context.Background(), TotalGuildsKey(version)).Result()
	if err != nil {
		log.Println(err)
		return 0
	}
	return count
}

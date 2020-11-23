package metrics

import (
	"context"
	"fmt"
	redis_common "github.com/denverquane/amongusdiscord/redis-common"
	"github.com/go-redis/redis/v8"
	"log"
	"time"
)

func discordRequestsZsetKeyByCommit(commit string) string {
	return "automuteus:requests:commit:" + commit
}

func incrementDiscordRequests(client *redis.Client, count int64) {
	_, comm := redis_common.GetVersionAndCommit(client)

	t := time.Now()

	for i := int64(0); i < count; i++ {
		_, err := client.ZAdd(context.Background(), discordRequestsZsetKeyByCommit(comm), &redis.Z{
			Score:  float64(t.UnixNano() + i),
			Member: float64(t.UnixNano() + i), //add the time as the member to ensure (approx.) uniqueness
		}).Result()
		if err != nil {
			log.Println(err)
		}
	}
}

func GetDiscordRequestsInLastMinutes(client *redis.Client, numMinutes int) int {
	_, comm := redis_common.GetVersionAndCommit(client)

	before := time.Now().Add(-time.Minute * time.Duration(numMinutes)).UnixNano()

	games, err := client.ZRangeByScore(context.Background(), discordRequestsZsetKeyByCommit(comm), &redis.ZRangeBy{
		Min:    fmt.Sprintf("%d", before),
		Max:    fmt.Sprintf("%d", time.Now().UnixNano()),
		Offset: 0,
		Count:  0,
	}).Result()
	if err != nil {
		log.Println(err)
	}
	return len(games)
}

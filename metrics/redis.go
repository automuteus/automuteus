package metrics

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"log"
	"time"
)

func discordRequestsZsetKeyByCommit() string {
	return "automuteus:requests:zset"
}

func discordRequestsKeyByCommitAndType(typeStr string) string {
	return "automuteus:requests:type:" + typeStr
}

func incrementDiscordRequests(client *redis.Client, requestType MetricsEventType, count int64) {
	//t := time.Now()

	for i := int64(0); i < count; i++ {
		//only record in this zset if it's issued on the main token
		//if requestType != MuteDeafenCapture && requestType != MuteDeafenWorker {
		//	_, err := client.ZAdd(context.Background(), discordRequestsZsetKeyByCommit(), &redis.Z{
		//		Score:  float64(t.UnixNano() + i),
		//		Member: float64(t.UnixNano() + i), //add the time as the member to ensure (approx.) uniqueness
		//	}).Result()
		//	if err != nil {
		//		log.Println(err)
		//	}
		//}
		typeStr := MetricTypeStrings[requestType]
		client.Incr(context.Background(), discordRequestsKeyByCommitAndType(typeStr))
	}
}

func GetDiscordRequestsInLastMinutes(client *redis.Client, numMinutes int) int {
	before := time.Now().Add(-time.Minute * time.Duration(numMinutes)).UnixNano()

	games, err := client.ZRangeByScore(context.Background(), discordRequestsZsetKeyByCommit(), &redis.ZRangeBy{
		Min:    fmt.Sprintf("%d", before),
		Max:    fmt.Sprintf("%d", time.Now().UnixNano()),
		Offset: 0,
		Count:  0,
	}).Result()
	if err != nil {
		log.Println(err)
	}
	go client.ZRemRangeByScore(context.Background(), discordRequestsZsetKeyByCommit(), "-inf", fmt.Sprintf("%d", before))
	return len(games)
}

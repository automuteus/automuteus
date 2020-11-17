package metrics

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"log"
	"time"
)

func activeNodesKey() string {
	return "automuteus:nodes:all"
}

func discordRequestsZsetKeyByNodeID(nodeID string) string {
	return "automuteus:requests:" + nodeID
}

func IncrementDiscordRequests(client *redis.Client, nodeID string, count int) {
	if nodeID == "" {
		return
	}

	t := time.Now()

	//make sure the entry is refreshed in the overall nodes listing
	_, err := client.ZAdd(context.Background(), activeNodesKey(), &redis.Z{
		Score:  float64(t.Unix()),
		Member: nodeID,
	}).Result()

	for i := int64(0); i < int64(count); i++ {
		_, err = client.ZAdd(context.Background(), discordRequestsZsetKeyByNodeID(nodeID), &redis.Z{
			Score:  float64(t.UnixNano() + i),
			Member: float64(t.UnixNano() + i), //add the time as an element as it's always unique PER NODE (no 2 requests in the same ms, for the same node)
		}).Result()
	}

	if err != nil {
		log.Println(err)
	}
}

func GetDiscordRequestsInLastMinutesByNodeID(client *redis.Client, numMinutes int, nodeID string) int {
	if nodeID == "" {
		return 0
	}

	before := time.Now().Add(-time.Minute * time.Duration(numMinutes)).UnixNano()

	games, err := client.ZRangeByScore(context.Background(), discordRequestsZsetKeyByNodeID(nodeID), &redis.ZRangeBy{
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

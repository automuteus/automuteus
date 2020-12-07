package metrics

import (
	"context"
	"github.com/go-redis/redis/v8"
)

func discordRequestsKeyByCommitAndType(typeStr string) string {
	return "automuteus:requests:type:" + typeStr
}

func incrementDiscordRequests(client *redis.Client, requestType MetricsEventType, count int64) {
	for i := int64(0); i < count; i++ {
		typeStr := MetricTypeStrings[requestType]
		client.Incr(context.Background(), discordRequestsKeyByCommitAndType(typeStr))
	}
}

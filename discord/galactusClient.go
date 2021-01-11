package discord

import (
	"github.com/automuteus/utils/pkg/task"
	"github.com/denverquane/amongusdiscord/metrics"
	"github.com/go-redis/redis/v8"
)

func RecordDiscordRequestsByCounts(client *redis.Client, counts *task.MuteDeafenSuccessCounts) {
	metrics.RecordDiscordRequests(client, metrics.MuteDeafenOfficial, counts.Official)
	metrics.RecordDiscordRequests(client, metrics.MuteDeafenWorker, counts.Worker)
	metrics.RecordDiscordRequests(client, metrics.MuteDeafenCapture, counts.Capture)
	metrics.RecordDiscordRequests(client, metrics.InvalidRequest, counts.RateLimit)
}

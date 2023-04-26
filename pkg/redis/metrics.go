package redis

import "context"

type EventType int

const (
	MuteDeafenOfficial EventType = iota
	MessageCreateDelete
	MessageEdit
	MuteDeafenCapture
	MuteDeafenWorker
	InvalidRequest
	OfficialRequest //must be the last metric
)

var MetricTypeStrings = []string{
	"mute_deafen_official",
	"message_create_delete",
	"message_edit",
	"mute_deafen_capture",
	"mute_deafen_worker",
	"invalid_request",
	"official_request", //must be the last request
}

func (redisDriver *Driver) RecordDiscordRequests(requestType EventType, num int64) {
	for i := int64(0); i < num; i++ {
		typeStr := MetricTypeStrings[requestType]
		redisDriver.IncrRequestType(typeStr)
	}
}

func (redisDriver Driver) GetRequestsByType(str string) (string, error) {
	return redisDriver.client.Get(context.Background(), RequestsByType(str)).Result()
}

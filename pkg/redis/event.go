package redis

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/automuteus/automuteus/v8/pkg/capture"
	"time"
)

const DefaultCaptureBotTimeout = time.Second

const EventTTLSeconds = 3600

func (redisDriver *Driver) PushEvent(ctx context.Context, connCode string, jobType capture.EventType, payload string) error {
	event := capture.Event{
		EventType: jobType,
		Payload:   []byte(payload),
	}
	jBytes, err := json.Marshal(event)
	if err != nil {
		return err
	}

	count, err := redisDriver.client.RPush(ctx, EventsNamespace+connCode, string(jBytes)).Result()

	// new list
	if count < 2 {
		// log.Printf("Set TTL for List")
		redisDriver.client.Expire(ctx, EventsNamespace+connCode, EventTTLSeconds*time.Second)
	}

	return err
}

func (redisDriver *Driver) PopRawEvent(ctx context.Context, connCode string, timeout time.Duration) (string, error) {
	elems, err := redisDriver.client.BLPop(ctx, timeout, EventsNamespace+connCode).Result()
	if err != nil {
		return "", err
	}

	if len(elems) < 2 {
		return "", errors.New("insufficient elements returned")
	} else {
		return elems[1], nil
	}
}

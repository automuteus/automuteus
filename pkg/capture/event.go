package capture

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/automuteus/automuteus/pkg/rediskey"
	"github.com/go-redis/redis/v8"
	"time"
)

type EventType int

const (
	Connection EventType = iota
	Lobby
	State
	Player
	GameOver
)

type Event struct {
	EventType EventType `json:"type"`
	Payload   []byte    `json:"payload"`
}

const EventTTLSeconds = 3600

func PushEvent(ctx context.Context, redis *redis.Client, connCode string, jobType EventType, payload string) error {
	event := Event{
		EventType: jobType,
		Payload:   []byte(payload),
	}
	jBytes, err := json.Marshal(event)
	if err != nil {
		return err
	}

	count, err := redis.RPush(ctx, rediskey.EventsNamespace+connCode, string(jBytes)).Result()

	// new list
	if count < 2 {
		// log.Printf("Set TTL for List")
		redis.Expire(ctx, rediskey.EventsNamespace+connCode, EventTTLSeconds*time.Second)
	}

	return err
}

func PopRawEvent(ctx context.Context, redis *redis.Client, connCode string, timeout time.Duration) (string, error) {
	elems, err := redis.BLPop(ctx, timeout, rediskey.EventsNamespace+connCode).Result()
	if err != nil {
		return "", err
	}

	if len(elems) < 2 {
		return "", errors.New("insufficient elements returned")
	} else {
		return elems[1], nil
	}
}

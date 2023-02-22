package task

import (
	"context"
	"encoding/json"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/go-redis/redis/v8"
	"time"
)

type JobType int

const (
	ConnectionJob JobType = iota
	LobbyJob
	StateJob
	PlayerJob
	GameOverJob
)

type Job struct {
	JobType JobType     `json:"type"`
	Payload interface{} `json:"payload"`
}

const JobTTLSeconds = 3600

func PushJob(ctx context.Context, redis *redis.Client, connCode string, jobType JobType, payload string) error {
	job := Job{
		JobType: jobType,
		Payload: payload,
	}
	jBytes, err := json.Marshal(job)
	if err != nil {
		return err
	}

	count, err := redis.RPush(ctx, rediskey.JobNamespace+connCode, string(jBytes)).Result()
	if err == nil {
		notify(ctx, redis, connCode)
	}

	// new list
	if count < 2 {
		// log.Printf("Set TTL for List")
		redis.Expire(ctx, rediskey.JobNamespace+connCode, JobTTLSeconds*time.Second)
	}

	return err
}

func notify(ctx context.Context, redis *redis.Client, connCode string) {
	redis.Publish(ctx, rediskey.JobNamespace+connCode+":notify", true)
}

func Subscribe(ctx context.Context, redis *redis.Client, connCode string) *redis.PubSub {
	return redis.Subscribe(ctx, rediskey.JobNamespace+connCode+":notify")
}

func PopJob(ctx context.Context, redis *redis.Client, connCode string) (Job, error) {
	str, err := redis.LPop(ctx, rediskey.JobNamespace+connCode).Result()

	j := Job{}
	if err != nil {
		return j, err
	}
	err = json.Unmarshal([]byte(str), &j)
	return j, err
}

func Ack(ctx context.Context, redis *redis.Client, connCode string) {
	redis.Publish(ctx, rediskey.JobNamespace+connCode+":ack", true)
}

func AckSubscribe(ctx context.Context, redis *redis.Client, connCode string) *redis.PubSub {
	return redis.Subscribe(ctx, rediskey.JobNamespace+connCode+":ack")
}

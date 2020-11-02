package redisutil

import (
	"context"
	"encoding/json"
	"github.com/go-redis/redis/v8"
)

type JobType int

const (
	Connection JobType = iota
	ConnectCode
	Lobby
	State
	Player
)

type Job struct {
	JobType JobType     `json:"type"`
	Payload interface{} `json:"payload"`
}

const JobNamespace = "automuteus:jobs:"

func PushJob(ctx context.Context, redis *redis.Client, connCode string, jobType JobType, payload string) error {
	job := Job{
		JobType: jobType,
		Payload: payload,
	}
	jBytes, err := json.Marshal(job)
	if err != nil {
		return err
	}

	_, err = redis.RPush(ctx, JobNamespace+connCode, string(jBytes)).Result()
	if err == nil {
		notify(ctx, redis, connCode)
	}

	return err
}

func notify(ctx context.Context, redis *redis.Client, connCode string) {
	redis.Publish(ctx, JobNamespace+connCode+":notify", true)
}

func Subscribe(ctx context.Context, redis *redis.Client, connCode string) *redis.PubSub {
	return redis.Subscribe(ctx, JobNamespace+connCode+":notify")
}

func PopJob(ctx context.Context, redis *redis.Client, connCode string) (Job, error) {
	str, err := redis.LPop(ctx, JobNamespace+connCode).Result()

	j := Job{}
	if err != nil {
		return j, err
	}
	err = json.Unmarshal([]byte(str), &j)
	return j, err
}

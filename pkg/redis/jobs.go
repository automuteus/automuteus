package redis

import (
	"context"
	"encoding/json"
	"github.com/automuteus/automuteus/v8/pkg/task"
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

func (redisDriver *Driver) PushJob(ctx context.Context, connCode string, jobType JobType, payload string) error {
	job := Job{
		JobType: jobType,
		Payload: payload,
	}
	jBytes, err := json.Marshal(job)
	if err != nil {
		return err
	}

	count, err := redisDriver.client.RPush(ctx, JobNamespace+connCode, string(jBytes)).Result()
	if err == nil {
		redisDriver.notify(ctx, connCode)
	}

	// new list
	if count < 2 {
		// log.Printf("Set TTL for List")
		redisDriver.client.Expire(ctx, JobNamespace+connCode, JobTTLSeconds*time.Second)
	}

	return err
}

func (redisDriver *Driver) notify(ctx context.Context, connCode string) {
	redisDriver.client.Publish(ctx, JobNamespace+connCode+":notify", true)
}

func (redisDriver *Driver) TaskPublish(ctx context.Context, connCode string, task task.ModifyTask) error {
	jBytes, err := json.Marshal(task)
	if err != nil {
		return err
	}
	return redisDriver.client.Publish(ctx, tasksList(connCode), jBytes).Err()
}

func (redisDriver *Driver) Subscribe(ctx context.Context, connCode string) *redis.PubSub {
	return redisDriver.client.Subscribe(ctx, JobNamespace+connCode+":notify")
}

func (redisDriver *Driver) TaskSubscribe(ctx context.Context, task task.ModifyTask) *redis.PubSub {
	return redisDriver.client.Subscribe(ctx, completeTask(task.TaskID))
}

func (redisDriver *Driver) PopJob(ctx context.Context, connCode string) (Job, error) {
	str, err := redisDriver.client.LPop(ctx, JobNamespace+connCode).Result()

	j := Job{}
	if err != nil {
		return j, err
	}
	err = json.Unmarshal([]byte(str), &j)
	return j, err
}

func (redisDriver *Driver) Ack(ctx context.Context, connCode string) {
	redisDriver.client.Publish(ctx, JobNamespace+connCode+":ack", true)
}

func (redisDriver *Driver) AckSubscribe(ctx context.Context, connCode string) *redis.PubSub {
	return redisDriver.client.Subscribe(ctx, JobNamespace+connCode+":ack")
}

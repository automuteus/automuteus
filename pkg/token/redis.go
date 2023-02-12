package token

import (
	"context"
	"github.com/automuteus/automuteus/v7/pkg/rediskey"
	"github.com/go-redis/redis/v8"
	"log"
	"time"
)

func LockForToken(client *redis.Client, token string) {
	log.Println("Locking token for 5 seconds")
	err := client.Set(context.Background(), rediskey.BotTokenIdentifyLock(token), "", time.Second*5).Err()
	if err != nil {
		log.Println(err)
	}
}

func WaitForToken(client *redis.Client, token string) {
	for IsTokenLocked(client, token) {
		log.Println("Sleeping for 5 seconds while waiting for token to become available")
		time.Sleep(time.Second * 5)
	}
}

func IsTokenLocked(client *redis.Client, token string) bool {
	v, err := client.Exists(context.Background(), rediskey.BotTokenIdentifyLock(token)).Result()
	if err != nil {
		return false
	}

	return v == 1 //=1 means the rediskey is present, hence locked
}

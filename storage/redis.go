package storage

import (
	"context"
	"encoding/json"
	"github.com/go-redis/redis/v8"
	"log"
	"sync"
)

var ctx = context.Background()

type RedisCache struct {
	client *redis.Client

	guildSettingsLock sync.RWMutex
	guildSettings     map[HashedID]*GuildSettings
}

type RedisParameters struct {
	Addr     string
	Username string
	Password string
}

func (rd *RedisCache) Init(params interface{}) error {
	redisParams := params.(RedisParameters)
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisParams.Addr,
		Username: redisParams.Username,
		Password: redisParams.Password,
		DB:       0, // use default DB
	})
	rd.client = rdb

	rd.guildSettingsLock = sync.RWMutex{}
	rd.guildSettings = make(map[HashedID]*GuildSettings)
	return nil
}

func guildSettingsKey(guildID string) string {
	return "guild:settings:" + string(HashGuildID(guildID))
}

func (rd *RedisCache) InitGuildSettings(guildID string, guildName string) error {
	key := guildSettingsKey(guildID)
	val, err := rd.client.Get(ctx, key).Result()
	if err == redis.Nil {
		log.Printf("Creating guild settings for [%s | %s]\n", guildID, guildName)
		gs := MakeGuildSettings(guildID, guildName)
		jBytes, err := json.Marshal(gs)
		if err != nil {
			return err
		} else {
			log.Printf("Uploading guild settings for [%s | %s] to Redis\n", guildID, guildName)
			err := rd.client.Set(ctx, key, jBytes, 0).Err()
			if err != nil {
				return err
			} else {
				pubsub := rd.client.Subscribe(ctx, key)
				_, err := pubsub.Receive(ctx)
				if err != nil {
					return err
				}
				msgChannel := pubsub.Channel()
				err = rd.client.Publish(ctx, key, jBytes).Err()
				if err != nil {
					return err
				}
				log.Println("Worker Subscribed to events from " + key)
				go rd.guildSettingsWorker(msgChannel)

				return nil
			}
		}
	} else if err != nil {
		return err
	} else {
		log.Printf("Found guild settings for [%s | %s] in Redis\n", guildID, guildName)
		gs := GuildSettings{}
		err := json.Unmarshal([]byte(val), &gs)
		if err != nil {
			log.Println(err)
			return err
		}
		rd.guildSettingsLock.Lock()
		rd.guildSettings[HashedID(key)] = &gs
		rd.guildSettingsLock.Unlock()

		pubsub := rd.client.Subscribe(ctx, key)
		msgChannel := pubsub.Channel()
		log.Println("Worker Subscribed to events from " + key)
		go rd.guildSettingsWorker(msgChannel)
		return nil
	}
}

func (rd *RedisCache) guildSettingsWorker(msgs <-chan *redis.Message) {
	for {
		select {
		case msg := <-msgs:
			gs := GuildSettings{}
			err := json.Unmarshal([]byte(msg.Payload), &gs)
			if err != nil {
				log.Println(err)
				break
			}
			rd.guildSettingsLock.Lock()
			rd.guildSettings[HashedID(msg.Channel)] = &gs
			rd.guildSettingsLock.Unlock()
		}
	}
}

func (rd *RedisCache) Close() error {
	return rd.client.Close()
}

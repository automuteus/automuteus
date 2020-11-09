package storage

import (
	"context"
	"encoding/json"
	"github.com/go-redis/redis/v8"
	"log"
)

var ctx = context.Background()

type StorageInterface struct {
	client *redis.Client
}

type RedisParameters struct {
	Addr     string
	Username string
	Password string
}

func (storageInterface *StorageInterface) Init(params interface{}) error {
	redisParams := params.(RedisParameters)
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisParams.Addr,
		Username: redisParams.Username,
		Password: redisParams.Password,
		DB:       0, // use default DB
	})
	storageInterface.client = rdb
	return nil
}

func guildSettingsKey(id HashedID) string {
	return "automuteus:settings:guild:" + string(id)
}

func (storageInterface *StorageInterface) GetGuildSettings(guildID string) *GuildSettings {
	key := guildSettingsKey(HashGuildID(guildID))

	j, err := storageInterface.client.Get(ctx, key).Result()
	if err == redis.Nil {
		s := MakeGuildSettings()
		jBytes, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			log.Println(err)
			return MakeGuildSettings()
		}
		err = storageInterface.client.Set(ctx, key, jBytes, 0).Err()
		if err != nil {
			log.Println(err)
		}
		return s
	} else if err != nil {
		log.Println(err)
		return MakeGuildSettings()
	} else {
		s := GuildSettings{}
		err := json.Unmarshal([]byte(j), &s)
		if err != nil {
			log.Println(err)
			return MakeGuildSettings()
		}
		return &s
	}
}

func (storageInterface *StorageInterface) SetGuildSettings(guildID string, guildSettings *GuildSettings) error {
	key := guildSettingsKey(HashGuildID(guildID))

	jbytes, err := json.MarshalIndent(guildSettings, "", "  ")
	if err != nil {
		return err
	}
	err = storageInterface.client.Set(ctx, key, jbytes, 0).Err()
	return err
}

func (storageInterface *StorageInterface) DeleteGuildSettings(guildID string) error {
	key := guildSettingsKey(HashGuildID(guildID))

	err := storageInterface.client.Del(ctx, key).Err()
	return err
}

func (storageInterface *StorageInterface) Close() error {
	return storageInterface.client.Close()
}

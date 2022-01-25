package storage

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/automuteus/utils/pkg/rediskey"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/go-redis/redis/v8"
	"log"
	"os"
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

func (storageInterface *StorageInterface) GetGuildSettings(guildID string) *settings.GuildSettings {
	globalPrefix := os.Getenv("AUTOMUTEUS_GLOBAL_PREFIX")
	isOfficial := os.Getenv("AUTOMUTEUS_OFFICIAL") != ""
	key := rediskey.GuildSettings(string(HashGuildID(guildID)))

	j, err := storageInterface.client.Get(ctx, key).Result()
	switch {
	case errors.Is(err, redis.Nil):
		s := settings.MakeGuildSettings(globalPrefix, isOfficial)
		jBytes, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			log.Println(err)
			return settings.MakeGuildSettings(globalPrefix, isOfficial)
		}
		err = storageInterface.client.Set(ctx, key, jBytes, 0).Err()
		if err != nil {
			log.Println(err)
		}
		return s
	case err != nil:
		log.Println(err)
		return settings.MakeGuildSettings(globalPrefix, isOfficial)
	default:
		s := settings.GuildSettings{}
		err := json.Unmarshal([]byte(j), &s)
		if err != nil {
			log.Println(err)
			return settings.MakeGuildSettings(globalPrefix, isOfficial)
		}
		return &s
	}
}

func (storageInterface *StorageInterface) SetGuildSettings(guildID string, guildSettings *settings.GuildSettings) error {
	key := rediskey.GuildSettings(string(HashGuildID(guildID)))

	jbytes, err := json.MarshalIndent(guildSettings, "", "  ")
	if err != nil {
		return err
	}
	err = storageInterface.client.Set(ctx, key, jbytes, 0).Err()
	return err
}

func (storageInterface *StorageInterface) DeleteGuildSettings(guildID string) error {
	key := rediskey.GuildSettings(string(HashGuildID(guildID)))

	err := storageInterface.client.Del(ctx, key).Err()
	return err
}

func (storageInterface *StorageInterface) Close() error {
	return storageInterface.client.Close()
}

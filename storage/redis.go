package storage

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/j0nas500/automuteus/v8/pkg/rediskey"
	"github.com/j0nas500/automuteus/v8/pkg/settings"
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

func (storageInterface *StorageInterface) GuildSettingsExists(guildID string) bool {
	key := rediskey.GuildSettings(rediskey.HashGuildID(guildID))

	v, err := storageInterface.client.Exists(ctx, key).Result()
	return err == nil && v == 1
}

func (storageInterface *StorageInterface) GetGuildSettings(guildID string) *settings.GuildSettings {
	key := rediskey.GuildSettings(rediskey.HashGuildID(guildID))

	j, err := storageInterface.client.Get(ctx, key).Result()
	switch {
	case errors.Is(err, redis.Nil):
		s := settings.MakeGuildSettings()
		jBytes, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			log.Println(err)
			return settings.MakeGuildSettings()
		}
		err = storageInterface.client.Set(ctx, key, jBytes, 0).Err()
		if err != nil {
			log.Println(err)
		}
		return s
	case err != nil:
		log.Println(err)
		return settings.MakeGuildSettings()
	default:
		s := settings.GuildSettings{}
		err := json.Unmarshal([]byte(j), &s)
		if err != nil {
			log.Println(err)
			return settings.MakeGuildSettings()
		}
		return &s
	}
}

func (storageInterface *StorageInterface) SetGuildSettings(guildID string, guildSettings *settings.GuildSettings) error {
	key := rediskey.GuildSettings(rediskey.HashGuildID(guildID))

	jbytes, err := json.MarshalIndent(guildSettings, "", "  ")
	if err != nil {
		return err
	}
	err = storageInterface.client.Set(ctx, key, jbytes, 0).Err()
	return err
}

func (storageInterface *StorageInterface) DeleteGuildSettings(guildID string) error {
	key := rediskey.GuildSettings(rediskey.HashGuildID(guildID))

	err := storageInterface.client.Del(ctx, key).Err()
	return err
}

func (storageInterface *StorageInterface) Close() error {
	return storageInterface.client.Close()
}

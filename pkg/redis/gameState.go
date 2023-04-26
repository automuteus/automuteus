package redis

import (
	"encoding/json"
	"errors"
	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/bsm/redislock"
	redisv8 "github.com/go-redis/redis/v8"
	"log"
	"time"
)

// need at least one of these fields to fetch
func (redisDriver *Driver) GetReadOnlyDiscordGameState(gsr discord.GameStateRequest) *discord.GameState {
	dgs := redisDriver.getDiscordGameState(gsr, false)
	i := 0
	for dgs == nil {
		i++
		if i > 10 {
			log.Println("RETURNING NIL GAMESTATE FOR READONLY FETCH")
			return nil
		}
		dgs = redisDriver.getDiscordGameState(gsr, false)
	}
	return dgs
}

func (redisDriver *Driver) GetDiscordGameStateAndLockRetries(gsr discord.GameStateRequest, retries int) (*redislock.Lock, *discord.GameState) {
	lock, state := redisDriver.GetDiscordGameStateAndLock(gsr)
	var i int
	for lock == nil && i < retries {
		lock, state = redisDriver.GetDiscordGameStateAndLock(gsr)
		i++
	}
	return lock, state
}

func (redisDriver *Driver) GetDiscordGameStateAndLock(gsr discord.GameStateRequest) (*redislock.Lock, *discord.GameState) {
	key := redisDriver.getDiscordGameStateKey(gsr)
	locker := redislock.New(redisDriver.client)
	lock, err := locker.Obtain(ctx, key+":lock", time.Millisecond*LockTimeoutMs, &redislock.Options{
		RetryStrategy: redislock.LimitRetry(redislock.LinearBackoff(time.Millisecond*LinearBackoffMs), MaxRetries),
		Metadata:      "",
	})
	if errors.Is(err, redislock.ErrNotObtained) {
		return nil, nil
	} else if err != nil {
		log.Println(err)
		return nil, nil
	}

	return lock, redisDriver.getDiscordGameState(gsr, true)
}

func (redisDriver *Driver) getDiscordGameState(gsr discord.GameStateRequest, createOnNil bool) *discord.GameState {
	key := redisDriver.getDiscordGameStateKey(gsr)

	jsonStr, err := redisDriver.client.Get(ctx, key).Result()
	switch {
	case errors.Is(err, redisv8.Nil):
		if createOnNil {
			dgs := discord.NewDiscordGameState(gsr.GuildID)
			dgs.ConnectCode = gsr.ConnectCode
			dgs.GameStateMsg.MessageChannelID = gsr.TextChannel
			dgs.VoiceChannel = gsr.VoiceChannel
			redisDriver.SetDiscordGameState(dgs, nil)
			return dgs
		} else {
			return nil
		}
	case err != nil:
		log.Println(err)
		return nil
	default:
		dgs := discord.GameState{}
		err := json.Unmarshal([]byte(jsonStr), &dgs)
		if err != nil {
			log.Println(err)
			return nil
		}
		return &dgs
	}
}

func (redisDriver *Driver) CheckPointer(pointer string) string {
	key, err := redisDriver.client.Get(ctx, pointer).Result()
	if err != nil {
		return ""
	}
	return key
}

func (redisDriver *Driver) SetDiscordGameState(data *discord.GameState, lock *redislock.Lock) {
	if data == nil {
		if lock != nil {
			lock.Release(ctx)
		}
		return
	}

	key := redisDriver.getDiscordGameStateKey(discord.GameStateRequest{
		GuildID:      data.GuildID,
		TextChannel:  data.GameStateMsg.MessageChannelID,
		VoiceChannel: data.VoiceChannel,
		ConnectCode:  data.ConnectCode,
	})

	// connectCode is the 1 sole key we should ever rely on for tracking games. Because we generate it ourselves
	// randomly, it's unique to every single game, and the capture and bot BOTH agree on the linkage
	if key == "" && data.ConnectCode == "" {
		if lock != nil {
			lock.Release(ctx)
		}
		return
	}
	key = ConnectCodeData(data.GuildID, data.ConnectCode)

	jBytes, err := json.Marshal(data)
	if err != nil {
		log.Println(err)
		if lock != nil {
			lock.Release(ctx)
		}
		return
	}

	err = redisDriver.client.Set(ctx, key, jBytes, GameTimeoutSeconds*time.Second).Err()
	if err != nil {
		log.Println(err)
	}

	if lock != nil {
		lock.Release(ctx)
	}

	if data.ConnectCode != "" {
		err = redisDriver.client.Set(ctx, ConnectCodePtr(data.GuildID, data.ConnectCode), key, GameTimeoutSeconds*time.Second).Err()
		if err != nil {
			log.Println(err)
		}
	}

	if data.VoiceChannel != "" {
		err = redisDriver.client.Set(ctx, VoiceChannelPtr(data.GuildID, data.VoiceChannel), key, GameTimeoutSeconds*time.Second).Err()
		if err != nil {
			log.Println(err)
		}
	}

	if data.GameStateMsg.MessageChannelID != "" {
		err = redisDriver.client.Set(ctx, TextChannelPtr(data.GuildID, data.GameStateMsg.MessageChannelID), key, GameTimeoutSeconds*time.Second).Err()
		if err != nil {
			log.Println(err)
		}
	}
}

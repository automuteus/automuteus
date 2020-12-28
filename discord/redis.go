package discord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/automuteus/utils/pkg/rediskey"
	"github.com/bsm/redislock"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/metrics"
	"github.com/denverquane/amongusdiscord/storage"
	"github.com/go-redis/redis/v8"
	"log"
	"time"
)

var ctx = context.Background()

const LockTimeoutMs = 250
const LinearBackoffMs = 100
const MaxRetries = 10
const SnowflakeLockMs = 3000

// 15 minute timeout
const GameTimeoutSeconds = 900

type RedisInterface struct {
	client *redis.Client
}

func (redisInterface *RedisInterface) Init(params interface{}) error {
	redisParams := params.(storage.RedisParameters)
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisParams.Addr,
		Username: redisParams.Username,
		Password: redisParams.Password,
		DB:       0, // use default DB
	})
	redisInterface.client = rdb
	return nil
}

func (bot *Bot) refreshGameLiveness(code string) {
	t := time.Now()
	bot.RedisInterface.client.ZAdd(ctx, rediskey.ActiveGamesZSet, &redis.Z{
		Score:  float64(t.Unix()),
		Member: code,
	})
	before := t.Add(-time.Second * GameTimeoutSeconds)
	go bot.RedisInterface.client.ZRemRangeByScore(context.Background(), rediskey.ActiveGamesZSet, "-inf", fmt.Sprintf("%d", before.Unix()))
}

func (bot *Bot) rateLimitEventCallback(sess *discordgo.Session, rl *discordgo.RateLimit) {
	log.Println(rl.Message)
	metrics.RecordDiscordRequests(bot.RedisInterface.client, metrics.InvalidRequest, 1)
}

func (redisInterface *RedisInterface) AddUniqueGuildCounter(guildID string) {
	_, err := redisInterface.client.SAdd(ctx, rediskey.TotalGuildsSet, string(storage.HashGuildID(guildID))).Result()
	if err != nil {
		log.Println(err)
	}
}

func (redisInterface *RedisInterface) LeaveUniqueGuildCounter(guildID string) {
	_, err := redisInterface.client.SRem(ctx, rediskey.TotalGuildsSet, string(storage.HashGuildID(guildID))).Result()
	if err != nil {
		log.Println(err)
	}
}

// TODO this can technically be a race condition? what happens if one of these is updated while we're fetching...
func (redisInterface *RedisInterface) getDiscordGameStateKey(gsr GameStateRequest) string {
	key := redisInterface.CheckPointer(rediskey.ConnectCodePtr(gsr.GuildID, gsr.ConnectCode))
	if key == "" {
		key = redisInterface.CheckPointer(rediskey.TextChannelPtr(gsr.GuildID, gsr.TextChannel))
		if key == "" {
			key = redisInterface.CheckPointer(rediskey.VoiceChannelPtr(gsr.GuildID, gsr.VoiceChannel))
		}
	}
	return key
}

type GameStateRequest struct {
	GuildID      string
	TextChannel  string
	VoiceChannel string
	ConnectCode  string
}

func (redisInterface *RedisInterface) LockVoiceChanges(connectCode string, dur time.Duration) *redislock.Lock {
	locker := redislock.New(redisInterface.client)
	lock, err := locker.Obtain(ctx, rediskey.VoiceChangesForGameCodeLock(connectCode), dur, &redislock.Options{
		RetryStrategy: redislock.LimitRetry(redislock.LinearBackoff(time.Millisecond*LinearBackoffMs), MaxRetries),
		Metadata:      "",
	})
	if errors.Is(err, redislock.ErrNotObtained) {
		return nil
	} else if err != nil {
		log.Println(err)
		return nil
	}

	return lock
}

// need at least one of these fields to fetch
func (redisInterface *RedisInterface) GetReadOnlyDiscordGameState(gsr GameStateRequest) *GameState {
	dgs := redisInterface.getDiscordGameState(gsr)
	i := 0
	for dgs == nil {
		i++
		if i > 10 {
			log.Println("RETURNING NIL GAMESTATE FOR READONLY FETCH")
			return nil
		}
		dgs = redisInterface.getDiscordGameState(gsr)
	}
	return dgs
}

func (redisInterface *RedisInterface) GetDiscordGameStateAndLock(gsr GameStateRequest) (*redislock.Lock, *GameState) {
	key := redisInterface.getDiscordGameStateKey(gsr)
	locker := redislock.New(redisInterface.client)
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

	return lock, redisInterface.getDiscordGameState(gsr)
}

func (redisInterface *RedisInterface) getDiscordGameState(gsr GameStateRequest) *GameState {
	key := redisInterface.getDiscordGameStateKey(gsr)

	jsonStr, err := redisInterface.client.Get(ctx, key).Result()
	switch {
	case errors.Is(err, redis.Nil):
		dgs := NewDiscordGameState(gsr.GuildID)
		dgs.ConnectCode = gsr.ConnectCode
		dgs.GameStateMsg.MessageChannelID = gsr.TextChannel
		dgs.Tracking.ChannelID = gsr.VoiceChannel
		redisInterface.SetDiscordGameState(dgs, nil)
		return dgs
	case err != nil:
		log.Println(err)
		return nil
	default:
		dgs := GameState{}
		err := json.Unmarshal([]byte(jsonStr), &dgs)
		if err != nil {
			log.Println(err)
			return nil
		}
		return &dgs
	}
}

func (redisInterface *RedisInterface) CheckPointer(pointer string) string {
	key, err := redisInterface.client.Get(ctx, pointer).Result()
	if err != nil {
		return ""
	}
	return key
}

func (redisInterface *RedisInterface) SetDiscordGameState(data *GameState, lock *redislock.Lock) {
	if data == nil {
		if lock != nil {
			lock.Release(ctx)
		}
		return
	}

	key := redisInterface.getDiscordGameStateKey(GameStateRequest{
		GuildID:      data.GuildID,
		TextChannel:  data.GameStateMsg.MessageChannelID,
		VoiceChannel: data.Tracking.ChannelID,
		ConnectCode:  data.ConnectCode,
	})

	// connectCode is the 1 sole key we should ever rely on for tracking games. Because we generate it ourselves
	// randomly, it's unique to every single amongus, and the capture and bot BOTH agree on the linkage
	if key == "" && data.ConnectCode == "" {
		if lock != nil {
			lock.Release(ctx)
		}
		return
	}
	key = rediskey.ConnectCodeData(data.GuildID, data.ConnectCode)

	jBytes, err := json.Marshal(data)
	if err != nil {
		log.Println(err)
		if lock != nil {
			lock.Release(ctx)
		}
		return
	}

	err = redisInterface.client.Set(ctx, key, jBytes, GameTimeoutSeconds*time.Second).Err()
	if err != nil {
		log.Println(err)
	}

	if lock != nil {
		lock.Release(ctx)
	}

	if data.ConnectCode != "" {
		err = redisInterface.client.Set(ctx, rediskey.ConnectCodePtr(data.GuildID, data.ConnectCode), key, GameTimeoutSeconds*time.Second).Err()
		if err != nil {
			log.Println(err)
		}
	}

	if data.Tracking.ChannelID != "" {
		err = redisInterface.client.Set(ctx, rediskey.VoiceChannelPtr(data.GuildID, data.Tracking.ChannelID), key, GameTimeoutSeconds*time.Second).Err()
		if err != nil {
			log.Println(err)
		}
	}

	if data.GameStateMsg.MessageChannelID != "" {
		err = redisInterface.client.Set(ctx, rediskey.TextChannelPtr(data.GuildID, data.GameStateMsg.MessageChannelID), key, GameTimeoutSeconds*time.Second).Err()
		if err != nil {
			log.Println(err)
		}
	}
}

func (redisInterface *RedisInterface) RefreshActiveGame(guildID, connectCode string) {
	key := rediskey.ActiveGamesForGuild(guildID)
	t := time.Now()
	_, err := redisInterface.client.ZAdd(ctx, key, &redis.Z{
		Score:  float64(t.Unix()),
		Member: connectCode,
	}).Result()

	if err != nil {
		log.Println(err)
	}
	before := t.Add(-time.Second * GameTimeoutSeconds)
	go redisInterface.client.ZRemRangeByScore(context.Background(), rediskey.ActiveGamesZSet, "-inf", fmt.Sprintf("%d", before.Unix()))
}

func (redisInterface *RedisInterface) RemoveOldGame(guildID, connectCode string) {
	key := rediskey.ActiveGamesForGuild(guildID)

	err := redisInterface.client.ZRem(ctx, key, connectCode).Err()
	if err != nil {
		log.Println(err)
	}
}

// only deletes from the guild's responsibility, NOT the entire guild counter!
func (redisInterface *RedisInterface) LoadAllActiveGames(guildID string) []string {
	hash := rediskey.ActiveGamesForGuild(guildID)

	before := time.Now().Add(-time.Second * GameTimeoutSeconds).Unix()

	games, err := redisInterface.client.ZRangeByScore(ctx, hash, &redis.ZRangeBy{
		Min:    fmt.Sprintf("%d", before),
		Max:    fmt.Sprintf("%d", time.Now().Unix()),
		Offset: 0,
		Count:  0,
	}).Result()

	if err != nil {
		log.Println(err)
		return []string{}
	}
	go redisInterface.client.ZRemRangeByScore(context.Background(), hash, "-inf", fmt.Sprintf("%d", before))

	return games
}

func (redisInterface *RedisInterface) DeleteDiscordGameState(dgs *GameState) {
	guildID := dgs.GuildID
	connCode := dgs.ConnectCode
	if guildID == "" || connCode == "" {
		log.Println("Can't delete DGS with null guildID or null ConnCode")
	}
	data := redisInterface.getDiscordGameState(GameStateRequest{
		GuildID:     guildID,
		ConnectCode: connCode,
	})
	key := rediskey.ConnectCodeData(guildID, connCode)

	locker := redislock.New(redisInterface.client)
	lock, err := locker.Obtain(ctx, key+":lock", time.Millisecond*LockTimeoutMs, &redislock.Options{
		RetryStrategy: redislock.LimitRetry(redislock.LinearBackoff(time.Millisecond*LinearBackoffMs), MaxRetries),
		Metadata:      "",
	})
	switch {
	case errors.Is(err, redislock.ErrNotObtained):
		fmt.Println("Could not obtain lock!")
	case err != nil:
		log.Fatalln(err)
	default:
		defer lock.Release(ctx)
	}

	// delete all the pointers to the underlying -actual- discord data
	err = redisInterface.client.Del(ctx, rediskey.TextChannelPtr(guildID, data.GameStateMsg.MessageChannelID)).Err()
	if err != nil {
		log.Println(err)
	}
	err = redisInterface.client.Del(ctx, rediskey.VoiceChannelPtr(guildID, data.Tracking.ChannelID)).Err()
	if err != nil {
		log.Println(err)
	}
	err = redisInterface.client.Del(ctx, rediskey.ConnectCodePtr(guildID, data.ConnectCode)).Err()
	if err != nil {
		log.Println(err)
	}

	err = redisInterface.client.Del(ctx, key).Err()
	if err != nil {
		log.Println(err)
	}
}

func (redisInterface *RedisInterface) GetUsernameOrUserIDMappings(guildID, key string) map[string]interface{} {
	cacheHash := rediskey.GuildCacheHash(guildID)

	value, err := redisInterface.client.HGet(ctx, cacheHash, key).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			log.Println(err)
		}
		return map[string]interface{}{}
	}
	var ret map[string]interface{}
	err = json.Unmarshal([]byte(value), &ret)
	if err != nil {
		log.Println(err)
		return map[string]interface{}{}
	}

	// log.Println(ret)
	return ret
}

func (redisInterface *RedisInterface) AddUsernameLink(guildID, userID, userName string) error {
	err := redisInterface.appendToHashedEntry(guildID, userID, userName)
	if err != nil {
		return err
	}
	return redisInterface.appendToHashedEntry(guildID, userName, userID)
}

func (redisInterface *RedisInterface) DeleteLinksByUserID(guildID, userID string) error {
	// over all the usernames associated with just this userID, delete the underlying mapping of username->userID
	usernames := redisInterface.GetUsernameOrUserIDMappings(guildID, userID)
	for username := range usernames {
		err := redisInterface.deleteHashSubEntry(guildID, username, userID)
		if err != nil {
			log.Println(err)
		}
	}

	// now delete the userID->username list entirely
	cacheHash := rediskey.GuildCacheHash(guildID)
	return redisInterface.client.HDel(ctx, cacheHash, userID).Err()
}

func (redisInterface *RedisInterface) appendToHashedEntry(guildID, key, value string) error {
	resp := redisInterface.GetUsernameOrUserIDMappings(guildID, key)

	resp[value] = struct{}{}

	return redisInterface.setUsernameOrUserIDMappings(guildID, key, resp)
}

func (redisInterface *RedisInterface) deleteHashSubEntry(guildID, key, entry string) error {
	entries := redisInterface.GetUsernameOrUserIDMappings(guildID, key)

	delete(entries, entry)
	return redisInterface.setUsernameOrUserIDMappings(guildID, key, entries)
}

func (redisInterface *RedisInterface) setUsernameOrUserIDMappings(guildID, key string, values map[string]interface{}) error {
	cacheHash := rediskey.GuildCacheHash(guildID)

	jBytes, err := json.Marshal(values)
	if err != nil {
		return err
	}

	err = redisInterface.client.HSet(ctx, cacheHash, key, jBytes).Err()
	// 1 week TTL on username cache
	if err == nil {
		redisInterface.client.Expire(ctx, cacheHash, time.Hour*24*7)
	}

	return err
}

func (redisInterface *RedisInterface) LockSnowflake(snowflake string) *redislock.Lock {
	locker := redislock.New(redisInterface.client)
	lock, err := locker.Obtain(ctx, rediskey.SnowflakeLockID(snowflake), time.Millisecond*SnowflakeLockMs, nil)
	if errors.Is(err, redislock.ErrNotObtained) {
		return nil
	} else if err != nil {
		log.Println(err)
		return nil
	}
	return lock
}

func (redisInterface *RedisInterface) Close() error {
	return redisInterface.client.Close()
}

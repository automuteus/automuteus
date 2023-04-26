package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/automuteus/automuteus/v8/pkg/storage"
	"github.com/bsm/redislock"
	"github.com/bwmarrin/discordgo"
	redisv8 "github.com/go-redis/redis/v8"
	"log"
	"time"
)

const LockTimeoutMs = 250
const LinearBackoffMs = 100
const MaxRetries = 10
const SnowflakeLockMs = 3000
const GameTimeoutSeconds = 900

var ctx = context.Background()

type Driver struct {
	client *redisv8.Client
}

func (redisDriver *Driver) Init(params interface{}) error {
	redisParams := params.(storage.RedisParameters)
	rdb := redisv8.NewClient(&redisv8.Options{
		Addr:     redisParams.Addr,
		Username: redisParams.Username,
		Password: redisParams.Password,
		DB:       0, // use default DB
	})
	redisDriver.client = rdb
	return nil
}

func (redisDriver *Driver) AddUniqueGuildCounter(guildID string) {
	_, err := redisDriver.client.SAdd(ctx, TotalGuildsSet, string(storage.HashGuildID(guildID))).Result()
	if err != nil {
		log.Println(err)
	}
}

func (redisDriver *Driver) LeaveUniqueGuildCounter(guildID string) {
	_, err := redisDriver.client.SRem(ctx, TotalGuildsSet, string(storage.HashGuildID(guildID))).Result()
	if err != nil {
		log.Println(err)
	}
}

func (redisDriver *Driver) CheckDiscordGameStateKey(gsr discord.GameStateRequest) bool {
	return redisDriver.getDiscordGameStateKey(gsr) != ""
}

func (redisDriver *Driver) GameExists(gsr discord.GameStateRequest) bool {
	return redisDriver.getDiscordGameStateKey(gsr) != ""
}

// TODO this can technically be a race condition? what happens if one of these is updated while we're fetching...
func (redisDriver *Driver) getDiscordGameStateKey(gsr discord.GameStateRequest) string {
	key := redisDriver.CheckPointer(ConnectCodePtr(gsr.GuildID, gsr.ConnectCode))
	if key == "" {
		key = redisDriver.CheckPointer(TextChannelPtr(gsr.GuildID, gsr.TextChannel))
		if key == "" {
			key = redisDriver.CheckPointer(VoiceChannelPtr(gsr.GuildID, gsr.VoiceChannel))
		}
	}
	return key
}

func (redisDriver *Driver) LockVoiceChanges(connectCode string, dur time.Duration) *redislock.Lock {
	locker := redislock.New(redisDriver.client)
	lock, err := locker.Obtain(ctx, VoiceChangesForGameCodeLock(connectCode), dur, &redislock.Options{
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

func (redisDriver *Driver) RefreshActiveGame(guildID, connectCode string) {
	key := ActiveGamesForGuild(guildID)
	t := time.Now()
	_, err := redisDriver.client.ZAdd(ctx, key, &redisv8.Z{
		Score:  float64(t.Unix()),
		Member: connectCode,
	}).Result()

	if err != nil {
		log.Println(err)
	}
	before := t.Add(-time.Second * GameTimeoutSeconds)
	go redisDriver.client.ZRemRangeByScore(context.Background(), ActiveGamesZSet, "-inf", fmt.Sprintf("%d", before.Unix()))
}

func (redisDriver *Driver) RemoveOldGame(guildID, connectCode string) {
	key := ActiveGamesForGuild(guildID)

	err := redisDriver.client.ZRem(ctx, key, connectCode).Err()
	if err != nil {
		log.Println(err)
	}
}

// only deletes from the guild's responsibility, NOT the entire guild counter!
func (redisDriver *Driver) LoadAllActiveGames(guildID string) []string {
	hash := ActiveGamesForGuild(guildID)

	before := time.Now().Add(-time.Second * GameTimeoutSeconds).Unix()

	games, err := redisDriver.client.ZRangeByScore(ctx, hash, &redisv8.ZRangeBy{
		Min:    fmt.Sprintf("%d", before),
		Max:    fmt.Sprintf("%d", time.Now().Unix()),
		Offset: 0,
		Count:  0,
	}).Result()

	if err != nil {
		log.Println(err)
		return []string{}
	}
	go redisDriver.client.ZRemRangeByScore(context.Background(), hash, "-inf", fmt.Sprintf("%d", before))

	return games
}

func (redisDriver *Driver) DeleteDiscordGameState(dgs *discord.GameState) {
	guildID := dgs.GuildID
	connCode := dgs.ConnectCode
	if guildID == "" || connCode == "" {
		log.Println("Can't delete DGS with null guildID or null ConnCode")
	}
	data := redisDriver.getDiscordGameState(discord.GameStateRequest{
		GuildID:     guildID,
		ConnectCode: connCode,
	}, false)

	// couldn't find the game state; exit
	if data == nil {
		return
	}
	key := ConnectCodeData(guildID, connCode)

	locker := redislock.New(redisDriver.client)
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
	err = redisDriver.client.Del(ctx, TextChannelPtr(guildID, data.GameStateMsg.MessageChannelID)).Err()
	if err != nil {
		log.Println(err)
	}
	err = redisDriver.client.Del(ctx, VoiceChannelPtr(guildID, data.VoiceChannel)).Err()
	if err != nil {
		log.Println(err)
	}
	err = redisDriver.client.Del(ctx, ConnectCodePtr(guildID, data.ConnectCode)).Err()
	if err != nil {
		log.Println(err)
	}

	err = redisDriver.client.Del(ctx, key).Err()
	if err != nil {
		log.Println(err)
	}
}

func (redisDriver *Driver) GetUsernameOrUserIDMappings(guildID, key string) (map[string]interface{}, error) {
	cacheHash := GuildCacheHash(guildID)

	value, err := redisDriver.client.HGet(ctx, cacheHash, key).Result()
	if err != nil {
		if !errors.Is(err, redisv8.Nil) {
			return map[string]interface{}{}, err
		}
		// redis.Nil (not found) is not *actually* an error, so return nil
		return map[string]interface{}{}, nil
	}
	var ret map[string]interface{}
	err = json.Unmarshal([]byte(value), &ret)
	if err != nil {
		return map[string]interface{}{}, err
	}

	return ret, nil
}

func (redisDriver *Driver) AddUsernameLink(guildID, userID, userName string) error {
	err := redisDriver.appendToHashedEntry(guildID, userID, userName)
	if err != nil {
		return err
	}
	return redisDriver.appendToHashedEntry(guildID, userName, userID)
}

func (redisDriver *Driver) DeleteLinksByUserID(guildID, userID string) error {
	// over all the usernames associated with just this userID, delete the underlying mapping of username->userID
	usernames, err := redisDriver.GetUsernameOrUserIDMappings(guildID, userID)
	if err != nil {
		log.Println(err)
	} else {
		for username := range usernames {
			err := redisDriver.deleteHashSubEntry(guildID, username, userID)
			if err != nil {
				log.Println(err)
			}
		}
	}

	// now delete the userID->username list entirely
	cacheHash := GuildCacheHash(guildID)
	return redisDriver.client.HDel(ctx, cacheHash, userID).Err()
}

func (redisDriver *Driver) appendToHashedEntry(guildID, key, value string) error {
	resp, err := redisDriver.GetUsernameOrUserIDMappings(guildID, key)
	if err != nil {
		log.Println(err)
	}

	resp[value] = struct{}{}

	return redisDriver.setUsernameOrUserIDMappings(guildID, key, resp)
}

func (redisDriver *Driver) deleteHashSubEntry(guildID, key, entry string) error {
	entries, err := redisDriver.GetUsernameOrUserIDMappings(guildID, key)
	if err != nil {
		log.Println(err)
	} else {
		delete(entries, entry)
	}

	return redisDriver.setUsernameOrUserIDMappings(guildID, key, entries)
}

func (redisDriver *Driver) setUsernameOrUserIDMappings(guildID, key string, values map[string]interface{}) error {
	cacheHash := GuildCacheHash(guildID)

	jBytes, err := json.Marshal(values)
	if err != nil {
		return err
	}

	err = redisDriver.client.HSet(ctx, cacheHash, key, jBytes).Err()
	// 1 week TTL on username cache
	if err == nil {
		redisDriver.client.Expire(ctx, cacheHash, time.Hour*24*7)
	}

	return err
}

func (redisDriver *Driver) LockSnowflake(snowflake string) *redislock.Lock {
	locker := redislock.New(redisDriver.client)
	lock, err := locker.Obtain(ctx, SnowflakeLockID(snowflake), time.Millisecond*SnowflakeLockMs, nil)
	if errors.Is(err, redislock.ErrNotObtained) {
		return nil
	} else if err != nil {
		log.Println(err)
		return nil
	}
	return lock
}

func (redisDriver *Driver) Close() error {
	return redisDriver.client.Close()
}

func (redisDriver *Driver) RefreshGameLiveness(code string) {
	t := time.Now()
	redisDriver.client.ZAdd(ctx, ActiveGamesZSet, &redisv8.Z{
		Score:  float64(t.Unix()),
		Member: code,
	})
	before := t.Add(-time.Second * GameTimeoutSeconds)
	go redisDriver.client.ZRemRangeByScore(context.Background(), ActiveGamesZSet, "-inf", fmt.Sprintf("%d", before.Unix()))
}

func (redisDriver *Driver) RateLimitEventCallback(rl *discordgo.RateLimit) {
	log.Println(rl.Message)
	redisDriver.RecordDiscordRequests(InvalidRequest, 1)
}

func (redisDriver *Driver) IncrRequestType(str string) {
	redisDriver.client.Incr(context.Background(), RequestsByType(str))
}

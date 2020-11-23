package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/bsm/redislock"
	rediscommon "github.com/denverquane/amongusdiscord/redis-common"
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

//15 minute timeout
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

func activeGamesCode() string {
	return "automuteus:games"
}

func (bot *Bot) refreshGameLiveness(code string) {
	t := time.Now().Unix()
	bot.RedisInterface.client.ZAdd(ctx, activeGamesCode(), &redis.Z{
		Score:  float64(t),
		Member: code,
	})
}

func activeGamesKey(guildID string) string {
	return "automuteus:discord:" + guildID + ":games:set"
}

func discordKeyTextChannelPtr(guildID, channelID string) string {
	return "automuteus:discord:" + guildID + ":pointer:text:" + channelID
}

func discordKeyVoiceChannelPtr(guildID, channelID string) string {
	return "automuteus:discord:" + guildID + ":pointer:voice:" + channelID
}

func discordKeyConnectCodePtr(guildID, code string) string {
	return "automuteus:discord:" + guildID + ":pointer:code:" + code
}

func discordKey(guildID, id string) string {
	return "automuteus:discord:" + guildID + ":" + id
}

func cacheHash(guildID string) string {
	return "automuteus:discord:" + guildID + ":cache"
}

func snowflakeLockID(snowflake string) string {
	return "automuteus:snowflake:" + snowflake + ":lock"
}

func (redisInterface *RedisInterface) AddUniqueGuildCounter(guildID, version string) {
	_, err := redisInterface.client.SAdd(ctx, rediscommon.TotalGuildsKey(version), string(storage.HashGuildID(guildID))).Result()
	if err != nil {
		log.Println(err)
	}
}

func (redisInterface *RedisInterface) LeaveUniqueGuildCounter(guildID, version string) {
	_, err := redisInterface.client.SRem(ctx, rediscommon.TotalGuildsKey(version), string(storage.HashGuildID(guildID))).Result()
	if err != nil {
		log.Println(err)
	}
}

//todo this can technically be a race condition? what happens if one of these is updated while we're fetching...
func (redisInterface *RedisInterface) getDiscordGameStateKey(gsr GameStateRequest) string {
	key := redisInterface.CheckPointer(discordKeyConnectCodePtr(gsr.GuildID, gsr.ConnectCode))
	if key == "" {
		key = redisInterface.CheckPointer(discordKeyTextChannelPtr(gsr.GuildID, gsr.TextChannel))
		if key == "" {
			key = redisInterface.CheckPointer(discordKeyVoiceChannelPtr(gsr.GuildID, gsr.VoiceChannel))
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

//need at least one of these fields to fetch
func (redisInterface *RedisInterface) GetReadOnlyDiscordGameState(gsr GameStateRequest) *DiscordGameState {
	return redisInterface.getDiscordGameState(gsr)
}

func (redisInterface *RedisInterface) GetDiscordGameStateAndLock(gsr GameStateRequest) (*redislock.Lock, *DiscordGameState) {
	key := redisInterface.getDiscordGameStateKey(gsr)
	locker := redislock.New(redisInterface.client)
	lock, err := locker.Obtain(ctx, key+":lock", time.Millisecond*LockTimeoutMs, &redislock.Options{
		RetryStrategy: redislock.LimitRetry(redislock.LinearBackoff(time.Millisecond*LinearBackoffMs), MaxRetries),
		Metadata:      "",
	})
	if err == redislock.ErrNotObtained {
		return nil, nil
	} else if err != nil {
		log.Println(err)
		return nil, nil
	}

	return lock, redisInterface.getDiscordGameState(gsr)
}

func (redisInterface *RedisInterface) getDiscordGameState(gsr GameStateRequest) *DiscordGameState {
	key := redisInterface.getDiscordGameStateKey(gsr)

	jsonStr, err := redisInterface.client.Get(ctx, key).Result()
	if err == redis.Nil {
		dgs := NewDiscordGameState(gsr.GuildID)
		//this is a little silly, but it works...
		dgs.ConnectCode = gsr.ConnectCode
		dgs.GameStateMsg.MessageChannelID = gsr.TextChannel
		dgs.Tracking.ChannelID = gsr.VoiceChannel
		redisInterface.SetDiscordGameState(dgs, nil)
		return dgs
	} else if err != nil {
		log.Println(err)
		return nil
	} else {
		dgs := DiscordGameState{}
		err := json.Unmarshal([]byte(jsonStr), &dgs)
		if err != nil {
			log.Println(err)
			return nil
		} else {
			return &dgs
		}
	}
}

func (redisInterface *RedisInterface) CheckPointer(pointer string) string {
	key, err := redisInterface.client.Get(ctx, pointer).Result()
	if err != nil {
		return ""
	} else {
		return key
	}
}

func (redisInterface *RedisInterface) SetDiscordGameState(data *DiscordGameState, lock *redislock.Lock) {
	if data == nil {
		if lock != nil {
			//log.Println("UNLOCKING")
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
	//log.Println("unlock " + key)

	//connectCode is the 1 sole key we should ever rely on for tracking games. Because we generate it ourselves
	//randomly, it's unique to every single game, and the capture and bot BOTH agree on the linkage
	if key == "" && data.ConnectCode == "" {
		//log.Println("SET: No key found in Redis for any of the text, voice, or connect codes!")
		if lock != nil {
			//log.Println("UNLOCKING")
			lock.Release(ctx)
		}
		return
	} else {
		key = discordKey(data.GuildID, data.ConnectCode)
	}

	jBytes, err := json.Marshal(data)
	if err != nil {
		log.Println(err)
		if lock != nil {
			//log.Println("UNLOCKING")
			lock.Release(ctx)
		}
		return
	}

	//log.Printf("Setting %s to JSON", key)
	err = redisInterface.client.Set(ctx, key, jBytes, GameTimeoutSeconds*time.Second).Err()
	if err != nil {
		log.Println(err)
	}

	if lock != nil {
		//log.Println("UNLOCKING")
		lock.Release(ctx)
	}

	if data.ConnectCode != "" {
		//log.Printf("Setting %s to %s", discordKeyConnectCodePtr(guildID, data.ConnectCode), key)
		err = redisInterface.client.Set(ctx, discordKeyConnectCodePtr(data.GuildID, data.ConnectCode), key, GameTimeoutSeconds*time.Second).Err()
		if err != nil {
			log.Println(err)
		}
	}

	if data.Tracking.ChannelID != "" {
		//log.Printf("Setting %s to %s", discordKeyVoiceChannelPtr(guildID, data.Tracking.ChannelID), key)
		err = redisInterface.client.Set(ctx, discordKeyVoiceChannelPtr(data.GuildID, data.Tracking.ChannelID), key, GameTimeoutSeconds*time.Second).Err()
		if err != nil {
			log.Println(err)
		}
	}

	if data.GameStateMsg.MessageChannelID != "" {
		//log.Printf("Setting %s to %s", discordKeyTextChannelPtr(guildID, data.GameStateMsg.MessageChannelID), key)
		err = redisInterface.client.Set(ctx, discordKeyTextChannelPtr(data.GuildID, data.GameStateMsg.MessageChannelID), key, GameTimeoutSeconds*time.Second).Err()
		if err != nil {
			log.Println(err)
		}
	}
}

func (redisInterface *RedisInterface) RefreshActiveGame(guildID, connectCode string) {
	key := activeGamesKey(guildID)
	t := time.Now().Unix()
	_, err := redisInterface.client.ZAdd(ctx, key, &redis.Z{
		Score:  float64(t),
		Member: connectCode,
	}).Result()

	if err != nil {
		log.Println(err)
	}
}

func (redisInterface *RedisInterface) RemoveOldGame(guildID, connectCode string) {
	key := activeGamesKey(guildID)

	err := redisInterface.client.ZRem(ctx, key, connectCode).Err()
	if err != nil {
		log.Println(err)
	}
}

//only deletes from the guild's responsibility, NOT the entire guild counter!
func (redisInterface *RedisInterface) LoadAllActiveGames(guildID string) []string {
	hash := activeGamesKey(guildID)

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

	return games
}

func (redisInterface *RedisInterface) DeleteDiscordGameState(dgs *DiscordGameState) {
	guildID := dgs.GuildID
	connCode := dgs.ConnectCode
	if guildID == "" || connCode == "" {
		log.Println("Can't delete DGS with null guildID or null ConnCode")
	}
	data := redisInterface.getDiscordGameState(GameStateRequest{
		GuildID:     guildID,
		ConnectCode: connCode,
	})
	key := discordKey(guildID, connCode)

	locker := redislock.New(redisInterface.client)
	lock, err := locker.Obtain(ctx, key+":lock", time.Millisecond*LockTimeoutMs, &redislock.Options{
		RetryStrategy: redislock.LimitRetry(redislock.LinearBackoff(time.Millisecond*LinearBackoffMs), MaxRetries),
		Metadata:      "",
	})
	if err == redislock.ErrNotObtained {
		fmt.Println("Could not obtain lock!")
	} else if err != nil {
		log.Fatalln(err)
	} else {
		defer lock.Release(ctx)
	}

	//delete all the pointers to the underlying -actual- discord data
	err = redisInterface.client.Del(ctx, discordKeyTextChannelPtr(guildID, data.GameStateMsg.MessageChannelID)).Err()
	if err != nil {
		log.Println(err)
	}
	err = redisInterface.client.Del(ctx, discordKeyVoiceChannelPtr(guildID, data.Tracking.ChannelID)).Err()
	if err != nil {
		log.Println(err)
	}
	err = redisInterface.client.Del(ctx, discordKeyConnectCodePtr(guildID, data.ConnectCode)).Err()
	if err != nil {
		log.Println(err)
	}

	err = redisInterface.client.Del(ctx, key).Err()
	if err != nil {
		log.Println(err)
	}
}

func (redisInterface *RedisInterface) GetUsernameOrUserIDMappings(guildID, key string) map[string]interface{} {
	cacheHash := cacheHash(guildID)

	value, err := redisInterface.client.HGet(ctx, cacheHash, key).Result()
	if err != nil {
		log.Println(err)
		return map[string]interface{}{}
	}
	var ret map[string]interface{}
	err = json.Unmarshal([]byte(value), &ret)
	if err != nil {
		log.Println(err)
		return map[string]interface{}{}
	}

	//log.Println(ret)
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

	//over all the usernames associated with just this userID, delete the underlying mapping of username->userID
	usernames := redisInterface.GetUsernameOrUserIDMappings(guildID, userID)
	for username := range usernames {
		err := redisInterface.deleteHashSubEntry(guildID, username, userID)
		if err != nil {
			log.Println(err)
		}
	}

	//now delete the userID->username list entirely
	cacheHash := cacheHash(guildID)
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
	cacheHash := cacheHash(guildID)

	jBytes, err := json.Marshal(values)
	if err != nil {
		return err
	}

	return redisInterface.client.HSet(ctx, cacheHash, key, jBytes).Err()
}

func (redisInterface *RedisInterface) LockSnowflake(snowflake string) *redislock.Lock {
	locker := redislock.New(redisInterface.client)
	lock, err := locker.Obtain(ctx, snowflakeLockID(snowflake), time.Millisecond*SnowflakeLockMs, nil)
	if err == redislock.ErrNotObtained {
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

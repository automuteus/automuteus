package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/bsm/redislock"
	"github.com/denverquane/amongusdiscord/storage"
	"github.com/go-redis/redis/v8"
)

var ctx = context.Background()

const LockTimeoutSecs = 3
const LinearBackoffMs = 200
const MaxRetries = 10

const SecsPerHour = 3600

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

func versionKey() string {
	return "automuteus:version"
}

func totalGuildsKey(version string) string {
	return "automuteus:count:guilds:version-" + version
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

func secretKeysHash(guildID string) string {
	return "automuteus:discord:" + guildID + ":secretkeys"
}

func matchIDKey() string {
	return "automuteus:match:counter"
}

func (redisInterface *RedisInterface) GetAndIncrementMatchID() int64 {
	num, err := redisInterface.client.Incr(ctx, matchIDKey()).Result()
	if err != nil {
		log.Println(err)
	}
	return num
}

func (redisInterface *RedisInterface) SetVersion(version string) {
	err := redisInterface.client.Set(ctx, versionKey(), version, 0).Err()
	if err != nil {
		log.Println(err)
	}
}

func (redisInterface *RedisInterface) GetVersion() string {
	v, err := redisInterface.client.Get(ctx, versionKey()).Result()
	if err != nil {
		log.Println(err)
	}
	return v
}

func (redisInterface *RedisInterface) AddUniqueGuildCounter(guildID, version string) {
	_, err := redisInterface.client.SAdd(ctx, totalGuildsKey(version), string(storage.HashGuildID(guildID))).Result()
	if err != nil {
		log.Println(err)
	}
}

func (redisInterface *RedisInterface) LeaveUniqueGuildCounter(guildID, version string) {
	_, err := redisInterface.client.SRem(ctx, totalGuildsKey(version), string(storage.HashGuildID(guildID))).Result()
	if err != nil {
		log.Println(err)
	}
}

func (redisInterface *RedisInterface) GetGuildCounter(version string) int64 {
	count, err := redisInterface.client.SCard(ctx, totalGuildsKey(version)).Result()
	if err != nil {
		log.Println(err)
		return 0
	}
	return count
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
	lock, err := locker.Obtain(ctx, key+":lock", time.Second*LockTimeoutSecs, &redislock.Options{
		RetryStrategy: redislock.LimitRetry(redislock.LinearBackoff(time.Millisecond*LinearBackoffMs), MaxRetries),
		Metadata:      "",
	})
	if err == redislock.ErrNotObtained {
		return nil, nil
	} else if err != nil {
		log.Println(err)
		return nil, nil
	}
	//log.Println("LOCKING " + key)

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
	err = redisInterface.client.Set(ctx, key, jBytes, SecsPerHour*time.Second).Err()
	if err != nil {
		log.Println(err)
	}

	if lock != nil {
		//log.Println("UNLOCKING")
		lock.Release(ctx)
	}

	if data.ConnectCode != "" {
		//log.Printf("Setting %s to %s", discordKeyConnectCodePtr(guildID, data.ConnectCode), key)
		err = redisInterface.client.Set(ctx, discordKeyConnectCodePtr(data.GuildID, data.ConnectCode), key, SecsPerHour*time.Second).Err()
		if err != nil {
			log.Println(err)
		}
	}

	if data.Tracking.ChannelID != "" {
		//log.Printf("Setting %s to %s", discordKeyVoiceChannelPtr(guildID, data.Tracking.ChannelID), key)
		err = redisInterface.client.Set(ctx, discordKeyVoiceChannelPtr(data.GuildID, data.Tracking.ChannelID), key, SecsPerHour*time.Second).Err()
		if err != nil {
			log.Println(err)
		}
	}

	if data.GameStateMsg.MessageChannelID != "" {
		//log.Printf("Setting %s to %s", discordKeyTextChannelPtr(guildID, data.GameStateMsg.MessageChannelID), key)
		err = redisInterface.client.Set(ctx, discordKeyTextChannelPtr(data.GuildID, data.GameStateMsg.MessageChannelID), key, SecsPerHour*time.Second).Err()
		if err != nil {
			log.Println(err)
		}
	}
}

func (redisInterface *RedisInterface) AppendToActiveGames(guildID, connectCode string) {
	key := activeGamesKey(guildID)

	count, err := redisInterface.client.SAdd(ctx, key, connectCode).Result()

	if err != nil {
		log.Println(err)
	} else {
		log.Printf("Active games: %d", count)
	}
}

func (redisInterface *RedisInterface) RemoveOldGame(guildID, connectCode string) {
	key := activeGamesKey(guildID)

	err := redisInterface.client.SRem(ctx, key, connectCode).Err()
	if err != nil {
		log.Println(err)
	}
}

//only deletes from the guild's responsibility, NOT the entire guild counter!
func (redisInterface *RedisInterface) LoadAllActiveGamesAndDelete(guildID string) []string {
	hash := activeGamesKey(guildID)

	games, err := redisInterface.client.SMembers(ctx, hash).Result()
	if err != nil {
		log.Println(err)
		return []string{}
	}

	_, err = redisInterface.client.Del(ctx, hash).Result()
	if err != nil {
		log.Println(err)
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
	lock, err := locker.Obtain(ctx, key+":lock", LockTimeoutSecs*time.Second, &redislock.Options{
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

// => secretKey zone

func (redisInterface *RedisInterface) AddSecretKey(guildID, userID, secretKeys string) error {
	return redisInterface.appendToSecretKeyEntry(guildID, userID, secretKeys)
}

func (redisInterface *RedisInterface) DeleteSecretKeysByUserID(guildID, userID string) error {
	secretKeysHash := secretKeysHash(guildID)
	return redisInterface.client.HDel(ctx, secretKeysHash, userID).Err()
}

func (redisInterface *RedisInterface) GetSecretKeysMappings(guildID, userID string) map[string]interface{} {
	secretKeysHash := secretKeysHash(guildID)

	value, err := redisInterface.client.HGet(ctx, secretKeysHash, userID).Result()
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

	log.Println(ret)
	return ret
}

func (redisInterface *RedisInterface) appendToSecretKeyEntry(guildID, userID, value string) error {
	resp := redisInterface.GetSecretKeysMappings(guildID, userID)

	resp[value] = struct{}{}

	return redisInterface.setSecretKeysMappings(guildID, userID, resp)
}

func (redisInterface *RedisInterface) setSecretKeysMappings(guildID, key string, values map[string]interface{}) error {
	secretKeysHash := secretKeysHash(guildID)

	jBytes, err := json.Marshal(values)
	if err != nil {
		return err
	}

	return redisInterface.client.HSet(ctx, secretKeysHash, key, jBytes).Err()
}

// <= end secretKey zone

func (redisInterface *RedisInterface) Close() error {
	return redisInterface.client.Close()
}

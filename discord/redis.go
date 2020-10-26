package discord

import (
	"context"
	"encoding/json"
	"github.com/denverquane/amongusdiscord/storage"
	"github.com/go-redis/redis/v8"
	"log"
)

var ctx = context.Background()

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

func lobbyUpdateKey(connCode string) string {
	return gameKey(connCode) + ":events:lobby"
}

func phaseUpdateKey(connCode string) string {
	return gameKey(connCode) + ":events:phase"
}

func playerUpdateKey(connCode string) string {
	return gameKey(connCode) + ":events:player"
}

func connectUpdateKey(connCode string) string {
	return gameKey(connCode) + ":events:connect"
}

func gameKey(connCode string) string {
	return "automuteus:game:" + connCode
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

func usernameCacheHash(guildID, username string) string {
	return "automuteus:discord:" + guildID + ":username:" + username
}

func (redisInterface *RedisInterface) PublishLobbyUpdate(connectCode, lobbyJson string) {
	redisInterface.publish(lobbyUpdateKey(connectCode), lobbyJson)
}

func (redisInterface *RedisInterface) PublishPhaseUpdate(connectCode, phase string) {
	redisInterface.publish(phaseUpdateKey(connectCode), phase)
}

func (redisInterface *RedisInterface) PublishPlayerUpdate(connectCode, playerJson string) {
	redisInterface.publish(playerUpdateKey(connectCode), playerJson)
}

func (redisInterface *RedisInterface) PublishConnectUpdate(connectCode, connect string) {
	redisInterface.publish(connectUpdateKey(connectCode), connect)
}

func (redisInterface *RedisInterface) publish(topic, message string) {
	log.Printf("Publishing %s to %s\n", message, topic)
	err := redisInterface.client.Publish(ctx, topic, message).Err()
	if err != nil {
		log.Println(err)
	}
}

func (redisInterface *RedisInterface) SubscribeToGame(connectCode string) (connection, lobby, phase, player *redis.PubSub) {
	connect := redisInterface.client.Subscribe(ctx, connectUpdateKey(connectCode))
	lob := redisInterface.client.Subscribe(ctx, lobbyUpdateKey(connectCode))
	phas := redisInterface.client.Subscribe(ctx, phaseUpdateKey(connectCode))
	play := redisInterface.client.Subscribe(ctx, playerUpdateKey(connectCode))

	return connect, lob, phas, play
}

//need at least one of these fields to fetch
func (redisInterface *RedisInterface) GetDiscordGameState(guildID, textChannel, voiceChannel, connectCode string) *DiscordGameState {
	key := redisInterface.CheckPointer(discordKeyConnectCodePtr(guildID, connectCode))
	if key == "" {
		key = redisInterface.CheckPointer(discordKeyTextChannelPtr(guildID, textChannel))
		if key == "" {
			key = redisInterface.CheckPointer(discordKeyVoiceChannelPtr(guildID, voiceChannel))
		}
	}

	jsonStr, err := redisInterface.client.Get(ctx, key).Result()
	if err == redis.Nil {
		dgs := NewDiscordGameState(guildID)
		//this is a little silly, but it works...
		dgs.ConnectCode = connectCode
		//dgs.GameStateMsg.MessageChannelID = textChannel
		dgs.Tracking.ChannelID = voiceChannel
		redisInterface.SetDiscordGameState(guildID, dgs)
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

func (redisInterface *RedisInterface) SetDiscordGameState(guildID string, data *DiscordGameState) {
	if data == nil || !data.NeedsUpload {
		return
	}
	key := redisInterface.CheckPointer(discordKeyConnectCodePtr(guildID, data.ConnectCode))
	if key == "" {
		key = redisInterface.CheckPointer(discordKeyTextChannelPtr(guildID, data.GameStateMsg.MessageChannelID))
		if key == "" {
			key = redisInterface.CheckPointer(discordKeyVoiceChannelPtr(guildID, data.Tracking.ChannelID))
		}
	}

	//connectCode is the 1 sole key we should ever rely on for tracking games. Because we generate it ourselves
	//randomly, it's unique to every single game, and the capture and bot BOTH agree on the linkage
	if key == "" && data.ConnectCode == "" {
		//log.Println("SET: No key found in Redis for any of the text, voice, or connect codes!")
		return
	} else {
		key = discordKey(guildID, data.ConnectCode)
	}

	data.NeedsUpload = false

	jBytes, err := json.Marshal(data)
	if err != nil {
		log.Println(err)
		return
	}

	log.Printf("Setting %s to JSON", key)
	err = redisInterface.client.Set(ctx, key, jBytes, 0).Err()
	if err != nil {
		log.Println(err)
	}

	if data.ConnectCode != "" {
		log.Printf("Setting %s to %s", discordKeyConnectCodePtr(guildID, data.ConnectCode), key)
		err = redisInterface.client.Set(ctx, discordKeyConnectCodePtr(guildID, data.ConnectCode), key, 0).Err()
		if err != nil {
			log.Println(err)
		}
	}

	if data.Tracking.ChannelID != "" {
		log.Printf("Setting %s to %s", discordKeyVoiceChannelPtr(guildID, data.Tracking.ChannelID), key)
		err = redisInterface.client.Set(ctx, discordKeyVoiceChannelPtr(guildID, data.Tracking.ChannelID), key, 0).Err()
		if err != nil {
			log.Println(err)
		}
	}

	if data.GameStateMsg.MessageChannelID != "" {
		log.Printf("Setting %s to %s", discordKeyTextChannelPtr(guildID, data.GameStateMsg.MessageChannelID), key)
		err = redisInterface.client.Set(ctx, discordKeyTextChannelPtr(guildID, data.GameStateMsg.MessageChannelID), key, 0).Err()
		if err != nil {
			log.Println(err)
		}
	}
}

func (redisInterface *RedisInterface) DeleteDiscordGameState(dgs *DiscordGameState) {
	guildID := dgs.GuildID
	connCode := dgs.ConnectCode
	if guildID == "" || connCode == "" {
		log.Println("Can't delete DGS with null guildID or null ConnCode")
	}
	data := redisInterface.GetDiscordGameState(guildID, "", "", connCode)
	key := discordKey(guildID, connCode)

	//delete all the pointers to the underlying -actual- discord data
	err := redisInterface.client.Del(ctx, discordKeyTextChannelPtr(guildID, data.GameStateMsg.MessageChannelID)).Err()
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

func (redisInterface *RedisInterface) GetUidMappings(guildID, username string) []string {
	hash := usernameCacheHash(guildID, username)

	uids, err := redisInterface.client.HKeys(ctx, hash).Result()
	log.Println(uids)
	if err != nil && err != redis.Nil {
		log.Println(err)
		return []string{}
	}
	return uids
}

func (redisInterface *RedisInterface) AddUsernameLink(guildID, userID, userName string) error {
	hash := usernameCacheHash(guildID, userName)

	log.Println("Associating " + userID + " with " + userName + " and SET in Redis")

	return redisInterface.client.HSet(ctx, hash, userID, "").Err()
}

func (redisInterface *RedisInterface) Close() error {
	return redisInterface.client.Close()
}

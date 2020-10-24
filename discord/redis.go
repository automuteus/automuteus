package discord

import (
	"context"
	"encoding/json"
	"github.com/denverquane/amongusdiscord/game"
	"github.com/denverquane/amongusdiscord/storage"
	"github.com/go-redis/redis/v8"
	"log"
)

var ctx = context.Background()

type DatabaseInterface struct {
	client *redis.Client
}

type RedisParameters struct {
	Addr     string
	Username string
	Password string
}

func (rd *DatabaseInterface) Init(params interface{}) error {
	redisParams := params.(RedisParameters)
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisParams.Addr,
		Username: redisParams.Username,
		Password: redisParams.Password,
		DB:       0, // use default DB
	})
	rd.client = rdb
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
	return "amongusdiscord:game:" + connCode
}

func discordKeyTextChannelPtr(guildID, channelID string) string {
	return "amongusdiscord:discord:" + guildID + ":pointer:text:" + channelID
}

func discordKeyVoiceChannelPtr(guildID, channelID string) string {
	return "amongusdiscord:discord:" + guildID + ":pointer:voice:" + channelID
}

func discordKeyConnectCodePtr(guildID, code string) string {
	return "amongusdiscord:discord:" + guildID + ":pointer:code:" + code
}

func discordKey(guildID, id string) string {
	return "amongusdiscord:discord:" + guildID + ":" + id
}

func (rd *DatabaseInterface) PublishLobbyUpdate(connectCode, lobbyJson string) {
	rd.publish(lobbyUpdateKey(connectCode), lobbyJson)
}

func (rd *DatabaseInterface) PublishPhaseUpdate(connectCode, phase string) {
	rd.publish(phaseUpdateKey(connectCode), phase)
}

func (rd *DatabaseInterface) PublishPlayerUpdate(connectCode, playerJson string) {
	rd.publish(playerUpdateKey(connectCode), playerJson)
}

func (rd *DatabaseInterface) PublishConnectUpdate(connectCode, connect string) {
	rd.publish(connectUpdateKey(connectCode), connect)
}

func (rd *DatabaseInterface) publish(topic, message string) {
	log.Printf("Publishing %s to %s\n", message, topic)
	err := rd.client.Publish(ctx, topic, message).Err()
	if err != nil {
		log.Println(err)
	}
}

func (rd *DatabaseInterface) SubscribeToGame(connectCode string) (connection, lobby, phase, player <-chan *redis.Message) {
	connect := rd.client.Subscribe(ctx, connectUpdateKey(connectCode))
	lob := rd.client.Subscribe(ctx, lobbyUpdateKey(connectCode))
	phas := rd.client.Subscribe(ctx, phaseUpdateKey(connectCode))
	play := rd.client.Subscribe(ctx, playerUpdateKey(connectCode))

	return connect.Channel(), lob.Channel(), phas.Channel(), play.Channel()
}

func (di *DatabaseInterface) GetAmongUsData(connectCode string) *game.AmongUsData {
	key := gameKey(connectCode)
	jsonStr, err := di.client.Get(ctx, key).Result()
	if err == redis.Nil {
		aud := game.NewAmongUsData()
		jBytes, err := json.Marshal(&aud)
		if err != nil {
			log.Println(err)
			return nil
		} else {
			err := di.client.Set(ctx, key, jBytes, 0).Err()
			if err != nil {
				log.Println(err)
				return nil
			} else {
				return &aud
			}
		}
	} else if err != nil {
		log.Println(err)
		return nil
	} else {
		aud := game.AmongUsData{}
		err := json.Unmarshal([]byte(jsonStr), &aud)
		if err != nil {
			log.Println(err)
			return nil
		} else {
			return &aud
		}
	}
}

func (di *DatabaseInterface) SetAmongUsData(connectCode string, data *game.AmongUsData) {
	key := gameKey(connectCode)
	jBytes, err := json.Marshal(data)
	if err != nil {
		log.Println(err)
		return
	}
	err = di.client.Set(ctx, key, jBytes, 0).Err()
	if err != nil {
		log.Println(err)
	}
}

//need at least one of these fields to fetch
func (di *DatabaseInterface) GetDiscordGameState(guildID, textChannel, voiceChannel, connectCode string) *DiscordGameState {
	key := di.CheckPointer(discordKeyConnectCodePtr(guildID, connectCode))
	if key == "" {
		key = di.CheckPointer(discordKeyTextChannelPtr(guildID, textChannel))
		if key == "" {
			key = di.CheckPointer(discordKeyVoiceChannelPtr(guildID, voiceChannel))
		}
	}

	jsonStr, err := di.client.Get(ctx, key).Result()
	if err == redis.Nil {
		dgs := NewDiscordGameState(guildID)
		//this is a little silly, but it works...
		dgs.ConnectCode = connectCode
		dgs.GameStateMsg.MessageChannelID = textChannel
		dgs.Tracking.ChannelID = voiceChannel
		di.SetDiscordGameState(guildID, dgs)
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

func (di *DatabaseInterface) CheckPointer(pointer string) string {
	key, err := di.client.Get(ctx, pointer).Result()
	if err != nil {
		return ""
	} else {
		return key
	}
}

func (di *DatabaseInterface) SetDiscordGameState(guildID string, data *DiscordGameState) {
	if data == nil {
		return
	}
	key := di.CheckPointer(discordKeyConnectCodePtr(guildID, data.ConnectCode))
	if key == "" {
		key = di.CheckPointer(discordKeyTextChannelPtr(guildID, data.GameStateMsg.MessageChannelID))
		if key == "" {
			key = di.CheckPointer(discordKeyVoiceChannelPtr(guildID, data.Tracking.ChannelID))
		}
	}

	//connectCode is the 1 sole key we should ever rely on for tracking games. Because we generate it ourselves
	//randomly, it's unique to every single game, and the capture and bot both agree on the linkage
	if key == "" && data.ConnectCode == "" {
		log.Println("SET: No key found in Redis for any of the text, voice, or connect codes!")
		return
	} else {
		key = discordKey(guildID, data.ConnectCode)
	}

	jBytes, err := json.Marshal(data)
	if err != nil {
		log.Println(err)
		return
	}

	log.Printf("Setting %s to JSON", key)
	err = di.client.Set(ctx, key, jBytes, 0).Err()
	if err != nil {
		log.Println(err)
	}

	if data.ConnectCode != "" {
		log.Printf("Setting %s to %s", discordKeyConnectCodePtr(guildID, data.ConnectCode), key)
		err = di.client.Set(ctx, discordKeyConnectCodePtr(guildID, data.ConnectCode), key, 0).Err()
		if err != nil {
			log.Println(err)
		}
	}

	if data.Tracking.ChannelID != "" {
		log.Printf("Setting %s to %s", discordKeyVoiceChannelPtr(guildID, data.Tracking.ChannelID), key)
		err = di.client.Set(ctx, discordKeyVoiceChannelPtr(guildID, data.Tracking.ChannelID), key, 0).Err()
		if err != nil {
			log.Println(err)
		}
	}

	if data.GameStateMsg.MessageChannelID != "" {
		log.Printf("Setting %s to %s", discordKeyTextChannelPtr(guildID, data.GameStateMsg.MessageChannelID), key)
		err = di.client.Set(ctx, discordKeyTextChannelPtr(guildID, data.GameStateMsg.MessageChannelID), key, 0).Err()
		if err != nil {
			log.Println(err)
		}
	}
}

func (di *DatabaseInterface) GetDiscordSettings(guildID string) *storage.GuildSettings {
	return storage.MakeGuildSettings(guildID, "")
}

func (rd *DatabaseInterface) Close() error {
	return rd.client.Close()
}

package discord

import (
	"context"
	galactus_client "github.com/automuteus/galactus/pkg/client"
	"github.com/automuteus/galactus/pkg/discord_message"
	"github.com/automuteus/utils/pkg/game"
	"github.com/automuteus/utils/pkg/rediskey"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/amongus"
	"github.com/denverquane/amongusdiscord/storage"
	"go.uber.org/zap"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Bot struct {
	url string

	// mapping of socket connections to the game connect codes
	ConnsToGames map[string]string

	StatusEmojis AlivenessEmojis

	EndGameChannels map[string]chan EndGameMessage

	ChannelsMapLock sync.RWMutex

	GalactusClient *galactus_client.GalactusClient

	RedisInterface *RedisInterface

	StorageInterface *storage.StorageInterface

	PostgresInterface *storage.PsqlInterface

	logger *zap.Logger

	captureTimeout int

	globalPrefix  string
	defaultPrefix string
}

func MakeAndStartBot(url, emojiGuildID string,
	redisInterface *RedisInterface,
	storageInterface *storage.StorageInterface,
	psql *storage.PsqlInterface,
	gc *galactus_client.GalactusClient,
	logger *zap.Logger) *Bot {

	bot := Bot{
		url:          url,
		ConnsToGames: make(map[string]string),
		StatusEmojis: emptyStatusEmojis(),

		EndGameChannels:   make(map[string]chan EndGameMessage),
		ChannelsMapLock:   sync.RWMutex{},
		GalactusClient:    gc,
		RedisInterface:    redisInterface,
		StorageInterface:  storageInterface,
		PostgresInterface: psql,
		logger:            logger,
		captureTimeout:    GameTimeoutSeconds,
	}

	bot.GalactusClient.RegisterDiscordHandler(discord_message.MessageCreate, bot.handleMessageCreate)
	bot.GalactusClient.RegisterDiscordHandler(discord_message.VoiceStateUpdate, bot.handleVoiceStateChange)
	bot.GalactusClient.RegisterDiscordHandler(discord_message.GuildDelete, bot.leaveGuild)
	bot.GalactusClient.RegisterDiscordHandler(discord_message.MessageReactionAdd, bot.handleReactionGameStartAdd)
	bot.GalactusClient.RegisterDiscordHandler(discord_message.GuildCreate, bot.handleNewGuild)

	err := bot.GalactusClient.StartDiscordPolling()
	if err != nil {
		log.Println(err)
	}

	bot.ensureEmojisExist(emojiGuildID)

	go StartHealthCheckServer("8080")

	//listeningTo := os.Getenv("AUTOMUTEUS_LISTENING")
	//if listeningTo == "" {
	//	prefix := os.Getenv("AUTOMUTEUS_GLOBAL_PREFIX")
	//	if prefix == "" {
	//		prefix = ".au"
	//	}
	//
	//	listeningTo = prefix + " help"
	//}

	//status := &discordgo.UpdateStatusData{
	//	IdleSince: nil,
	//	Game: &discordgo.Game{
	//		Name: listeningTo,
	//		Type: discordgo.GameTypeListening,
	//	},
	//	AFK:    false,
	//	Status: "",
	//}
	//err = dg.UpdateStatusComplex(*status)
	//if err != nil {
	//	log.Println(err)
	//}

	// indicate to Kubernetes that we're ready to start receiving traffic
	GlobalReady = true

	// TODO this is ugly. Should make a proper cronjob to refresh the stats regularly
	go bot.statsRefreshWorker(rediskey.TotalUsersExpiration)

	return &bot
}

func (bot *Bot) statsRefreshWorker(dur time.Duration) {
	for {
		users := rediskey.GetTotalUsers(context.Background(), bot.RedisInterface.client)
		if users == rediskey.NotFound {
			log.Println("Refreshing user stats with worker")
			rediskey.RefreshTotalUsers(context.Background(), bot.RedisInterface.client, bot.PostgresInterface.Pool)
		}

		games := rediskey.GetTotalGames(context.Background(), bot.RedisInterface.client)
		if games == rediskey.NotFound {
			log.Println("Refreshing game stats with worker")
			rediskey.RefreshTotalGames(context.Background(), bot.RedisInterface.client, bot.PostgresInterface.Pool)
		}

		time.Sleep(dur)
	}
}

func (bot *Bot) Close() {
	bot.GalactusClient.StopAllPolling()
	bot.RedisInterface.Close()
	bot.StorageInterface.Close()
}

var EmojiLock = sync.Mutex{}
var AllEmojisStartup []*discordgo.Emoji = nil

func (bot *Bot) ensureEmojisExist(emojiGuildID string) {
	if emojiGuildID == "" {
		return
	}
	EmojiLock.Lock()
	if AllEmojisStartup == nil {
		log.Printf("Adding any missing emojis to guild %s. "+
			"On first startup, this can take a long time to complete (Discord's rate-limits on adding emojis)", emojiGuildID)
		allEmojis, err := bot.GalactusClient.GetGuildEmojis(emojiGuildID)
		if err != nil {
			log.Println(err)
		} else {
			bot.addAllMissingEmojis(bot.GalactusClient, emojiGuildID, true, allEmojis)
			bot.addAllMissingEmojis(bot.GalactusClient, emojiGuildID, false, allEmojis)

			AllEmojisStartup = allEmojis
			log.Println("Emojis added and verified successfully")
		}
	}
	EmojiLock.Unlock()
}

func (bot *Bot) handleNewGuild(m discordgo.GuildCreate) {
	gid, err := strconv.ParseUint(m.Guild.ID, 10, 64)
	if err != nil {
		log.Println(err)
	}
	go bot.PostgresInterface.EnsureGuildExists(gid, m.Guild.Name)

	log.Printf("Added to new Guild, id %s, name %s", m.Guild.ID, m.Guild.Name)
	bot.RedisInterface.AddUniqueGuildCounter(m.Guild.ID)

	bot.ensureEmojisExist(m.Guild.ID)

	games := bot.RedisInterface.LoadAllActiveGames(m.Guild.ID)

	for _, connCode := range games {
		gsr := GameStateRequest{
			GuildID:     m.Guild.ID,
			ConnectCode: connCode,
		}
		lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
		for lock == nil {
			lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
		}
		if dgs != nil && dgs.ConnectCode != "" {
			log.Println("Resubscribing to Redis events for an old game: " + connCode)
			killChan := make(chan EndGameMessage)
			go bot.SubscribeToGameByConnectCode(gsr.GuildID, dgs.ConnectCode, killChan)
			dgs.Subscribed = true

			bot.RedisInterface.SetDiscordGameState(dgs, lock)

			bot.ChannelsMapLock.Lock()
			bot.EndGameChannels[dgs.ConnectCode] = killChan
			bot.ChannelsMapLock.Unlock()
		}
		lock.Release(ctx)
	}

}

func (bot *Bot) leaveGuild(m discordgo.GuildDelete) {
	log.Println("Bot was removed from Guild " + m.ID)
	bot.RedisInterface.LeaveUniqueGuildCounter(m.ID)

	err := bot.StorageInterface.DeleteGuildSettings(m.ID)
	if err != nil {
		log.Println(err)
	}
}

func (bot *Bot) linkPlayer(galactus *galactus_client.GalactusClient, g *discordgo.Guild, dgs *GameState, args []string) {
	userID, err := extractUserIDFromMention(args[0])
	if userID == "" || err != nil {
		log.Printf("Sorry, I don't know who `%s` is. You can pass in ID, username, username#XXXX, nickname or @mention", args[0])
	}

	_, added := dgs.checkCacheAndAddUser(g, galactus, userID)
	if !added {
		log.Println("No users found in Discord for UserID " + userID)
	}

	combinedArgs := strings.ToLower(strings.Join(args[1:], ""))
	var auData amongus.PlayerData
	found := false
	if game.IsColorString(combinedArgs) {
		auData, found = dgs.AmongUsData.GetByColor(combinedArgs)
	} else {
		auData, found = dgs.AmongUsData.GetByName(combinedArgs)
	}
	if found {
		foundID := dgs.AttemptPairingByUserIDs(auData, map[string]interface{}{userID: ""})
		if foundID != "" {
			log.Printf("Successfully linked %s to a color\n", userID)
			err := bot.RedisInterface.AddUsernameLink(dgs.GuildID, userID, auData.Name)
			if err != nil {
				log.Println(err)
			}
		} else {
			log.Printf("No player was found with id %s\n", userID)
		}
	}
}

func (bot *Bot) forceEndGame(gsr GameStateRequest) {
	// lock because we don't want anyone else modifying while we delete
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)

	for lock == nil {
		lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
	}
	bot.GalactusClient.StopCapturePolling(dgs.ConnectCode)

	dgs.DeleteGameStateMsg(bot.GalactusClient)

	bot.RedisInterface.SetDiscordGameState(dgs, lock)

	bot.RedisInterface.RemoveOldGame(dgs.GuildID, dgs.ConnectCode)

	// Note, this shouldn't be necessary with the TTL of the keys, but it can't hurt to clean up...
	bot.RedisInterface.DeleteDiscordGameState(dgs)
}

func MessageDeleteWorker(galactus *galactus_client.GalactusClient, msgChannelID, msgID string, waitDur time.Duration) {
	log.Printf("Message worker is sleeping for %s before deleting message", waitDur.String())
	time.Sleep(waitDur)
	galactus.DeleteChannelMessage(msgChannelID, msgID)
}

func (bot *Bot) RefreshGameStateMessage(gsr GameStateRequest, sett *settings.GuildSettings) {
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
	for lock == nil {
		lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
	}
	// log.Println("Refreshing game state message")

	// don't try to edit this message, because we're about to delete it
	RemovePendingDGSEdit(dgs.GameStateMsg.MessageID)

	if dgs.GameStateMsg.MessageChannelID != "" && dgs.GameStateMsg.MessageID != "" {
		dgs.DeleteGameStateMsg(bot.GalactusClient) // delete the old message
		dgs.CreateMessage(bot.GalactusClient, bot.gameStateResponse(dgs, sett), dgs.GameStateMsg.MessageChannelID, dgs.GameStateMsg.LeaderID)
	}

	bot.RedisInterface.SetDiscordGameState(dgs, lock)

	// add the emojis to the refreshed message
	if dgs.GameStateMsg.MessageChannelID != "" && dgs.GameStateMsg.MessageID != "" {
		dgs.AddReaction(bot.GalactusClient, "▶️")
	}
}

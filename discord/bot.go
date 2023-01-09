package discord

import (
	"context"
	"errors"
	"fmt"
	"github.com/automuteus/automuteus/amongus"
	"github.com/automuteus/automuteus/discord/command"
	"github.com/automuteus/automuteus/metrics"
	"github.com/automuteus/automuteus/storage"
	"github.com/automuteus/utils/pkg/discord"
	"github.com/automuteus/utils/pkg/game"
	"github.com/automuteus/utils/pkg/premium"
	"github.com/automuteus/utils/pkg/rediskey"
	"github.com/automuteus/utils/pkg/settings"
	storageutils "github.com/automuteus/utils/pkg/storage"
	"github.com/automuteus/utils/pkg/token"
	"github.com/bwmarrin/discordgo"
	"github.com/top-gg/go-dbl"
	"log"
	"os"
	"strconv"
	"sync"
	"time"
)

type Bot struct {
	official bool
	url      string

	// mapping of socket connections to the game connect codes
	ConnsToGames map[string]string

	StatusEmojis AlivenessEmojis

	EndGameChannels map[string]chan EndGameMessage

	ChannelsMapLock sync.RWMutex

	PrimarySession *discordgo.Session

	GalactusClient *GalactusClient

	TopGGClient *dbl.Client

	RedisInterface *RedisInterface

	StorageInterface *storage.StorageInterface

	PostgresInterface *storageutils.PsqlInterface

	logPath string

	captureTimeout int
}

// MakeAndStartBot does what it sounds like
// TODO collapse these fields into proper structs?
func MakeAndStartBot(version, commit, botToken, topGGToken, url, emojiGuildID string, numShards, shardID int, redisInterface *RedisInterface, storageInterface *storage.StorageInterface, psql *storageutils.PsqlInterface, gc *GalactusClient, logPath string) *Bot {
	dg, err := discordgo.New("Bot " + botToken)
	if err != nil {
		log.Println("error creating Discord session,", err)
		return nil
	}

	if numShards > 1 {
		log.Printf("Identifying to the Discord API with %d total shards, and shard ID=%d\n", numShards, shardID)
		dg.ShardCount = numShards
		dg.ShardID = shardID
	}

	bot := Bot{
		official:     os.Getenv("AUTOMUTEUS_OFFICIAL") != "",
		url:          url,
		ConnsToGames: make(map[string]string),
		StatusEmojis: emptyStatusEmojis(),

		EndGameChannels:   make(map[string]chan EndGameMessage),
		ChannelsMapLock:   sync.RWMutex{},
		PrimarySession:    dg,
		GalactusClient:    gc,
		RedisInterface:    redisInterface,
		StorageInterface:  storageInterface,
		PostgresInterface: psql,
		logPath:           logPath,
		captureTimeout:    GameTimeoutSeconds,
	}
	dg.LogLevel = discordgo.LogInformational

	dg.AddHandler(bot.handleVoiceStateChange)
	dg.AddHandler(bot.newGuild(emojiGuildID))
	dg.AddHandler(bot.leaveGuild)
	dg.AddHandler(bot.rateLimitEventCallback)
	// Slash commands
	dg.AddHandler(bot.handleInteractionCreate)

	dg.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Println("Bot is now online according to discord Ready handler")
	})

	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildVoiceStates | discordgo.IntentsGuilds | discordgo.IntentsGuildMessages)

	token.WaitForToken(bot.RedisInterface.client, botToken)
	token.LockForToken(bot.RedisInterface.client, botToken)
	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		log.Println("Could not connect Bot to the Discord Servers with error:", err)
		return nil
	}

	rediskey.SetVersionAndCommit(context.Background(), bot.RedisInterface.client, version, commit)

	nodeID := os.Getenv("SCW_NODE_ID")
	go metrics.PrometheusMetricsServer(bot.RedisInterface.client, nodeID, "2112")

	go metrics.StartHealthCheckServer("8080")

	log.Println("Finished identifying to the Discord API. Now ready for incoming events")

	listeningTo := os.Getenv("AUTOMUTEUS_LISTENING")
	if listeningTo == "" {
		listeningTo = "/help"
	}

	status := &discordgo.UpdateStatusData{
		IdleSince: nil,
		Activities: []*discordgo.Activity{&discordgo.Activity{
			Name: listeningTo,
			Type: discordgo.ActivityTypeListening,
		}},
		AFK:    false,
		Status: "",
	}
	err = dg.UpdateStatusComplex(*status)
	if err != nil {
		log.Println(err)
	}

	// indicate to Kubernetes that we're ready to start receiving traffic
	metrics.GlobalReady = true

	if topGGToken != "" {
		dblClient, err := dbl.NewClient(topGGToken)
		if err != nil {
			log.Println("Error creating Top.gg client: ", err)
		}
		bot.TopGGClient = dblClient
	} else {
		log.Println("No TOP_GG_TOKEN provided")
	}

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
	bot.PrimarySession.Close()
	bot.RedisInterface.Close()
	bot.StorageInterface.Close()
}

var EmojiLock = sync.Mutex{}
var AllEmojisStartup []*discordgo.Emoji = nil

func (bot *Bot) newGuild(emojiGuildID string) func(s *discordgo.Session, m *discordgo.GuildCreate) {
	return func(s *discordgo.Session, m *discordgo.GuildCreate) {
		gid, err := strconv.ParseUint(m.Guild.ID, 10, 64)
		if err != nil {
			log.Println(err)
		}

		go func() {
			guild, err := bot.PostgresInterface.EnsureGuildExists(gid, m.Guild.Name)
			if err != nil {
				log.Println(err)
			} else if guild != nil {
				err = bot.GalactusClient.VerifyPremiumMembership(guild.GuildID, premium.Tier(guild.Premium))
				if err != nil {
					log.Println(err)
				}
			}
		}()

		log.Printf("Added to new Guild, id %s, name %s", m.Guild.ID, m.Guild.Name)
		bot.RedisInterface.AddUniqueGuildCounter(m.Guild.ID)

		if emojiGuildID == "" {
			log.Println("[This is not an error] No explicit guildID provided for emojis; using the current guild default")
			emojiGuildID = m.Guild.ID
		}

		// TODO make the emoji guild ID mandatory
		EmojiLock.Lock()
		if AllEmojisStartup == nil {
			allEmojis, err := s.GuildEmojis(emojiGuildID)
			if err != nil {
				log.Println(err)
			} else {
				bot.addAllMissingEmojis(s, m.Guild.ID, true, allEmojis)
				bot.addAllMissingEmojis(s, m.Guild.ID, false, allEmojis)

				// if we specified the guild ID, then any subsequent guilds should just use the existing emojis
				if os.Getenv("EMOJI_GUILD_ID") != "" {
					AllEmojisStartup = allEmojis
					log.Println("Skipping subsequent guilds; emojis added successfully")
				}
			}
		} else {
			bot.addAllMissingEmojis(s, m.Guild.ID, true, AllEmojisStartup)

			bot.addAllMissingEmojis(s, m.Guild.ID, false, AllEmojisStartup)
		}
		EmojiLock.Unlock()

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
}

func (bot *Bot) leaveGuild(_ *discordgo.Session, m *discordgo.GuildDelete) {
	log.Println("Bot was removed from Guild " + m.ID)
	bot.RedisInterface.LeaveUniqueGuildCounter(m.ID)

	err := bot.StorageInterface.DeleteGuildSettings(m.ID)
	if err != nil {
		log.Println(err)
	}
}

func (bot *Bot) forceEndGame(gsr GameStateRequest) {
	// lock because we don't want anyone else modifying while we delete
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)

	for lock == nil {
		lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
	}

	deleted := dgs.DeleteGameStateMsg(bot.PrimarySession, true)
	if deleted {
		go metrics.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageCreateDelete, 1)
	}

	bot.RedisInterface.SetDiscordGameState(dgs, lock)

	bot.RedisInterface.RemoveOldGame(dgs.GuildID, dgs.ConnectCode)

	// Note, this shouldn't be necessary with the TTL of the keys, but it can't hurt to clean up...
	bot.RedisInterface.DeleteDiscordGameState(dgs)
}

func MessageDeleteWorker(s *discordgo.Session, msgChannelID, msgID string, waitDur time.Duration) {
	log.Printf("Message worker is sleeping for %s before deleting message", waitDur.String())
	time.Sleep(waitDur)
	err := s.ChannelMessageDelete(msgChannelID, msgID)
	if err != nil {
		log.Println(err)
	}
}

func (bot *Bot) RefreshGameStateMessage(gsr GameStateRequest, sett *settings.GuildSettings) bool {
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
	for lock == nil {
		lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
	}

	// don't try to edit this message, because we're about to delete it
	RemovePendingDGSEdit(dgs.GameStateMsg.MessageID)

	// note, this checks the variables being set, not whether or not the actual Discord message still exists
	gameExists := dgs.GameStateMsg.Exists()
	if !gameExists {
		return false // no-op; no active game to refresh
	}

	deleted := dgs.DeleteGameStateMsg(bot.PrimarySession, false) // delete the old message
	created := dgs.CreateMessage(bot.PrimarySession, bot.gameStateResponse(dgs, sett), dgs.GameStateMsg.MessageChannelID, dgs.GameStateMsg.LeaderID)

	if deleted && created {
		go metrics.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageCreateDelete, 2)
	} else if deleted || created {
		go metrics.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageCreateDelete, 1)
	}

	bot.RedisInterface.SetDiscordGameState(dgs, lock)
	// if for whatever reason the message failed to create, this would catch it
	return dgs.GameStateMsg.Exists()
}

func (bot *Bot) getInfo() command.BotInfo {
	version, commit := rediskey.GetVersionAndCommit(context.Background(), bot.RedisInterface.client)
	totalGuilds := rediskey.GetGuildCounter(context.Background(), bot.RedisInterface.client)
	activeGames := rediskey.GetActiveGames(context.Background(), bot.RedisInterface.client, GameTimeoutSeconds)

	totalUsers := rediskey.GetTotalUsers(context.Background(), bot.RedisInterface.client)
	if totalUsers == rediskey.NotFound {
		totalUsers = rediskey.RefreshTotalUsers(context.Background(), bot.RedisInterface.client, bot.PostgresInterface.Pool)
	}

	totalGames := rediskey.GetTotalGames(context.Background(), bot.RedisInterface.client)
	if totalGames == rediskey.NotFound {
		totalGames = rediskey.RefreshTotalGames(context.Background(), bot.RedisInterface.client, bot.PostgresInterface.Pool)
	}
	return command.BotInfo{
		Version:     version,
		Commit:      commit,
		ShardID:     bot.PrimarySession.ShardID,
		ShardCount:  bot.PrimarySession.ShardCount,
		TotalGuilds: totalGuilds,
		ActiveGames: activeGames,
		TotalUsers:  totalUsers,
		TotalGames:  totalGames,
	}
}

func linkPlayer(redis *RedisInterface, dgs *GameState, userID, color string) (command.LinkStatus, error) {
	var auData amongus.PlayerData
	found := false
	if game.IsColorString(color) {
		auData, found = dgs.GameData.GetByColor(color)
	}
	if found {
		foundID := dgs.AttemptPairingByUserIDs(auData, map[string]interface{}{userID: struct{}{}})
		if foundID != "" {
			err := redis.AddUsernameLink(dgs.GuildID, userID, auData.Name)
			if err != nil {
				log.Println(err)
			}
			return command.LinkSuccess, nil
		} else {
			err := fmt.Sprintf("No player in the current game was found matching %s", discord.MentionByUserID(userID))
			return command.LinkNoPlayer, errors.New(err)
		}
	} else {
		err := fmt.Errorf("no game data found for player %s and color %s", discord.MentionByUserID(userID), color)
		return command.LinkNoGameData, err
	}
}

func unlinkPlayer(dgs *GameState, userID string) command.UnlinkStatus {
	// if we found the player and cleared their data
	success := dgs.ClearPlayerData(userID)
	if success {
		return command.UnlinkSuccess
	} else {
		return command.UnlinkNoPlayer
	}
}

func getTrackingChannel(guild *discordgo.Guild, userID string) string {
	// loop over all the channels in the discord and cross-reference with the one that the .au new author is in
	for _, v := range guild.VoiceStates {
		// if the User who typed au new is in a voice channel
		if v.UserID == userID {
			return v.ChannelID
		}
	}
	return ""
}

func (bot *Bot) newGame(dgs *GameState) (_ command.NewStatus, activeGames int64) {
	if dgs.GameStateMsg.Exists() {
		if v, ok := bot.EndGameChannels[dgs.ConnectCode]; ok {
			v <- true
		}
		delete(bot.EndGameChannels, dgs.ConnectCode)

		dgs.Reset()
	} else {
		premStatus, days, err := bot.PostgresInterface.GetGuildOrUserPremiumStatus(
			bot.official, bot.TopGGClient, dgs.GuildID, dgs.GameStateMsg.LeaderID)
		if err != nil {
			log.Println("Error in /newgame get premium:", err)
		}
		premTier := premium.FreeTier
		if !premium.IsExpired(premStatus, days) {
			premTier = premStatus
		}

		// Premium users should always be allowed to start new games; only check the free guilds
		if premTier == premium.FreeTier {
			activeGames = rediskey.GetActiveGames(context.Background(), bot.RedisInterface.client, GameTimeoutSeconds)
			if activeGames > command.DefaultMaxActiveGames {
				return command.NewLockout, activeGames
			}
		}
	}

	dgs.ConnectCode = generateConnectCode(dgs.GuildID)
	dgs.Subscribed = true

	return command.NewSuccess, activeGames
}

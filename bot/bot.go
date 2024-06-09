package bot

import (
	"context"
	"errors"
	"fmt"
	"github.com/j0nas500/automuteus-tor/v8/bot/command"
	"github.com/j0nas500/automuteus-tor/v8/bot/tokenprovider"
	"github.com/j0nas500/automuteus-tor/v8/internal/server"
	"github.com/j0nas500/automuteus-tor/v8/pkg/amongus"
	"github.com/j0nas500/automuteus/v8/pkg/discord"
	"github.com/j0nas500/automuteus/v8/pkg/game"
	"github.com/j0nas500/automuteus/v8/pkg/premium"
	"github.com/j0nas500/automuteus/v8/pkg/rediskey"
	"github.com/j0nas500/automuteus/v8/pkg/settings"
	storageutils "github.com/j0nas500/automuteus/v8/pkg/storage"
	"github.com/j0nas500/automuteus/v8/pkg/token"
	"github.com/automuteus/automuteus/v8/storage"
	"github.com/bwmarrin/discordgo"
	"github.com/top-gg/go-dbl"
	"log"
	"os"
	"strconv"
	"sync"
	"time"
)

type Bot struct {
	version  string
	commit   string
	official bool
	url      string

	// mapping of socket connections to the game connect codes
	ConnsToGames map[string]string

	StatusEmojis AlivenessEmojis

	EndGameChannels map[string]chan EndGameMessage

	ChannelsMapLock sync.RWMutex

	PrimarySession *discordgo.Session

	TokenProvider *tokenprovider.TokenProvider

	TopGGClient *dbl.Client

	RedisInterface *RedisInterface

	StorageInterface *storage.StorageInterface

	PostgresInterface *storageutils.PsqlInterface

	logPath string

	captureTimeout int
}

// MakeAndStartBot does what it sounds like
// TODO collapse these fields into proper structs?
func MakeAndStartBot(version, commit, botToken, topGGToken, url, emojiGuildID string, numShards, shardID int, redisInterface *RedisInterface, storageInterface *storage.StorageInterface, psql *storageutils.PsqlInterface, logPath string) *Bot {
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
		version:      version,
		commit:       commit,
		official:     os.Getenv("AUTOMUTEUS_OFFICIAL") != "",
		url:          url,
		ConnsToGames: make(map[string]string),
		StatusEmojis: emptyStatusEmojis(),

		EndGameChannels:   make(map[string]chan EndGameMessage),
		ChannelsMapLock:   sync.RWMutex{},
		PrimarySession:    dg,
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

	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildVoiceStates | discordgo.IntentsGuilds)

	token.WaitForToken(bot.RedisInterface.client, botToken)
	token.LockForToken(bot.RedisInterface.client, botToken)
	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		log.Println("Could not connect Bot to the Discord Servers with error:", err)
		return nil
	}

	log.Println("Finished identifying to the Discord API. Now ready for incoming events")

	listeningTo := os.Getenv("AUTOMUTEUS_LISTENING")
	if listeningTo == "" {
		listeningTo = "/help"
	}

	// pretty sure this needs to happen per-shard
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

	if topGGToken != "" {
		dblClient, err := dbl.NewClient(topGGToken)
		if err != nil {
			log.Println("Error creating Top.gg client: ", err)
		}
		bot.TopGGClient = dblClient
	} else {
		log.Println("No TOP_GG_TOKEN provided")
	}

	return &bot
}

func (bot *Bot) InitTokenProvider(tp *tokenprovider.TokenProvider) {
	tp.Init(bot.RedisInterface.client, bot.PrimarySession)
}

func (bot *Bot) StartMetricsServer(nodeID string) error {
	return server.PrometheusMetricsServer(bot.RedisInterface.client, nodeID, "2112")
}

func (bot *Bot) Close() {
	bot.PrimarySession.Close()
	bot.RedisInterface.Close()
	bot.StorageInterface.Close()
}

var EmojiLock = sync.Mutex{}

func (bot *Bot) newGuild(emojiGuildID string) func(s *discordgo.Session, m *discordgo.GuildCreate) {
	return func(s *discordgo.Session, m *discordgo.GuildCreate) {
		gid, err := strconv.ParseUint(m.Guild.ID, 10, 64)
		if err != nil {
			log.Println(err)
		}

		go func() {
			_, err = bot.PostgresInterface.EnsureGuildExists(gid, m.Guild.Name)
			if err != nil {
				log.Println(err)
			}
		}()

		log.Printf("Added to new Guild, id %s, name %s", m.Guild.ID, m.Guild.Name)
		bot.RedisInterface.AddUniqueGuildCounter(m.Guild.ID)

		if emojiGuildID == "" {
			log.Println("[This is not an error] No explicit guildID provided for emojis; using the current guild default")
			emojiGuildID = m.Guild.ID
		}
		// only check/add emojis to the server denoted for emojis, OR, this server that we picked as a fallback above ^
		uploadMissingEmojis := emojiGuildID == m.Guild.ID

		EmojiLock.Lock()
		// only add the emojis if they haven't been added already. Saves api calls for bots in guilds
		if bot.StatusEmojis.isEmpty() {
			allEmojis, err := s.GuildEmojis(emojiGuildID)
			if err != nil {
				log.Println(err)
			} else {
				bot.verifyEmojis(s, emojiGuildID, true, allEmojis, uploadMissingEmojis)
				bot.verifyEmojis(s, emojiGuildID, false, allEmojis, uploadMissingEmojis)
			}
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
		go server.RecordDiscordRequests(bot.RedisInterface.client, server.MessageCreateDelete, 1)
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
		go server.RecordDiscordRequests(bot.RedisInterface.client, server.MessageCreateDelete, 2)
	} else if deleted || created {
		go server.RecordDiscordRequests(bot.RedisInterface.client, server.MessageCreateDelete, 1)
	}

	bot.RedisInterface.SetDiscordGameState(dgs, lock)
	// if for whatever reason the message failed to create, this would catch it
	return dgs.GameStateMsg.Exists()
}

func (bot *Bot) getInfo() command.BotInfo {
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
		Version:     bot.version,
		Commit:      bot.commit,
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

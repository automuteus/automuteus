package bot

import (
	"context"
	"errors"
	"fmt"
	"github.com/automuteus/automuteus/v8/bot/command"
	"github.com/automuteus/automuteus/v8/bot/server"
	"github.com/automuteus/automuteus/v8/bot/tokenprovider"
	"github.com/automuteus/automuteus/v8/pkg"
	"github.com/automuteus/automuteus/v8/pkg/amongus"
	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/automuteus/automuteus/v8/pkg/game"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/redis"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/automuteus/automuteus/v8/pkg/storage"
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

	StatusEmojis discord.AlivenessEmojis

	EndGameChannels map[string]chan EndGameMessage

	ChannelsMapLock sync.RWMutex

	PrimarySession *discordgo.Session

	TokenProvider *tokenprovider.TokenProvider

	TopGGClient *dbl.Client

	RedisDriver redis.Driver

	StorageInterface storage.StorageInterface

	PostgresInterface storage.PsqlInterface

	logPath string

	captureTimeout int
}

// MakeAndStartBot does what it sounds like
// TODO collapse these fields into proper structs?
func MakeAndStartBot(botToken, topGGToken, url, emojiGuildID string, numShards, shardID int, redisDriver redis.Driver, storageInterface storage.StorageInterface, psql storage.PsqlInterface, logPath string) *Bot {
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
		StatusEmojis: discord.EmptyStatusEmojis(),

		EndGameChannels:   make(map[string]chan EndGameMessage),
		ChannelsMapLock:   sync.RWMutex{},
		PrimarySession:    dg,
		RedisDriver:       redisDriver,
		StorageInterface:  storageInterface,
		PostgresInterface: psql,
		logPath:           logPath,
		captureTimeout:    redis.GameTimeoutSeconds,
	}
	dg.LogLevel = discordgo.LogInformational

	dg.AddHandler(bot.handleVoiceStateChange)
	dg.AddHandler(bot.newGuild(emojiGuildID))
	dg.AddHandler(bot.leaveGuild)
	dg.AddHandler(bot.RedisDriver.RateLimitEventCallback)
	// Slash commands
	dg.AddHandler(bot.handleInteractionCreate)

	dg.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Println("Bot is now online according to discord Ready handler")
	})

	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildVoiceStates | discordgo.IntentsGuilds)

	bot.RedisDriver.WaitForToken(botToken)
	bot.RedisDriver.LockForToken(botToken)
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
		Activities: []*discordgo.Activity{{
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
	tp.Init(bot.RedisDriver, bot.PrimarySession)
}

func (bot *Bot) StartMetricsServer(nodeID string) error {
	return server.PrometheusMetricsServer(bot.RedisDriver, nodeID, "2112")
}

func (bot *Bot) Close() {
	bot.PrimarySession.Close()
	bot.RedisDriver.Close()
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
			_, err = bot.PostgresInterface.EnsureGuildExists(gid, m.Guild.Name)
			if err != nil {
				log.Println(err)
			}
		}()

		log.Printf("Added to new Guild, id %s, name %s", m.Guild.ID, m.Guild.Name)
		bot.RedisDriver.AddUniqueGuildCounter(m.Guild.ID)

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

		games := bot.RedisDriver.LoadAllActiveGames(m.Guild.ID)

		for _, connCode := range games {
			gsr := discord.GameStateRequest{
				GuildID:     m.Guild.ID,
				ConnectCode: connCode,
			}
			lock, dgs := bot.RedisDriver.GetDiscordGameStateAndLock(gsr)
			for lock == nil {
				lock, dgs = bot.RedisDriver.GetDiscordGameStateAndLock(gsr)
			}
			if dgs != nil && dgs.ConnectCode != "" {
				log.Println("Resubscribing to Redis events for an old game: " + connCode)
				killChan := make(chan EndGameMessage)
				go bot.SubscribeToGameByConnectCode(gsr.GuildID, dgs.ConnectCode, killChan)
				dgs.Subscribed = true

				bot.RedisDriver.SetDiscordGameState(dgs, lock)

				bot.ChannelsMapLock.Lock()
				bot.EndGameChannels[dgs.ConnectCode] = killChan
				bot.ChannelsMapLock.Unlock()
			}
			lock.Release(context.Background())
		}
	}
}

func (bot *Bot) leaveGuild(_ *discordgo.Session, m *discordgo.GuildDelete) {
	log.Println("Bot was removed from Guild " + m.ID)
	bot.RedisDriver.LeaveUniqueGuildCounter(m.ID)

	err := bot.StorageInterface.DeleteGuildSettings(m.ID)
	if err != nil {
		log.Println(err)
	}
}

func (bot *Bot) forceEndGame(gsr discord.GameStateRequest) {
	// lock because we don't want anyone else modifying while we delete
	lock, dgs := bot.RedisDriver.GetDiscordGameStateAndLock(gsr)

	for lock == nil {
		lock, dgs = bot.RedisDriver.GetDiscordGameStateAndLock(gsr)
	}

	deleted := dgs.DeleteGameStateMsg(bot.PrimarySession, true)
	if deleted {
		go bot.RedisDriver.RecordDiscordRequests(redis.MessageCreateDelete, 1)
	}

	bot.RedisDriver.SetDiscordGameState(dgs, lock)

	bot.RedisDriver.RemoveOldGame(dgs.GuildID, dgs.ConnectCode)

	// Note, this shouldn't be necessary with the TTL of the keys, but it can't hurt to clean up...
	bot.RedisDriver.DeleteDiscordGameState(dgs)
}

func MessageDeleteWorker(s *discordgo.Session, msgChannelID, msgID string, waitDur time.Duration) {
	log.Printf("Message worker is sleeping for %s before deleting message", waitDur.String())
	time.Sleep(waitDur)
	err := s.ChannelMessageDelete(msgChannelID, msgID)
	if err != nil {
		log.Println(err)
	}
}

func (bot *Bot) RefreshGameStateMessage(gsr discord.GameStateRequest, sett *settings.GuildSettings) bool {
	lock, dgs := bot.RedisDriver.GetDiscordGameStateAndLock(gsr)
	for lock == nil {
		lock, dgs = bot.RedisDriver.GetDiscordGameStateAndLock(gsr)
	}

	// don't try to edit this message, because we're about to delete it
	discord.RemovePendingDGSEdit(dgs.GameStateMsg.MessageID)

	// note, this checks the variables being set, not whether or not the actual Discord message still exists
	gameExists := dgs.GameStateMsg.Exists()
	if !gameExists {
		return false // no-op; no active game to refresh
	}

	deleted := dgs.DeleteGameStateMsg(bot.PrimarySession, false) // delete the old message
	created := dgs.CreateMessage(bot.PrimarySession, bot.gameStateResponse(dgs, sett), dgs.GameStateMsg.MessageChannelID, dgs.GameStateMsg.LeaderID)

	if deleted && created {
		go bot.RedisDriver.RecordDiscordRequests(redis.MessageCreateDelete, 2)
	} else if deleted || created {
		go bot.RedisDriver.RecordDiscordRequests(redis.MessageCreateDelete, 1)
	}

	bot.RedisDriver.SetDiscordGameState(dgs, lock)
	// if for whatever reason the message failed to create, this would catch it
	return dgs.GameStateMsg.Exists()
}

func (bot *Bot) GetInfo() discord.BotInfo {
	totalGuilds := bot.RedisDriver.GetGuildCounter(context.Background())
	activeGames := bot.RedisDriver.GetActiveGames(context.Background(), redis.GameTimeoutSeconds)

	totalUsers := bot.RedisDriver.GetTotalUsers(context.Background())
	if totalUsers == redis.NotFound {
		totalUsers = bot.RedisDriver.RefreshTotalUsers(context.Background(), bot.PostgresInterface)
	}

	totalGames := bot.RedisDriver.GetTotalGames(context.Background())
	if totalGames == redis.NotFound {
		totalGames = bot.RedisDriver.RefreshTotalGames(context.Background(), bot.PostgresInterface)
	}
	return discord.BotInfo{
		Version:     pkg.Version,
		Commit:      pkg.Commit,
		ShardID:     bot.PrimarySession.ShardID,
		ShardCount:  bot.PrimarySession.ShardCount,
		TotalGuilds: totalGuilds,
		ActiveGames: activeGames,
		TotalUsers:  totalUsers,
		TotalGames:  totalGames,
	}
}

func linkPlayer(redisDriver redis.Driver, dgs *discord.GameState, userID, color string) (amongus.LinkStatus, error) {
	var auData amongus.PlayerData
	found := false
	if game.IsColorString(color) {
		auData, found = dgs.GameData.GetByColor(color)
	}
	if found {
		foundID := dgs.AttemptPairingByUserIDs(auData, map[string]interface{}{userID: struct{}{}})
		if foundID != "" {
			err := redisDriver.AddUsernameLink(dgs.GuildID, userID, auData.Name)
			if err != nil {
				log.Println(err)
			}
			return amongus.LinkSuccess, nil
		} else {
			err := fmt.Sprintf("No player in the current game was found matching %s", discord.MentionByUserID(userID))
			return amongus.LinkNoPlayer, errors.New(err)
		}
	} else {
		err := fmt.Errorf("no game data found for player %s and color %s", discord.MentionByUserID(userID), color)
		return amongus.LinkNoGameData, err
	}
}

func unlinkPlayer(dgs *discord.GameState, userID string) amongus.UnlinkStatus {
	// if we found the player and cleared their data
	success := dgs.ClearPlayerData(userID)
	if success {
		return amongus.UnlinkSuccess
	} else {
		return amongus.UnlinkNoPlayer
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

func (bot *Bot) newGame(dgs *discord.GameState) (_ command.NewStatus, activeGames int64) {
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
			activeGames = bot.RedisDriver.GetActiveGames(context.Background(), redis.GameTimeoutSeconds)
			if activeGames > command.DefaultMaxActiveGames {
				return command.NewLockout, activeGames
			}
		}
	}

	dgs.ConnectCode = discord.GenerateConnectCode(dgs.GuildID)
	dgs.Subscribed = true

	return command.NewSuccess, activeGames
}

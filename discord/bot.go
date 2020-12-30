package discord

import (
	"context"
	"github.com/automuteus/utils/pkg/game"
	"github.com/automuteus/utils/pkg/rediskey"
	"github.com/automuteus/utils/pkg/token"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/amongus"
	"github.com/denverquane/amongusdiscord/metrics"
	"github.com/denverquane/amongusdiscord/storage"
	"log"
	"os"
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

	PrimarySession *discordgo.Session

	GalactusClient *GalactusClient

	RedisInterface *RedisInterface

	StorageInterface *storage.StorageInterface

	PostgresInterface *storage.PsqlInterface

	logPath string

	captureTimeout int
}

// MakeAndStartBot does what it sounds like
// TODO collapse these fields into proper structs?
func MakeAndStartBot(version, commit, botToken, url, emojiGuildID string, extraTokens []string, numShards, shardID int, redisInterface *RedisInterface, storageInterface *storage.StorageInterface, psql *storage.PsqlInterface, gc *GalactusClient, logPath string) *Bot {
	dg, err := discordgo.New("Bot " + botToken)
	if err != nil {
		log.Println("error creating Discord session,", err)
		return nil
	}

	for _, v := range extraTokens {
		err := gc.AddToken(v)
		if err != nil {
			log.Println("error adding extra bot token to galactus:", err)
		}
	}

	if numShards > 1 {
		log.Printf("Identifying to the Discord API with %d total shards, and shard ID=%d\n", numShards, shardID)
		dg.ShardCount = numShards
		dg.ShardID = shardID
	}

	bot := Bot{
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
	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(bot.handleMessageCreate)
	dg.AddHandler(bot.handleReactionGameStartAdd)
	dg.AddHandler(bot.newGuild(emojiGuildID))
	dg.AddHandler(bot.leaveGuild)
	dg.AddHandler(bot.rateLimitEventCallback)

	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildVoiceStates | discordgo.IntentsGuildMessages | discordgo.IntentsGuilds | discordgo.IntentsGuildMessageReactions)

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

	go StartHealthCheckServer("8080")

	log.Println("Finished identifying to the Discord API. Now ready for incoming events")

	listeningTo := os.Getenv("AUTOMUTEUS_LISTENING")
	if listeningTo == "" {
		prefix := os.Getenv("AUTOMUTEUS_GLOBAL_PREFIX")
		if prefix == "" {
			prefix = ".au"
		}

		listeningTo = prefix + " help"
	}

	status := &discordgo.UpdateStatusData{
		IdleSince: nil,
		Game: &discordgo.Game{
			Name: listeningTo,
			Type: discordgo.GameTypeListening,
		},
		AFK:    false,
		Status: "",
	}
	err = dg.UpdateStatusComplex(*status)
	if err != nil {
		log.Println(err)
	}

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
		go bot.PostgresInterface.EnsureGuildExists(gid, m.Guild.Name)

		log.Printf("Added to new Guild, id %s, name %s", m.Guild.ID, m.Guild.Name)
		bot.RedisInterface.AddUniqueGuildCounter(m.Guild.ID)

		if emojiGuildID == "" {
			log.Println("[This is not an error] No explicit guildID provided for emojis; using the current guild default")
			emojiGuildID = m.Guild.ID
		}

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

func (bot *Bot) leaveGuild(s *discordgo.Session, m *discordgo.GuildDelete) {
	log.Println("Bot was removed from Guild " + m.ID)
	bot.RedisInterface.LeaveUniqueGuildCounter(m.ID)

	err := bot.StorageInterface.DeleteGuildSettings(m.ID)
	if err != nil {
		log.Println(err)
	}
}

func (bot *Bot) linkPlayer(s *discordgo.Session, dgs *GameState, args []string) {
	g, err := s.State.Guild(dgs.GuildID)
	if err != nil {
		log.Println(err)
		return
	}

	userID, err := extractUserIDFromMention(args[0])
	if userID == "" || err != nil {
		log.Printf("Sorry, I don't know who `%s` is. You can pass in ID, username, username#XXXX, nickname or @mention", args[0])
	}

	_, added := dgs.checkCacheAndAddUser(g, s, userID)
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

	dgs.DeleteGameStateMsg(bot.PrimarySession)
	metrics.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageCreateDelete, 1)

	bot.RedisInterface.SetDiscordGameState(dgs, lock)

	bot.RedisInterface.RemoveOldGame(dgs.GuildID, dgs.ConnectCode)

	// Note, this shouldn't be necessary with the TTL of the keys, but it can't hurt to clean up...
	bot.RedisInterface.DeleteDiscordGameState(dgs)
}

func MessageDeleteWorker(s *discordgo.Session, msgChannelID, msgID string, waitDur time.Duration) {
	log.Printf("Message worker is sleeping for %s before deleting message", waitDur.String())
	time.Sleep(waitDur)
	deleteMessage(s, msgChannelID, msgID)
}

func (bot *Bot) RefreshGameStateMessage(gsr GameStateRequest, sett *storage.GuildSettings) {
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
	for lock == nil {
		lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
	}
	// log.Println("Refreshing game state message")

	// don't try to edit this message, because we're about to delete it
	RemovePendingDGSEdit(dgs.GameStateMsg.MessageID)

	if dgs.GameStateMsg.MessageChannelID != "" {
		dgs.DeleteGameStateMsg(bot.PrimarySession) // delete the old message
		dgs.CreateMessage(bot.PrimarySession, bot.gameStateResponse(dgs, sett), dgs.GameStateMsg.MessageChannelID, dgs.GameStateMsg.LeaderID)
		metrics.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageCreateDelete, 2)
	}

	bot.RedisInterface.SetDiscordGameState(dgs, lock)

	// add the emojis to the refreshed message
	if dgs.GameStateMsg.MessageChannelID != "" && dgs.GameStateMsg.MessageID != "" {
		metrics.RecordDiscordRequests(bot.RedisInterface.client, metrics.ReactionAdd, 1)
		dgs.AddReaction(bot.PrimarySession, "▶️")
		// go dgs.AddAllReactions(bot.PrimarySession, bot.StatusEmojis[true])
	}
}

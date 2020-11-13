package discord

import (
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"github.com/denverquane/amongusdiscord/storage"
	"log"
	"os"
	"strings"
	"sync"
)

type Bot struct {
	url string

	//mapping of socket connections to the game connect codes
	ConnsToGames map[string]string

	StatusEmojis AlivenessEmojis

	EndGameChannels map[string]chan EndGameMessage

	ChannelsMapLock sync.RWMutex

	SessionManager *SessionManager

	RedisInterface *RedisInterface

	StorageInterface *storage.StorageInterface

	logPath string

	captureTimeout int
}

var Version string
var Commit string

// MakeAndStartBot does what it sounds like
//TODO collapse these fields into proper structs?
func MakeAndStartBot(version, commit, token, token2, url, emojiGuildID string, numShards, shardID int, redisInterface *RedisInterface, storageInterface *storage.StorageInterface, logPath string, timeoutSecs int) *Bot {
	Version = version
	Commit = commit

	var altDiscordSession *discordgo.Session = nil

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Println("error creating Discord session,", err)
		return nil
	}
	if token2 != "" {
		altDiscordSession, err = discordgo.New("Bot " + token2)
		if err != nil {
			log.Println("error creating 2nd Discord session,", err)
			return nil
		}
	}

	if numShards > 1 {
		log.Printf("Identifying to the Discord API with %d total shards, and shard ID=%d\n", numShards, shardID)
		dg.ShardCount = numShards
		dg.ShardID = shardID
		if altDiscordSession != nil {
			log.Printf("Identifying to the Discord API for the 2nd Bot with %d total shards, and shard ID=%d\n", numShards, shardID)
			altDiscordSession.ShardCount = numShards
			altDiscordSession.ShardID = shardID
		}
	}

	bot := Bot{
		url:          url,
		ConnsToGames: make(map[string]string),
		StatusEmojis: emptyStatusEmojis(),

		EndGameChannels:  make(map[string]chan EndGameMessage),
		ChannelsMapLock:  sync.RWMutex{},
		SessionManager:   NewSessionManager(dg, altDiscordSession),
		RedisInterface:   redisInterface,
		StorageInterface: storageInterface,
		logPath:          logPath,
		captureTimeout:   timeoutSecs,
	}
	dg.LogLevel = discordgo.LogInformational

	dg.AddHandler(bot.handleVoiceStateChange)
	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(bot.handleMessageCreate)
	dg.AddHandler(bot.handleReactionGameStartAdd)
	dg.AddHandler(bot.newGuild(emojiGuildID))
	dg.AddHandler(bot.leaveGuild)

	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildVoiceStates | discordgo.IntentsGuildMessages | discordgo.IntentsGuilds | discordgo.IntentsGuildMessageReactions)

	//Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		log.Println("Could not connect Bot to the Discord Servers with error:", err)
		return nil
	}

	if altDiscordSession != nil {
		altDiscordSession.AddHandler(bot.newAltGuild)
		altDiscordSession.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuilds)
		err = altDiscordSession.Open()
		if err != nil {
			log.Println("Could not connect 2nd Bot to the Discord Servers with error:", err)
			return nil
		}
	}

	bot.RedisInterface.SetVersionAndCommit(Version, Commit)

	go StartHealthCheckServer("8080")

	log.Println("Finished identifying to the Discord API. Now ready for incoming events")

	listeningTo := os.Getenv("AUTOMUTEUS_LISTENING")
	if listeningTo == "" {
		listeningTo = ".au help"
	}

	status := &discordgo.UpdateStatusData{
		IdleSince: nil,
		Activities: &[]discordgo.Game{
			{
				Name: listeningTo,
				Type: discordgo.GameTypeListening,
			}},
		AFK:    false,
		Status: "",
	}
	dg.UpdateStatusComplex(*status)

	return &bot
}

func (bot *Bot) GracefulClose() {
	bot.ChannelsMapLock.RLock()
	for _, v := range bot.EndGameChannels {
		v <- EndGameMessage{EndGameType: EndAndSave}
	}

	bot.ChannelsMapLock.RUnlock()
}
func (bot *Bot) Close() {
	bot.SessionManager.Close()
	bot.RedisInterface.Close()
	bot.StorageInterface.Close()
}

func (bot *Bot) PurgeConnection(socketID string) {

	delete(bot.ConnsToGames, socketID)

	//TODO purge all the data in the database here

}

func (bot *Bot) gracefulShutdownWorker(guildID, connCode string) {
	dgs := bot.RedisInterface.GetReadOnlyDiscordGameState(GameStateRequest{
		GuildID:     guildID,
		ConnectCode: connCode,
	})

	log.Printf("Received graceful shutdown message, saving and shutting down")

	gsr := GameStateRequest{
		GuildID:      dgs.GuildID,
		TextChannel:  dgs.GameStateMsg.MessageChannelID,
		VoiceChannel: dgs.Tracking.ChannelID,
		ConnectCode:  dgs.ConnectCode,
	}
	bot.gracefulEndGame(gsr)

	log.Println("Finished gracefully shutting down")
}

func (bot *Bot) newGuild(emojiGuildID string) func(s *discordgo.Session, m *discordgo.GuildCreate) {
	return func(s *discordgo.Session, m *discordgo.GuildCreate) {

		log.Printf("Added to new Guild, id %s, name %s", m.Guild.ID, m.Guild.Name)
		bot.RedisInterface.AddUniqueGuildCounter(m.Guild.ID, Version)

		//f, err := os.Create(path.Join(bot.logPath, m.Guild.ID+"_log.txt"))
		//w := io.MultiWriter(os.Stdout)
		//if err != nil {
		//	log.Println("Couldn't create logger for " + m.Guild.ID + "; only using stdout for logging")
		//} else {
		//	w = io.MultiWriter(f, os.Stdout)
		//}

		if emojiGuildID == "" {
			log.Println("[This is not an error] No explicit guildID provided for emojis; using the current guild default")
			emojiGuildID = m.Guild.ID
		}
		allEmojis, err := s.GuildEmojis(emojiGuildID)
		if err != nil {
			log.Println(err)
		} else {
			bot.addAllMissingEmojis(s, m.Guild.ID, true, allEmojis)

			bot.addAllMissingEmojis(s, m.Guild.ID, false, allEmojis)
		}

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
	bot.RedisInterface.LeaveUniqueGuildCounter(m.ID, Version)

	err := bot.StorageInterface.DeleteGuildSettings(m.ID)
	if err != nil {
		log.Println(err)
	}
}

func (bot *Bot) newAltGuild(s *discordgo.Session, m *discordgo.GuildCreate) {
	bot.SessionManager.RegisterGuildSecondSession(m.Guild.ID)
}

func (bot *Bot) linkPlayer(s *discordgo.Session, dgs *DiscordGameState, args []string) {
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
	var auData game.PlayerData
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
	return
}

func (bot *Bot) gracefulEndGame(gsr GameStateRequest) {
	//sett := bot.StorageInterface.GetGuildSettings(gsr.GuildID)
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
	if lock == nil {
		log.Println("Couldnt obtain lock when ending game")
		//s.ChannelMessageSend(gsr.TextChannel, "Could not obtain lock when ending game! You'll need to manually unmute/undeafen players!")
		return
	}
	//log.Println("lock obtained for game end")

	if dgs.Linked && dgs.GameStateMsg.MessageID != "" && dgs.GameStateMsg.MessageChannelID != "" {
		sess := bot.SessionManager.GetPrimarySession()
		sess.ChannelMessageSend(dgs.GameStateMsg.MessageChannelID, "Your game might be momentarily disrupted while I upgrade...")
	}

	dgs.Subscribed = false
	dgs.Linked = false

	for v, userData := range dgs.UserData {
		userData.SetShouldBeMuteDeaf(false, false)
		dgs.UserData[v] = userData
	}

	bot.RedisInterface.SetDiscordGameState(dgs, lock)
	sett := bot.StorageInterface.GetGuildSettings(gsr.GuildID)
	dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs, sett))

	log.Println("Done saving guild data")
}

func (bot *Bot) forceEndGame(gsr GameStateRequest) {
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
	if lock == nil {
		return
	}

	dgs.AmongUsData.SetAllAlive()
	dgs.AmongUsData.UpdatePhase(game.LOBBY)
	dgs.AmongUsData.SetRoomRegion("", "")

	lock.Release(ctx)

	bot.RedisInterface.RemoveOldGame(dgs.GuildID, dgs.ConnectCode)

	//TODO this shouldn't be necessary with the TTL of the keys, but it can't hurt to clean up...
	bot.RedisInterface.DeleteDiscordGameState(dgs)
}

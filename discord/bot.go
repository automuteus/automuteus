package discord

import (
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"github.com/denverquane/amongusdiscord/storage"
	"log"
	"strings"
	"sync"
)

const DefaultPort = "8123"

type Bot struct {
	url          string
	internalPort string

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
func MakeAndStartBot(version, commit, token, token2, url, internalPort, emojiGuildID string, numShards, shardID int, redisInterface *RedisInterface, storageInterface *storage.StorageInterface, logPath string, timeoutSecs int) *Bot {
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
		internalPort: internalPort,
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

	dg.AddHandler(bot.handleVoiceStateChange)
	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(bot.handleMessageCreate)
	dg.AddHandler(bot.handleReactionGameStartAdd)
	dg.AddHandler(bot.newGuild(emojiGuildID))

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

	status := &discordgo.UpdateStatusData{
		IdleSince: nil,
		Game: &discordgo.Game{
			Name: ".au help",
			Type: discordgo.GameTypeListening,
		},
		AFK:    false,
		Status: "",
	}

	dg.UpdateStatusComplex(*status)

	bot.Run(internalPort)

	return &bot
}

func (bot *Bot) Run(port string) {
	go bot.socketioServer(port)
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

	bot.RedisInterface.AppendToActiveGames(gsr.GuildID, gsr.ConnectCode)

	log.Println("Finished gracefully shutting down")

	//this is only for forceful shutdown
	//bot.RedisInterface.DeleteDiscordGameState(dgs)
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

		games := bot.RedisInterface.LoadAllActiveGamesAndDelete(m.Guild.ID)

		for _, connCode := range games {
			gsr := GameStateRequest{
				GuildID:     m.Guild.ID,
				ConnectCode: connCode,
			}
			lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
			if lock != nil && dgs != nil && !dgs.Subscribed && dgs.ConnectCode != "" {
				log.Println("Resubscribing to Redis events for an old game: " + connCode)
				killChan := make(chan EndGameMessage)
				go bot.SubscribeToGameByConnectCode(gsr.GuildID, dgs.ConnectCode, killChan)
				dgs.Subscribed = true

				bot.RedisInterface.SetDiscordGameState(dgs, lock)

				bot.ChannelsMapLock.Lock()
				bot.EndGameChannels[dgs.ConnectCode] = killChan
				bot.ChannelsMapLock.Unlock()
			} else if lock != nil {
				//log.Println("UNLOCKING")
				lock.Release()
			}
		}

		if len(games) == 0 {
			dsg := NewDiscordGameState(m.Guild.ID)

			//put an empty entry in Redis
			bot.RedisInterface.SetDiscordGameState(dsg, nil)
		}

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
		found = dgs.AttemptPairingByUserIDs(auData, map[string]interface{}{userID: ""})
		if found {
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

	lock.Release()

	//TODO this shouldn't be necessary with the TTL of the keys, but it can't hurt to clean up...
	bot.RedisInterface.DeleteDiscordGameState(dgs)
}

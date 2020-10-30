package discord

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"github.com/denverquane/amongusdiscord/storage"
	socketio "github.com/googollee/go-socket.io"
)

const DefaultPort = "8123"

type BcastMsgType int

const (
	GRACEFUL_SHUTDOWN BcastMsgType = iota
	FORCE_SHUTDOWN
)

type BroadcastMessage struct {
	Type    BcastMsgType
	Data    int
	Message string
}

type Bot struct {
	url          string
	internalPort string

	//mapping of socket connections to the game connect codes
	ConnsToGames map[string]string

	StatusEmojis AlivenessEmojis

	GlobalBroadcastChannels map[string]chan BroadcastMessage

	RedisSubscriberKillChannels map[string]chan bool

	ChannelsMapLock sync.RWMutex

	SessionManager *SessionManager

	RedisInterface *RedisInterface

	StorageInterface *storage.StorageInterface

	logPath string

	captureTimeout int
}

var Version string

// MakeAndStartBot does what it sounds like
//TODO collapse these fields into proper structs?
func MakeAndStartBot(version, token, token2, url, internalPort, emojiGuildID string, numShards, shardID int, redisInterface *RedisInterface, storageInterface *storage.StorageInterface, logPath string, timeoutSecs int) *Bot {
	Version = version

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

		RedisSubscriberKillChannels: make(map[string]chan bool),
		GlobalBroadcastChannels:     make(map[string]chan BroadcastMessage),
		ChannelsMapLock:             sync.RWMutex{},
		SessionManager:              NewSessionManager(dg, altDiscordSession),
		RedisInterface:              redisInterface,
		StorageInterface:            storageInterface,
		logPath:                     logPath,
		captureTimeout:              timeoutSecs,
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

	bot.Run(internalPort)

	return &bot
}

func (bot *Bot) Run(port string) {
	go bot.socketioServer(port)
}

func (bot *Bot) GracefulClose(seconds int, message string) {
	bot.ChannelsMapLock.RLock()
	for _, v := range bot.GlobalBroadcastChannels {
		v <- BroadcastMessage{
			Type:    GRACEFUL_SHUTDOWN,
			Data:    seconds,
			Message: message,
		}
	}
	for _, v := range bot.RedisSubscriberKillChannels {
		v <- true
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

func (bot *Bot) InactiveGameWorker(socket socketio.Conn, c <-chan string) {
	timer := time.NewTimer(time.Second * time.Duration(bot.captureTimeout))
	guildID := ""
	for {
		select {
		case <-timer.C:
			log.Printf("Socket ID %s timed out with no new messages after %d seconds\n", socket.ID(), bot.captureTimeout)
			socket.Close()
			bot.PurgeConnection(socket.ID())

			if v, ok := bot.GlobalBroadcastChannels[guildID]; ok {
				if v != nil {
					v <- BroadcastMessage{
						Type:    GRACEFUL_SHUTDOWN,
						Data:    1,
						Message: fmt.Sprintf("**I haven't received any messages from your capture in %d seconds, so I'm ending the game!**", bot.captureTimeout),
					}
				}
			}
			timer.Stop()
			return
		case b := <-c:
			if b != "" {
				guildID = b
			}
			//received true; the socket is alive
			log.Printf("Bot inactivity timer has been reset to %d seconds\n", bot.captureTimeout)
			timer.Reset(time.Second * time.Duration(bot.captureTimeout))
		}
	}
}

func (bot *Bot) gracefulShutdownWorker(guildID, connCode string, s *discordgo.Session, seconds int, message string) {
	dgs := bot.RedisInterface.GetReadOnlyDiscordGameState(GameStateRequest{
		GuildID:     guildID,
		ConnectCode: connCode,
	})
	if dgs.GameStateMsg.MessageID != "" {
		log.Printf("Received graceful shutdown message, saving and shutting down in %d seconds", seconds)

		//sendMessage(s, dgs.GameStateMsg.MessageChannelID, message)
	}

	time.Sleep(time.Duration(seconds) * time.Second)

	gsr := GameStateRequest{
		GuildID:      dgs.GuildID,
		TextChannel:  dgs.GameStateMsg.MessageChannelID,
		VoiceChannel: dgs.Tracking.ChannelID,
		ConnectCode:  dgs.ConnectCode,
	}
	bot.gracefulEndGame(gsr, s)

	//this is only for forceful shutdown
	//bot.RedisInterface.DeleteDiscordGameState(dgs)
}

func (bot *Bot) newGuild(emojiGuildID string) func(s *discordgo.Session, m *discordgo.GuildCreate) {
	return func(s *discordgo.Session, m *discordgo.GuildCreate) {

		log.Printf("Added to new Guild, id %s, name %s", m.Guild.ID, m.Guild.Name)

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

		globalUpdates := make(chan BroadcastMessage)

		bot.ChannelsMapLock.Lock()
		bot.GlobalBroadcastChannels[m.Guild.ID] = globalUpdates
		bot.ChannelsMapLock.Unlock()

		dsg := NewDiscordGameState(m.Guild.ID)

		//put an empty entry in Redis
		bot.RedisInterface.SetDiscordGameState(dsg, nil)

		go bot.updatesListener(s, m.Guild.ID, globalUpdates)
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

	userID := getMemberFromString(s, dgs.GuildID, args[0])
	if userID == "" {
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
		found = dgs.AttemptPairingByMatchingNames(auData)
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

func (bot *Bot) gracefulEndGame(gsr GameStateRequest, s *discordgo.Session) {
	//sett := bot.StorageInterface.GetGuildSettings(gsr.GuildID)
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
	if lock == nil {
		log.Println("Couldnt obtain lock when ending game")
		s.ChannelMessageSend(gsr.TextChannel, "Could not obtain lock when ending game! You'll need to manually unmute/undeafen players!")
		return
	}

	if v, ok := bot.RedisSubscriberKillChannels[dgs.ConnectCode]; ok {
		v <- true
	}
	delete(bot.RedisSubscriberKillChannels, dgs.ConnectCode)

	dgs.Subscribed = false
	dgs.Linked = false

	bot.RedisInterface.SetDiscordGameState(dgs, lock)

	dgs.AmongUsData.SetAllAlive()
	dgs.AmongUsData.UpdatePhase(game.LOBBY)
	dgs.AmongUsData.SetRoomRegion("", "")

	//TODO need an override to unmute, write some custom handler for it

	// apply the unmute/deafen to users who have state linked to them
	//bot.handleTrackedMembers(bot.SessionManager, sett, 0, NoPriority, gsr)

	log.Println("Done saving guild data. Ready for shutdown")
}

func (bot *Bot) forceEndGame(gsr GameStateRequest, s *discordgo.Session) {
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
	if lock == nil {
		s.ChannelMessageSend(gsr.TextChannel, "Could not obtain lock when forcefully ending game! You'll need to manually unmute/undeafen players!")
		return
	}

	if v, ok := bot.RedisSubscriberKillChannels[dgs.ConnectCode]; ok {
		v <- true
	}
	delete(bot.RedisSubscriberKillChannels, dgs.ConnectCode)

	dgs.AmongUsData.SetAllAlive()
	dgs.AmongUsData.UpdatePhase(game.LOBBY)
	dgs.AmongUsData.SetRoomRegion("", "")

	bot.RedisInterface.SetDiscordGameState(dgs, lock)

	sett := bot.StorageInterface.GetGuildSettings(dgs.GuildID)

	// apply the unmute/deafen to users who have state linked to them
	bot.handleTrackedMembers(bot.SessionManager, sett, 0, NoPriority, gsr)

	lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(gsr)

	//clear the Tracking and make sure all users are unlinked
	dgs.clearGameTracking(s)

	dgs.Running = false

	bot.RedisInterface.SetDiscordGameState(dgs, lock)

}

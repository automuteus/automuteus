package discord

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"github.com/denverquane/amongusdiscord/storage"
	socketio "github.com/googollee/go-socket.io"
	"github.com/gorilla/mux"
)

const DefaultPort = "8123"

type GameOrLobbyCode struct {
	gameCode    string
	connectCode string
}

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

type LobbyStatus struct {
	GuildID string
	Lobby   game.Lobby
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

func (bot *Bot) socketioServer(port string) {
	inactiveWorkerChannels := make(map[string]chan string)

	server, err := socketio.NewServer(nil)
	if err != nil {
		log.Fatal(err)
	}
	server.OnConnect("/", func(s socketio.Conn) error {
		s.SetContext("")
		log.Println("connected:", s.ID())
		inactiveWorkerChannels[s.ID()] = make(chan string)
		go bot.InactiveGameWorker(s, inactiveWorkerChannels[s.ID()])
		return nil
	})
	server.OnEvent("/", "connectCode", func(s socketio.Conn, msg string) {
		log.Printf("Received connection code: \"%s\"", msg)

		bot.ConnsToGames[s.ID()] = msg
		bot.RedisInterface.PublishConnectUpdate(msg, "true")
	})
	server.OnEvent("/", "lobby", func(s socketio.Conn, msg string) {
		log.Println("lobby:", msg)
		lobby := game.Lobby{}
		err := json.Unmarshal([]byte(msg), &lobby)
		if err != nil {
			log.Println(err)
		} else {
			if cCode, ok := bot.ConnsToGames[s.ID()]; ok {
				bot.RedisInterface.PublishLobbyUpdate(cCode, msg)
			}
		}
	})
	server.OnEvent("/", "state", func(s socketio.Conn, msg string) {
		log.Println("phase received from capture: ", msg)
		_, err := strconv.Atoi(msg)
		if err != nil {
			log.Println(err)
		} else {
			if cCode, ok := bot.ConnsToGames[s.ID()]; ok {
				bot.RedisInterface.PublishPhaseUpdate(cCode, msg)
			}
		}
	})
	server.OnEvent("/", "player", func(s socketio.Conn, msg string) {
		log.Println("player received from capture: ", msg)
		player := game.Player{}
		err := json.Unmarshal([]byte(msg), &player)
		if err != nil {
			log.Println(err)
		} else {
			if cCode, ok := bot.ConnsToGames[s.ID()]; ok {
				bot.RedisInterface.PublishPlayerUpdate(cCode, msg)
			}
		}
	})
	server.OnError("/", func(s socketio.Conn, e error) {
		log.Println("meet error:", e)
	})
	server.OnDisconnect("/", func(s socketio.Conn, reason string) {
		log.Println("Client connection closed: ", reason)

		if cCode, ok := bot.ConnsToGames[s.ID()]; ok {
			bot.RedisInterface.PublishConnectUpdate(cCode, "false")
		}

		bot.PurgeConnection(s.ID())
	})
	go server.Serve()
	defer server.Close()

	//http.Handle("/socket.io/", server)

	router := mux.NewRouter()
	router.Handle("/socket.io/", server)

	log.Printf("Serving at localhost:%s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
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

func MessagesServer(port string, bot *Bot) {
	http.HandleFunc("/graceful", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			bot.ChannelsMapLock.RLock()
			for _, v := range bot.GlobalBroadcastChannels {
				v <- BroadcastMessage{
					Type:    GRACEFUL_SHUTDOWN,
					Data:    30,
					Message: fmt.Sprintf("I'm being shut down in %d seconds, and will be closing your active game!", 30),
				}

			}
			bot.ChannelsMapLock.RUnlock()
		}
	})
	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		data := map[string]interface{}{
			"activeConnections": len(bot.ConnsToGames),
			//"activeGames":       len(bot.LinkCodes), //probably an inaccurate metric
			//"totalGuilds":       len(bot.AllGuilds),
		}
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			log.Println(err)
		}
		w.Write(jsonBytes)
	})

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func (bot *Bot) SubscribeToGameByConnectCode(guildID, connectCode string, killChan <-chan bool) {
	connection, lobby, phase, player := bot.RedisInterface.SubscribeToGame(connectCode)
	for {
		select {
		case gameMessage := <-connection.Channel():
			log.Println(gameMessage)
			dgs := bot.RedisInterface.GetDiscordGameState(guildID, "", "", connectCode)
			if gameMessage.Payload == "true" {
				dgs.Linked = true
			} else {
				dgs.Linked = false
			}
			dgs.ConnectCode = connectCode
			dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
			bot.RedisInterface.SetDiscordGameState(guildID, dgs)
			break
		case gameMessage := <-lobby.Channel():
			dgs := bot.RedisInterface.GetDiscordGameState(guildID, "", "", connectCode)
			var lobby game.Lobby
			err := json.Unmarshal([]byte(gameMessage.Payload), &lobby)
			if err != nil {
				log.Println(err)
				break
			}
			bot.processLobby(dgs, bot.SessionManager.GetPrimarySession(), lobby)
			bot.RedisInterface.SetDiscordGameState(guildID, dgs)
			break
		case gameMessage := <-phase.Channel():
			var phase game.Phase
			err := json.Unmarshal([]byte(gameMessage.Payload), &phase)
			if err != nil {
				log.Println(err)
				break
			}
			bot.processTransition(guildID, connectCode, phase)
			break
		case gameMessage := <-player.Channel():
			dgs := bot.RedisInterface.GetDiscordGameState(guildID, "", "", connectCode)
			sett := bot.StorageInterface.GetGuildSettings(guildID)
			var player game.Player
			err := json.Unmarshal([]byte(gameMessage.Payload), &player)
			if err != nil {
				log.Println(err)
				break
			}
			bot.processPlayer(dgs, sett, player)
			bot.RedisInterface.SetDiscordGameState(guildID, dgs)
			break
		case k := <-killChan:
			if k {
				log.Println("Redis subscriber received kill signal, closing all pubsubs")
				err := connection.Close()
				if err != nil {
					log.Println(err)
				}
				err = lobby.Close()
				if err != nil {
					log.Println(err)
				}
				err = phase.Close()
				if err != nil {
					log.Println(err)
				}
				err = player.Close()
				if err != nil {
					log.Println(err)
				}
				return
			}
		}
	}
}
func (bot *Bot) processPlayer(dgs *DiscordGameState, sett *storage.GuildSettings, player game.Player) {
	if player.Name != "" {
		if player.Disconnected || player.Action == game.LEFT {
			log.Println("I detected that " + player.Name + " disconnected or left! " +
				"I'm removing their linked game data; they will need to relink")

			dgs.ClearPlayerDataByPlayerName(player.Name)
			dgs.ClearAmongUsData(player.Name)
			dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
			return
		} else {
			updated, isAliveUpdated, data := dgs.UpdateAmongUsData(player)

			if player.Action == game.JOINED {
				log.Println("Detected a player joined, refreshing User data mappings")
				paired := dgs.AttemptPairingByMatchingNames(data)
				//try pairing via the cached usernames
				if !paired {
					uids := bot.RedisInterface.GetUidMappings(dgs.GuildID, player.Name)
					paired = dgs.AttemptPairingByUserIDs(data, uids)
				}

				dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
			} else if updated {
				paired := dgs.AttemptPairingByMatchingNames(data)
				//try pairing via the cached usernames
				if !paired {
					uids := bot.RedisInterface.GetUidMappings(dgs.GuildID, player.Name)
					paired = dgs.AttemptPairingByUserIDs(data, uids)
				}
				//log.Println("Player update received caused an update in cached state")
				if isAliveUpdated && dgs.GetPhase() == game.TASKS {
					if sett.GetUnmuteDeadDuringTasks() {
						// unmute players even if in tasks because unmuteDeadDuringTasks is true
						dgs.handleTrackedMembers(bot.SessionManager, sett, 0, NoPriority, game.TASKS)
						dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
					} else {
						log.Println("NOT updating the discord status message; would leak info")
					}
				} else {
					dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
				}
			} else {
				//No changes occurred; no reason to update
			}
		}
	}
}

func (bot *Bot) processTransition(guildID, connectCode string, phase game.Phase) {
	dgs := bot.RedisInterface.GetDiscordGameState(guildID, "", "", connectCode)
	defer bot.RedisInterface.SetDiscordGameState(guildID, dgs)

	sett := bot.StorageInterface.GetGuildSettings(guildID)
	oldPhase := dgs.UpdatePhase(phase)
	if oldPhase == phase {
		return
	}

	switch phase {
	case game.MENU:
		dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
		dgs.RemoveAllReactions(bot.SessionManager.GetPrimarySession())
		break
	case game.LOBBY:
		delay := sett.Delays.GetDelay(oldPhase, phase)
		dgs.handleTrackedMembers(bot.SessionManager, sett, delay, NoPriority, phase)
		dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
		dgs.AddAllReactions(bot.SessionManager.GetPrimarySession(), bot.StatusEmojis[true])
		break
	case game.TASKS:
		delay := sett.Delays.GetDelay(oldPhase, phase)
		//when going from discussion to tasks, we should mute alive players FIRST
		priority := AlivePriority
		if oldPhase == game.LOBBY {
			priority = NoPriority
		}
		dgs.handleTrackedMembers(bot.SessionManager, sett, delay, priority, phase)
		dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
		break
	case game.DISCUSS:
		delay := sett.Delays.GetDelay(oldPhase, phase)

		dgs.handleTrackedMembers(bot.SessionManager, sett, delay, DeadPriority, dgs.GetPhase())

		dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
		break
	}
}

func (bot *Bot) processLobby(dgs *DiscordGameState, s *discordgo.Session, lobby game.Lobby) {
	dgs.SetRoomRegion(lobby.LobbyCode, lobby.Region.ToString())

	dgs.Edit(s, bot.gameStateResponse(dgs))
}

func (bot *Bot) updatesListener(dg *discordgo.Session, guildID string, globalUpdates chan BroadcastMessage) {
	for {
		select {
		case worldUpdate := <-globalUpdates:
			bot.ChannelsMapLock.Lock()
			for i, connCode := range bot.ConnsToGames {
				if worldUpdate.Type == GRACEFUL_SHUTDOWN {
					dgs := bot.RedisInterface.GetDiscordGameState(guildID, "", "", connCode)
					go bot.gracefulShutdownWorker(dg, dgs, worldUpdate.Data, worldUpdate.Message)
					dgs.Linked = false
					delete(bot.ConnsToGames, i)
				}
			}
			bot.ChannelsMapLock.Unlock()
		}
	}
}

func (bot *Bot) gracefulShutdownWorker(s *discordgo.Session, dgs *DiscordGameState, seconds int, message string) {
	if dgs.GameStateMsg.MessageID != "" {
		log.Printf("**Received graceful shutdown message, shutting down in %d seconds**", seconds)

		sendMessage(s, dgs.GameStateMsg.MessageChannelID, message)
	}

	time.Sleep(time.Duration(seconds) * time.Second)

	bot.endGame(dgs, s)

	bot.RedisInterface.DeleteDiscordGameState(dgs)
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
		bot.RedisInterface.SetDiscordGameState(m.Guild.ID, dsg)

		go bot.updatesListener(s, m.Guild.ID, globalUpdates)
	}
}

func (bot *Bot) newAltGuild(s *discordgo.Session, m *discordgo.GuildCreate) {
	bot.SessionManager.RegisterGuildSecondSession(m.Guild.ID)
}

func (bot *Bot) handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	g, err := s.State.Guild(m.GuildID)
	if err != nil {
		log.Println(err)
		return
	}

	contents := m.Content
	sett := bot.StorageInterface.GetGuildSettings(m.GuildID)
	prefix := sett.GetCommandPrefix()

	if strings.HasPrefix(contents, prefix) {
		//either BOTH the admin/roles are empty, or the User fulfills EITHER perm "bucket"
		perms := sett.EmptyAdminAndRolePerms()
		if !perms {
			perms = sett.HasAdminPerms(m.Author) || sett.HasRolePerms(m.Member)
		}
		if !perms && g.OwnerID != m.Author.ID {
			s.ChannelMessageSend(m.ChannelID, "User does not have the required permissions to execute this command!")
		} else {
			oldLen := len(contents)
			contents = strings.Replace(contents, prefix+" ", "", 1)
			if len(contents) == oldLen { //didn't have a space
				contents = strings.Replace(contents, prefix, "", 1)
			}

			if len(contents) == 0 {
				if len(prefix) <= 1 {
					// prevent bot from spamming help message whenever the single character
					// prefix is sent by mistake
					return
				} else {
					embed := helpResponse(Version, prefix, AllCommands)
					s.ChannelMessageSendEmbed(m.ChannelID, &embed)
				}
			} else {
				args := strings.Split(contents, " ")

				for i, v := range args {
					args[i] = strings.ToLower(v)
				}
				bot.HandleCommand(sett, s, g, m, args)
			}
		}
	}
}

func (bot *Bot) handleReactionGameStartAdd(s *discordgo.Session, m *discordgo.MessageReactionAdd) {
	// Ignore all reactions created by the bot itself
	if m.UserID == s.State.User.ID {
		return
	}

	g, err := s.State.Guild(m.GuildID)
	if err != nil {
		log.Println(err)
		return
	}

	dgs := bot.RedisInterface.GetDiscordGameState(m.GuildID, m.ChannelID, "", "")
	if dgs != nil && dgs.Exists() {
		//verify that the User is reacting to the state/status message
		if dgs.IsReactionTo(m) {
			sett := bot.StorageInterface.GetGuildSettings(m.GuildID)
			idMatched := false
			for color, e := range bot.StatusEmojis[true] {
				if e.ID == m.Emoji.ID {
					idMatched = true
					log.Print(fmt.Sprintf("Player %s reacted with color %s\n", m.UserID, game.GetColorStringForInt(color)))
					//the User doesn't exist in our userdata cache; add them
					user, added := dgs.checkCacheAndAddUser(g, s, m.UserID)
					if !added {
						log.Println("No users found in Discord for UserID " + m.UserID)
						idMatched = false
					} else {
						auData, found := dgs.GetByColor(game.GetColorStringForInt(color))
						if found {
							user.Link(auData)
							dgs.UpdateUserData(m.UserID, user)
							err := bot.RedisInterface.AddUsernameLink(m.GuildID, m.UserID, auData.Name)
							if err != nil {
								log.Println(err)
							}
						} else {
							log.Println("I couldn't find any player data for that color; is your capture linked?")
							idMatched = false
						}
					}

					//then remove the player's reaction if we matched, or if we didn't
					err := s.MessageReactionRemove(m.ChannelID, m.MessageID, e.FormatForReaction(), m.UserID)
					if err != nil {
						log.Println(err)
					}
					break

				}
			}
			if !idMatched {
				//log.Println(m.Emoji.Name)
				if m.Emoji.Name == "❌" {
					log.Println("Removing player " + m.UserID)
					dgs.ClearPlayerData(m.UserID)
					err := s.MessageReactionRemove(m.ChannelID, m.MessageID, "❌", m.UserID)
					if err != nil {
						log.Println(err)
					}
					idMatched = true
				}
			}
			//make sure to update any voice changes if they occurred
			if idMatched {
				dgs.handleTrackedMembers(bot.SessionManager, sett, 0, NoPriority, dgs.GetPhase())
				dgs.Edit(s, bot.gameStateResponse(dgs))
			}
		}
		bot.RedisInterface.SetDiscordGameState(m.GuildID, dgs)
	}
}

//voiceStateChange handles more edge-case behavior for users moving between voice channels, and catches when
//relevant discord api requests are fully applied successfully. Otherwise, we can issue multiple requests for
//the same mute/unmute, erroneously
func (bot *Bot) handleVoiceStateChange(s *discordgo.Session, m *discordgo.VoiceStateUpdate) {

	dgs := bot.RedisInterface.GetDiscordGameState(m.GuildID, "", m.ChannelID, "")
	if dgs == nil {
		return
	}

	sett := bot.StorageInterface.GetGuildSettings(m.GuildID)

	g := dgs.verifyVoiceStateChanges(s, sett, dgs.GetPhase())

	if g == nil {
		return
	}

	//fetch the userData from our userData data cache
	userData, err := dgs.GetUser(m.UserID)
	if err != nil {
		//the User doesn't exist in our userdata cache; add them
		userData, _ = dgs.checkCacheAndAddUser(g, s, m.UserID)
	}
	tracked := m.ChannelID != "" && dgs.Tracking.ChannelID == m.ChannelID

	auData, found := dgs.GetByName(userData.InGameName)
	//only actually tracked if we're in a tracked channel AND linked to a player
	tracked = tracked && found
	mute, deaf := sett.GetVoiceState(auData.IsAlive, tracked, dgs.GetPhase())
	//check the userdata is linked here to not accidentally undeafen music bots, for example
	if found && userData.IsVoiceChangeReady() && (mute != m.Mute || deaf != m.Deaf) {
		userData.SetVoiceChangeReady(false)

		dgs.UpdateUserData(m.UserID, userData)

		nick := userData.GetPlayerName()
		if !sett.GetApplyNicknames() {
			nick = ""
		}

		go guildMemberUpdate(s, UserPatchParameters{m.GuildID, userData, deaf, mute, nick})
	}
	bot.RedisInterface.SetDiscordGameState(m.GuildID, dgs)
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
		auData, found = dgs.GetByColor(combinedArgs)

	} else {
		auData, found = dgs.GetByName(combinedArgs)
	}
	if found {
		found = dgs.AttemptPairingByMatchingNames(auData)
		if found {
			log.Printf("Successfully linked %s to a color\n", userID)
			bot.RedisInterface.SetDiscordGameState(dgs.GuildID, dgs)
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

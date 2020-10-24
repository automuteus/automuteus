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

	GlobalBroadcastChannels map[string]*chan BroadcastMessage

	ChannelsMapLock sync.RWMutex

	SessionManager *SessionManager

	StorageInterface *DatabaseInterface

	logPath string

	captureTimeout int
}

var Version string

// MakeAndStartBot does what it sounds like
//TODO collapse these fields into proper structs?
func MakeAndStartBot(version, token, token2, url, internalPort, emojiGuildID string, numShards, shardID int, storageClient *DatabaseInterface, logPath string, timeoutSecs int) *Bot {
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

		GlobalBroadcastChannels: make(map[string]*chan BroadcastMessage),
		ChannelsMapLock:         sync.RWMutex{},
		SessionManager:          NewSessionManager(dg, altDiscordSession),
		StorageInterface:        storageClient,
		logPath:                 logPath,
		captureTimeout:          timeoutSecs,
	}

	dg.AddHandler(bot.voiceStateChange)
	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(bot.messageCreate)
	dg.AddHandler(bot.reactionCreate)
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

	// Wait here until CTRL-C or other term signal is received.

	bot.Run(internalPort)

	return &bot
}

func (bot *Bot) Run(port string) {
	go bot.socketioServer(port)
}

func (bot *Bot) GracefulClose(seconds int, message string) {
	bot.ChannelsMapLock.RLock()
	for _, v := range bot.GlobalBroadcastChannels {
		if v != nil {
			*v <- BroadcastMessage{
				Type:    GRACEFUL_SHUTDOWN,
				Data:    seconds,
				Message: message,
			}
		}
	}
	bot.ChannelsMapLock.RUnlock()
}
func (bot *Bot) Close() {
	bot.SessionManager.Close()
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
		bot.StorageInterface.PublishConnectUpdate(msg, "true")
	})
	server.OnEvent("/", "lobby", func(s socketio.Conn, msg string) {
		log.Println("lobby:", msg)
		lobby := game.Lobby{}
		err := json.Unmarshal([]byte(msg), &lobby)
		if err != nil {
			log.Println(err)
		} else {
			if cCode, ok := bot.ConnsToGames[s.ID()]; ok {
				bot.StorageInterface.PublishLobbyUpdate(cCode, msg)
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
				bot.StorageInterface.PublishPhaseUpdate(cCode, msg)
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
				bot.StorageInterface.PublishPlayerUpdate(cCode, msg)
			}
		}
	})
	server.OnError("/", func(s socketio.Conn, e error) {
		log.Println("meet error:", e)
	})
	server.OnDisconnect("/", func(s socketio.Conn, reason string) {
		log.Println("Client connection closed: ", reason)

		if cCode, ok := bot.ConnsToGames[s.ID()]; ok {
			bot.StorageInterface.PublishConnectUpdate(cCode, "false")
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

	//connCode := bot.ConnsToGames[socketID]
	delete(bot.ConnsToGames, socketID)
	//bot.LinkCodeLock.Lock()
	//for i, v := range bot.LinkCodes {
	//	//delete the association between the link code and the guild
	//	if v == previousGid {
	//		delete(bot.LinkCodes, i)
	//		break
	//	}
	//}
	//bot.LinkCodeLock.Unlock()

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
					*v <- BroadcastMessage{
						Type:    GRACEFUL_SHUTDOWN,
						Data:    1,
						Message: fmt.Sprintf("**I haven't received any messages from your capture in %d seconds, so I'm ending the game!**", bot.captureTimeout),
					}
				}
			}
			//
			//bot.LinkCodeLock.Lock()
			//for i, v := range bot.LinkCodes {
			//	//delete the association between the link code and the guild
			//	if v == guildID {
			//		delete(bot.LinkCodes, i)
			//		break
			//	}
			//}
			//bot.LinkCodeLock.Unlock()
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
				if v != nil {
					*v <- BroadcastMessage{
						Type:    GRACEFUL_SHUTDOWN,
						Data:    30,
						Message: fmt.Sprintf("I'm being shut down in %d seconds, and will be closing your active game!", 30),
					}
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

func (bot *Bot) SubscribeToGameByConnectCode(guildID, connectCode string) {
	connection, lobby, phase, player := bot.StorageInterface.SubscribeToGame(connectCode)
	for {
		select {
		case gameMessage := <-connection:
			log.Println(gameMessage)
			aud := bot.StorageInterface.GetAmongUsData(connectCode)
			dgs := bot.StorageInterface.GetDiscordGameState(guildID, "", "", connectCode)
			if gameMessage.Payload == "true" {
				dgs.Linked = true
			} else {
				dgs.Linked = false
			}
			dgs.ConnectCode = connectCode
			dgs.GameStateMsg.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(aud, dgs))
			bot.StorageInterface.SetDiscordGameState(guildID, dgs)
			break
		case gameMessage := <-lobby:
			aud := bot.StorageInterface.GetAmongUsData(connectCode)
			dgs := bot.StorageInterface.GetDiscordGameState(guildID, "", "", connectCode)
			var lobby game.Lobby
			err := json.Unmarshal([]byte(gameMessage.Payload), &lobby)
			if err != nil {
				log.Println(err)
				break
			}
			bot.processLobby(aud, dgs, bot.SessionManager.GetPrimarySession(), lobby)
			bot.StorageInterface.SetAmongUsData(connectCode, aud)
			break
		case gameMessage := <-phase:
			aud := bot.StorageInterface.GetAmongUsData(connectCode)
			dgs := bot.StorageInterface.GetDiscordGameState(guildID, "", "", connectCode)
			sett := bot.StorageInterface.GetDiscordSettings(guildID)
			var phase game.Phase
			err := json.Unmarshal([]byte(gameMessage.Payload), &phase)
			if err != nil {
				log.Println(err)
				break
			}
			updatedDGS := bot.processTransition(aud, dgs, sett, phase)
			bot.StorageInterface.SetAmongUsData(connectCode, aud)
			if updatedDGS {
				bot.StorageInterface.SetDiscordGameState(guildID, dgs)
			}
			break
		case gameMessage := <-player:
			aud := bot.StorageInterface.GetAmongUsData(connectCode)
			dgs := bot.StorageInterface.GetDiscordGameState(guildID, "", "", connectCode)
			sett := bot.StorageInterface.GetDiscordSettings(guildID)
			var player game.Player
			err := json.Unmarshal([]byte(gameMessage.Payload), &player)
			if err != nil {
				log.Println(err)
				break
			}
			updatedDGS := bot.processPlayer(aud, dgs, sett, player)
			if updatedDGS {
				bot.StorageInterface.SetDiscordGameState(guildID, dgs)
			}
			break
		}
	}
}
func (bot *Bot) processPlayer(aud *game.AmongUsData, dgs *DiscordGameState, sett *storage.GuildSettings, player game.Player) bool {
	if player.Name != "" {

		if player.Disconnected || player.Action == game.LEFT {
			log.Println("I detected that " + player.Name + " disconnected or left! " +
				"I'm removing their linked game data; they will need to relink")

			dgs.UserData.ClearPlayerDataByPlayerName(player.Name)
			aud.ClearPlayerData(player.Name)
			dgs.GameStateMsg.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(aud, dgs))
			return true
		} else {
			updated, isAliveUpdated := aud.UpdatePlayer(player)

			if player.Action == game.JOINED {
				log.Println("Detected a player joined, refreshing user data mappings")
				data, found := aud.GetByName(player.Name)
				if !found {
					log.Println("No player data found for " + player.Name)
				} else {
					//if any discord user has this name, make sure to update their data
					dgs.UserData.UpdatePlayerMappingByName(player.Name, &data)
				}
			}

			//TODO this control flow needs to be improved and simplified, for sure
			//Fetch all the cache mappings for the in-game name in question (in-game -> discord hash).
			//Hash the users in VC who aren't linked
			//then iterate over the cache mappings, and see if any of the usernames match. This works across servers...
			if updated {
				data, found := aud.GetByName(player.Name)
				if found {
					_, _, _ = dgs.UserData.AttemptPairingByMatchingNames(player.Name, &data)
					//log.Println("Player update received caused an update in cached state")
					if isAliveUpdated && aud.GetPhase() == game.TASKS {
						if sett.GetUnmuteDeadDuringTasks() {
							// unmute players even if in tasks because unmuteDeadDuringTasks is true
							dgs.handleTrackedMembers(bot.SessionManager, sett, 0, NoPriority, game.TASKS)
							dgs.GameStateMsg.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(aud, dgs))
						} else {
							log.Println("NOT updating the discord status message; would leak info")
						}
					} else {
						dgs.GameStateMsg.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(aud, dgs))
					}
				}
			} else {
				//log.Println("Player update received did not cause an update in cached state")
			}
		}

	}

	return false
}

func (bot *Bot) processTransition(aud *game.AmongUsData, dgs *DiscordGameState, sett *storage.GuildSettings, phase game.Phase) bool {
	oldPhase := aud.UpdatePhase(phase)
	if oldPhase == phase {
		return false
	}

	switch phase {
	case game.MENU:
		dgs.GameStateMsg.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(aud, dgs))
		dgs.GameStateMsg.RemoveAllReactions(bot.SessionManager.GetPrimarySession())
		return false //this doesn't change the DGS from OUR perspective
	case game.LOBBY:
		delay := sett.Delays.GetDelay(oldPhase, phase)
		dgs.handleTrackedMembers(bot.SessionManager, sett, delay, NoPriority, phase)
		dgs.GameStateMsg.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(aud, dgs))
		dgs.GameStateMsg.AddAllReactions(bot.SessionManager.GetPrimarySession(), bot.StatusEmojis[true])
		return true
	case game.TASKS:
		delay := sett.Delays.GetDelay(oldPhase, phase)
		//when going from discussion to tasks, we should mute alive players FIRST
		priority := AlivePriority
		if oldPhase == game.LOBBY {
			priority = NoPriority
		}
		dgs.handleTrackedMembers(bot.SessionManager, sett, delay, priority, phase)
		dgs.GameStateMsg.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(aud, dgs))
		return true
	case game.DISCUSS:
		delay := sett.Delays.GetDelay(oldPhase, phase)

		dgs.handleTrackedMembers(bot.SessionManager, sett, delay, DeadPriority, aud.GetPhase())

		dgs.GameStateMsg.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(aud, dgs))
		return true
	}
	return false
}

func (bot *Bot) processLobby(aud *game.AmongUsData, dgs *DiscordGameState, s *discordgo.Session, lobby game.Lobby) {
	aud.SetRoomRegion(lobby.LobbyCode, lobby.Region.ToString())

	dgs.GameStateMsg.Edit(s, bot.gameStateResponse(aud, dgs))
}

//func (bot *Bot) updatesListener() func(dg *discordgo.Session, guildID string, socketUpdates *chan SocketStatus, phaseUpdates *chan game.Phase, playerUpdates *chan game.Player, lobbyUpdates *chan LobbyStatus, globalUpdates *chan BroadcastMessage) {
//	return func(dg *discordgo.Session, guildID string, socketUpdates *chan SocketStatus, phaseUpdates *chan game.Phase, playerUpdates *chan game.Player, lobbyUpdates *chan LobbyStatus, globalUpdates *chan BroadcastMessage) {
//		for {
//			select {
//
//			case worldUpdate := <-*globalUpdates:
//				if guild, ok := bot.AllGuilds[guildID]; ok {
//					if worldUpdate.Type == GRACEFUL_SHUTDOWN {
//						go bot.gracefulShutdownWorker(dg, guild, worldUpdate.Data, worldUpdate.Message)
//
//						bot.LinkCodeLock.Lock()
//						for i, v := range bot.LinkCodes {
//							//delete the association between the link code and the guild
//							if v == guildID {
//								delete(bot.LinkCodes, i)
//								break
//							}
//						}
//						bot.LinkCodeLock.Unlock()
//						guild.Linked = false
//					}
//				}
//			}
//		}
//	}
//}
//
//func (bot *Bot) gracefulShutdownWorker(s *discordgo.Session, dgs *DiscordGameState, seconds int, message string) {
//	if dgs.GameStateMsg.MessageID != "" {
//		log.Printf("**Received graceful shutdown message, shutting down in %d seconds**", seconds)
//
//		sendMessage(s, dgs.GameStateMsg.MessageChannelID, message)
//	}
//
//	time.Sleep(time.Duration(seconds) * time.Second)
//
//	bot.endGame(dgs, s)
//}

// Gets called whenever a voice state change occurs
func (bot *Bot) voiceStateChange(s *discordgo.Session, m *discordgo.VoiceStateUpdate) {
	sett := bot.StorageInterface.GetDiscordSettings(m.GuildID)
	bot.handleVoiceStateChange(sett, s, m)
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the authenticated bot has access to.
func (bot *Bot) messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	sett := bot.StorageInterface.GetDiscordSettings(m.GuildID)
	bot.handleMessageCreate(sett, s, m)
}

//this function is called whenever a reaction is created in a guild
func (bot *Bot) reactionCreate(s *discordgo.Session, m *discordgo.MessageReactionAdd) {

	sett := bot.StorageInterface.GetDiscordSettings(m.GuildID)
	bot.handleReactionGameStartAdd(sett, s, m)
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
		bot.GlobalBroadcastChannels[m.Guild.ID] = &globalUpdates
		bot.ChannelsMapLock.Unlock()

		dsg := NewDiscordGameState(m.Guild.ID)

		//put an empty entry in Redis
		bot.StorageInterface.SetDiscordGameState(m.Guild.ID, dsg)

		//go bot.updatesListener()(s, m.Guild.ID, &socketUpdates, &phaseUpdates, &playerUpdates, &lobbyUpdates, &globalUpdates)
	}
}

func (bot *Bot) newAltGuild(s *discordgo.Session, m *discordgo.GuildCreate) {
	bot.SessionManager.RegisterGuildSecondSession(m.Guild.ID)
}

func (bot *Bot) handleMessageCreate(sett *storage.GuildSettings, s *discordgo.Session, m *discordgo.MessageCreate) {
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
	prefix := sett.GetCommandPrefix()

	if strings.HasPrefix(contents, prefix) {
		//either BOTH the admin/roles are empty, or the user fulfills EITHER perm "bucket"
		perms := sett.EmptyAdminAndRolePerms()
		if !perms {
			perms = sett.HasAdminPerms(m.Member) || sett.HasRolePerms(m.Member)
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

func (bot *Bot) handleReactionGameStartAdd(sett *storage.GuildSettings, s *discordgo.Session, m *discordgo.MessageReactionAdd) {
	g, err := s.State.Guild(m.GuildID)
	if err != nil {
		log.Println(err)
		return
	}

	dgs := bot.StorageInterface.GetDiscordGameState(m.GuildID, m.ChannelID, "", "")

	if dgs != nil && dgs.GameStateMsg.Exists() {
		//verify that the user is reacting to the state/status message
		if dgs.GameStateMsg.IsReactionTo(m) {
			idMatched := false
			aud := bot.StorageInterface.GetAmongUsData(dgs.ConnectCode)
			//completely ignore any reactions if a game isn't going
			if aud == nil {
				return
			}
			for color, e := range bot.StatusEmojis[true] {
				if e.ID == m.Emoji.ID {
					idMatched = true
					log.Print(fmt.Sprintf("Player %s reacted with color %s\n", m.UserID, game.GetColorStringForInt(color)))
					//the user doesn't exist in our userdata cache; add them
					_, added := dgs.checkCacheAndAddUser(g, s, m.UserID)
					if !added {
						log.Println("No users found in Discord for userID " + m.UserID)
					}

					playerData, found := aud.GetByColor(game.GetColorStringForInt(color))
					if found {
						dgs.UserData.UpdatePlayerData(m.UserID, &playerData)
					} else {
						log.Println("I couldn't find any player data for that color; is your capture linked?")
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
					dgs.UserData.ClearPlayerData(m.UserID)
					err := s.MessageReactionRemove(m.ChannelID, m.MessageID, "❌", m.UserID)
					if err != nil {
						log.Println(err)
					}
					idMatched = true
				}
			}
			//make sure to update any voice changes if they occurred
			if idMatched {
				dgs.handleTrackedMembers(bot.SessionManager, sett, 0, NoPriority, aud.GetPhase())
				dgs.GameStateMsg.Edit(s, bot.gameStateResponse(aud, dgs))
			}
		}
	}
}

//voiceStateChange handles more edge-case behavior for users moving between voice channels, and catches when
//relevant discord api requests are fully applied successfully. Otherwise, we can issue multiple requests for
//the same mute/unmute, erroneously
func (bot *Bot) handleVoiceStateChange(sett *storage.GuildSettings, s *discordgo.Session, m *discordgo.VoiceStateUpdate) bool {
	//dgs := bot.StorageInterface.
	dgs := bot.StorageInterface.GetDiscordGameState(m.GuildID, "", m.ChannelID, "")
	if dgs == nil {
		return false
	}
	aud := bot.StorageInterface.GetAmongUsData(dgs.ConnectCode)
	if aud == nil {
		return false
	}

	g := dgs.verifyVoiceStateChanges(s, sett, aud.GetPhase())

	if g == nil {
		return false
	}

	updateMade := false

	//fetch the userData from our userData data cache
	userData, err := dgs.UserData.GetUser(m.UserID)
	if err != nil {
		//the user doesn't exist in our userdata cache; add them
		userData, _ = dgs.checkCacheAndAddUser(g, s, m.UserID)
	}
	tracked := m.ChannelID != "" && dgs.Tracking.ChannelID == m.ChannelID
	//only actually tracked if we're in a tracked channel AND linked to a player
	tracked = tracked && userData.IsLinked()
	mute, deaf := sett.GetVoiceState(userData.IsAlive(), tracked, aud.GetPhase())
	//check the userdata is linked here to not accidentally undeafen music bots, for example
	if userData.IsLinked() && !userData.IsPendingVoiceUpdate() && (mute != m.Mute || deaf != m.Deaf) {
		userData.SetPendingVoiceUpdate(true)

		dgs.UserData.UpdateUserData(m.UserID, userData)

		nick := userData.GetPlayerName()
		if !sett.GetApplyNicknames() {
			nick = ""
		}

		go guildMemberUpdate(s, UserPatchParameters{m.GuildID, userData, deaf, mute, nick})

		//log.Println("Applied deaf/undeaf mute/unmute via voiceStateChange")

		updateMade = true
	}

	return updateMade
}

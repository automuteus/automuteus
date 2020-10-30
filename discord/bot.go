package discord

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
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

type SocketStatus struct {
	GuildID   string
	Connected bool
}

type SessionManager struct {
	PrimarySession *discordgo.Session
	AltSession     *discordgo.Session
	//AltSessionGuilds is a record of which guilds also have the 2nd bot added to them (and therefore should be allowed to
	//use the 2nd bot token
	AltSessionGuilds map[string]struct{}
	count            int
	lock             sync.RWMutex
}

func NewSessionManager(primary, secondary *discordgo.Session) SessionManager {
	return SessionManager{
		PrimarySession:   primary,
		AltSession:       secondary,
		AltSessionGuilds: make(map[string]struct{}),
		count:            0,
		lock:             sync.RWMutex{},
	}
}

func (sm *SessionManager) GetPrimarySession() *discordgo.Session {
	return sm.PrimarySession
}

func (sm *SessionManager) GetSessionForRequest(guildID string) *discordgo.Session {
	if sm.AltSession == nil {
		return sm.PrimarySession
	}
	sm.lock.Lock()
	defer sm.lock.Unlock()

	//only bother using a separate token/session if the guild also has that bot invited/a member
	if _, hasSecond := sm.AltSessionGuilds[guildID]; hasSecond {
		sm.count++
		if sm.count%2 == 0 {
			log.Println("Using primary session for request")
			return sm.PrimarySession
		} else {
			log.Println("Using secondary session for request")
			return sm.AltSession
		}
	} else {
		log.Println("Using primary session for request")
		return sm.PrimarySession
	}
}

func (sm *SessionManager) Close() {
	if sm.PrimarySession != nil {
		sm.PrimarySession.Close()
	}

	if sm.AltSession != nil {
		sm.AltSession.Close()
	}
}

func (sm *SessionManager) RegisterGuildSecondSession(guildID string) {
	sm.lock.Lock()
	sm.AltSessionGuilds[guildID] = struct{}{}
	sm.lock.Unlock()
}

type Bot struct {
	url                     string
	internalPort            string
	AllConns                map[string]string
	AllGuilds               map[string]*GuildState
	LinkCodes               map[GameOrLobbyCode]string
	GamePhaseUpdateChannels map[string]*chan game.Phase

	PlayerUpdateChannels map[string]*chan game.Player

	SocketUpdateChannels map[string]*chan SocketStatus

	GlobalBroadcastChannels map[string]*chan BroadcastMessage

	LobbyUpdateChannels map[string]*chan LobbyStatus

	LinkCodeLock sync.RWMutex

	ChannelsMapLock sync.RWMutex

	SessionManager SessionManager

	StorageInterface storage.StorageInterface

	UserSettings *storage.UserSettingsCollection

	logPath string

	captureTimeout int
}

func (bot *Bot) PushGuildSocketUpdate(guildID string, status SocketStatus) {
	bot.ChannelsMapLock.RLock()
	channel := bot.SocketUpdateChannels[guildID]
	if channel != nil {
		*channel <- status
	}
	bot.ChannelsMapLock.RUnlock()
}

func (bot *Bot) PushGuildPlayerUpdate(guildID string, status game.Player) {
	bot.ChannelsMapLock.RLock()
	channel := bot.PlayerUpdateChannels[guildID]
	if channel != nil {
		*channel <- status
	}
	bot.ChannelsMapLock.RUnlock()
}

func (bot *Bot) PushGuildPhaseUpdate(guildID string, status game.Phase) {
	bot.ChannelsMapLock.RLock()
	channel := bot.GamePhaseUpdateChannels[guildID]
	if channel != nil {
		*channel <- status
	}
	bot.ChannelsMapLock.RUnlock()
}

func (bot *Bot) PushGuildLobbyUpdate(guildID string, status LobbyStatus) {
	bot.ChannelsMapLock.RLock()
	channel := bot.LobbyUpdateChannels[guildID]
	if channel != nil {
		*channel <- status
	}
	bot.ChannelsMapLock.RUnlock()
}

var Version string

// MakeAndStartBot does what it sounds like
//TODO collapse these fields into proper structs?
func MakeAndStartBot(version, token, token2, url, internalPort, emojiGuildID string, numShards, shardID int, storageClient storage.StorageInterface, logPath string, timeoutSecs int) *Bot {
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
		url:                     url,
		internalPort:            internalPort,
		AllConns:                make(map[string]string),
		AllGuilds:               make(map[string]*GuildState),
		LinkCodes:               make(map[GameOrLobbyCode]string),
		GamePhaseUpdateChannels: make(map[string]*chan game.Phase),
		PlayerUpdateChannels:    make(map[string]*chan game.Player),
		SocketUpdateChannels:    make(map[string]*chan SocketStatus),
		GlobalBroadcastChannels: make(map[string]*chan BroadcastMessage),
		LobbyUpdateChannels:     make(map[string]*chan LobbyStatus),
		LinkCodeLock:            sync.RWMutex{},
		ChannelsMapLock:         sync.RWMutex{},
		SessionManager:          NewSessionManager(dg, altDiscordSession),
		StorageInterface:        storageClient,
		UserSettings:            storageClient.GetAllUserSettings(),
		logPath:                 logPath,
		captureTimeout:          timeoutSecs,
	}

	dg.AddHandler(bot.voiceStateChange())
	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(bot.messageCreate())
	dg.AddHandler(bot.reactionCreate())
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

func (bot *Bot) guildIDForCode(code string) string {
	if code == "" {
		return ""
	}
	bot.LinkCodeLock.RLock()
	defer bot.LinkCodeLock.RUnlock()
	for codes, gid := range bot.LinkCodes {
		if code != "" {
			if codes.gameCode == code || codes.connectCode == code {
				return gid
			}
		}
	}
	return ""
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

		guildID := bot.guildIDForCode(msg)
		if guildID == "" {
			log.Printf("No guild has the current connect code of %s\n", msg)
			return
		}
		//only link the socket to guilds that we actually have a record of
		if guild, ok := bot.AllGuilds[guildID]; ok {
			bot.AllConns[s.ID()] = guildID
			guild.Linked = true
			if v, ok := inactiveWorkerChannels[s.ID()]; ok {
				v <- guildID
			}

			bot.PushGuildSocketUpdate(guildID, SocketStatus{
				GuildID:   guildID,
				Connected: true,
			})
		}

		log.Printf("Associated websocket id %s with guildID %s using code %s\n", s.ID(), guildID, msg)
		//s.Emit("reply", "set guildID successfully")
	})
	server.OnEvent("/", "lobby", func(s socketio.Conn, msg string) {
		log.Println("lobby:", msg)
		lobby := game.Lobby{}
		err := json.Unmarshal([]byte(msg), &lobby)
		if err != nil {
			log.Println(err)
		} else {
			guildID := ""

			//TODO race condition
			if gid, ok := bot.AllConns[s.ID()]; ok {
				guildID = gid
			} else {
				guildID = bot.guildIDForCode(lobby.LobbyCode)
			}

			if guildID != "" {
				if v, ok := inactiveWorkerChannels[s.ID()]; ok {
					v <- guildID
				}
				if _, ok := bot.AllGuilds[guildID]; ok { // Game is connected -> update its room code
					log.Println("Received room code", msg, "for guild", guildID, "from capture")
				} else {
					bot.PushGuildSocketUpdate(guildID, SocketStatus{
						GuildID:   guildID,
						Connected: true,
					})
					log.Println("Associated lobby with existing game!")
				}
				//we went to lobby, so set the phase. Also adds the initial reaction emojis
				bot.PushGuildPhaseUpdate(guildID, game.LOBBY)
				if bot.AllConns[s.ID()] != guildID {
					bot.AllConns[s.ID()] = guildID
				}
				bot.PushGuildLobbyUpdate(guildID, LobbyStatus{
					GuildID: guildID,
					Lobby:   lobby,
				})
			} else {
				log.Println("I don't have a record of any games with a lobby or connect code of " + lobby.LobbyCode)
			}
		}
	})
	server.OnEvent("/", "state", func(s socketio.Conn, msg string) {
		log.Println("phase received from capture: ", msg)
		phase, err := strconv.Atoi(msg)
		if err != nil {
			log.Println(err)
		} else {
			if gid, ok := bot.AllConns[s.ID()]; ok && gid != "" {
				if v, ok := inactiveWorkerChannels[s.ID()]; ok {
					v <- gid
				}
				log.Println("Pushing phase event to channel")
				bot.PushGuildPhaseUpdate(gid, game.Phase(phase))
			} else {
				log.Println("This websocket is not associated with any guilds")
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
			if gid, ok := bot.AllConns[s.ID()]; ok && gid != "" {
				if v, ok := inactiveWorkerChannels[s.ID()]; ok {
					v <- gid
				}
				bot.PushGuildPlayerUpdate(gid, player)
			} else {
				log.Println("This websocket is not associated with any guilds")
			}
		}
	})
	server.OnError("/", func(s socketio.Conn, e error) {
		log.Println("meet error:", e)
	})
	server.OnDisconnect("/", func(s socketio.Conn, reason string) {
		log.Println("Client connection closed: ", reason)

		bot.PurgeConnection(s.ID())
	})
	go server.Serve()
	defer server.Close()

	//http.Handle("/socket.io/", server)

	router := mux.NewRouter()
	router.Handle("/socket.io/", server)
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Auto-Mute Us is up and running.")
	})

	log.Printf("Serving at localhost:%s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}

func (bot *Bot) PurgeConnection(socketID string) {

	previousGid := bot.AllConns[socketID]
	delete(bot.AllConns, socketID)
	//bot.LinkCodeLock.Lock()
	//for i, v := range bot.LinkCodes {
	//	//delete the association between the link code and the guild
	//	if v == previousGid {
	//		delete(bot.LinkCodes, i)
	//		break
	//	}
	//}
	//bot.LinkCodeLock.Unlock()

	for gid, guild := range bot.AllGuilds {
		if gid == previousGid {
			bot.LinkCodeLock.Lock()
			guild.Linked = false
			bot.LinkCodeLock.Unlock()
			bot.PushGuildSocketUpdate(gid, SocketStatus{
				GuildID:   gid,
				Connected: false,
			})

			log.Printf("Deassociated websocket id %s with guildID %s\n", socketID, gid)
		}
	}
	log.Print("Done purging")
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
			"activeConnections": len(bot.AllConns),
			"activeGames":       len(bot.LinkCodes), //probably an inaccurate metric
			"totalGuilds":       len(bot.AllGuilds),
		}
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			log.Println(err)
		}
		w.Write(jsonBytes)
	})

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func (bot *Bot) updatesListener() func(dg *discordgo.Session, guildID string, socketUpdates *chan SocketStatus, phaseUpdates *chan game.Phase, playerUpdates *chan game.Player, lobbyUpdates *chan LobbyStatus, globalUpdates *chan BroadcastMessage) {
	return func(dg *discordgo.Session, guildID string, socketUpdates *chan SocketStatus, phaseUpdates *chan game.Phase, playerUpdates *chan game.Player, lobbyUpdates *chan LobbyStatus, globalUpdates *chan BroadcastMessage) {
		for {
			select {

			case phase := <-*phaseUpdates:

				log.Printf("Received PhaseUpdate message for guild %s\n", guildID)
				if guild, ok := bot.AllGuilds[guildID]; ok {
					if !guild.GameRunning {
						//completely ignore events if the game is ended/paused
						break
					}
					switch phase {
					case game.MENU:
						if guild.AmongUsData.GetPhase() == game.MENU {
							break
						}
						log.Println("Detected transition to Menu")
						guild.AmongUsData.SetRoomRegion("Unprovided", "Unprovided")
						guild.AmongUsData.SetPhase(phase)
						guild.GameStateMsg.Edit(dg, gameStateResponse(guild))
						guild.GameStateMsg.RemoveAllReactions(dg)
						break
					case game.LOBBY:
						if guild.AmongUsData.GetPhase() == game.LOBBY {
							break
						}
						log.Println("Detected transition to Lobby")

						delay := guild.GetDelay(guild.AmongUsData.GetPhase(), game.LOBBY)

						guild.AmongUsData.SetAllAlive()
						guild.AmongUsData.SetPhase(phase)

						//going back to the lobby, we have no preference on who gets applied first
						guild.handleTrackedMembers(&bot.SessionManager, delay, NoPriority)

						guild.GameStateMsg.Edit(dg, gameStateResponse(guild))

						guild.GameStateMsg.AddAllReactions(dg, guild.StatusEmojis[true])
						break
					case game.TASKS:
						if guild.AmongUsData.GetPhase() == game.TASKS {
							break
						}
						log.Println("Detected transition to Tasks")
						oldPhase := guild.AmongUsData.GetPhase()
						delay := guild.GetDelay(oldPhase, game.TASKS)
						//when going from discussion to tasks, we should mute alive players FIRST
						priority := AlivePriority

						if oldPhase == game.LOBBY {
							//when we go from lobby to tasks, mark all users as alive to be sure
							guild.AmongUsData.SetAllAlive()
							priority = NoPriority
						}

						guild.AmongUsData.SetPhase(phase)

						guild.handleTrackedMembers(&bot.SessionManager, delay, priority)

						guild.GameStateMsg.Edit(dg, gameStateResponse(guild))
						break
					case game.DISCUSS:
						if guild.AmongUsData.GetPhase() == game.DISCUSS {
							break
						}
						log.Println("Detected transition to Discussion")

						delay := guild.GetDelay(guild.AmongUsData.GetPhase(), game.DISCUSS)

						guild.AmongUsData.SetPhase(phase)

						guild.handleTrackedMembers(&bot.SessionManager, delay, DeadPriority)

						guild.GameStateMsg.Edit(dg, gameStateResponse(guild))
						break
					default:
						log.Printf("Undetected new state: %d\n", phase)
					}
				}

			case player := <-*playerUpdates:
				log.Printf("Received PlayerUpdate message for guild %s\n", guildID)
				if guild, ok := bot.AllGuilds[guildID]; ok {
					if !guild.GameRunning {
						break
					}

					//	this updates the copies in memory
					//	(player's associations to amongus data are just pointers to these structs)
					if player.Name != "" {
						if player.Action == game.EXILED {
							log.Println("Detected player EXILE event, marking as dead")
							player.IsDead = true
						}
						if player.IsDead == true && guild.AmongUsData.GetPhase() == game.LOBBY {
							log.Println("Received a dead event, but we're in the Lobby, so I'm ignoring it")
							player.IsDead = false
						}

						if player.Disconnected || player.Action == game.LEFT {
							log.Println("I detected that " + player.Name + " disconnected or left! " +
								"I'm removing their linked game data; they will need to relink")

							guild.UserData.ClearPlayerDataByPlayerName(player.Name)
							guild.AmongUsData.ClearPlayerData(player.Name)
							guild.GameStateMsg.Edit(dg, gameStateResponse(guild))
						} else {
							updated, isAliveUpdated := guild.AmongUsData.ApplyPlayerUpdate(player)

							if player.Action == game.JOINED {
								log.Println("Detected a player joined, refreshing user data mappings")
								data := guild.AmongUsData.GetByName(player.Name)
								if data == nil {
									log.Println("No player data found for " + player.Name)
								}

								guild.UserData.UpdatePlayerMappingByName(player.Name, data)
							}

							if updated {
								data := guild.AmongUsData.GetByName(player.Name)
								paired, userID, name := guild.UserData.AttemptPairingByMatchingNames(player.Name, data)

								if paired {
									log.Println("Successfully linked discord user to player using matching names!")
									user, found := bot.UserSettings.GetUser(userID)
									already := false
									if !found {
										user = storage.UserSettings{
											UserID:    userID,
											UserName:  name,
											GameNames: []string{player.Name},
										}
									} else {
										for _, v := range user.GameNames {
											if v == player.Name {
												already = true
												break
											}
										}
										if !already {
											user.GameNames = append(user.GameNames, player.Name)
										}
									}
									//if the name was already found/listed, don't bother writing
									if !already {
										bot.UserSettings.UpdateUser(userID, user)
										err := bot.StorageInterface.WriteUserSettings(userID, &user)
										if err != nil {
											log.Println(err)
										}
									}
								} else {
									log.Println("Attempting to link via cached user names")
									id := bot.UserSettings.PairByName(player.Name)
									if id != "" {
										log.Printf("Paired %s to their cached name of %s!\n", id, player.Name)
										guild.UserData.UpdatePlayerData(id, data)
									}
								}

								//log.Println("Player update received caused an update in cached state")
								if isAliveUpdated && guild.AmongUsData.GetPhase() == game.TASKS {
									if guild.guildSettings.GetUnmuteDeadDuringTasks() {
										// unmute players even if in tasks because unmuteDeadDuringTasks is true
										guild.handleTrackedMembers(&bot.SessionManager, 0, NoPriority)
										guild.GameStateMsg.Edit(dg, gameStateResponse(guild))
									} else {
										log.Println("NOT updating the discord status message; would leak info")
									}
								} else {
									guild.GameStateMsg.Edit(dg, gameStateResponse(guild))
								}
							} else {
								//log.Println("Player update received did not cause an update in cached state")
							}
						}

					}
				}
				break
			case socketUpdate := <-*socketUpdates:
				if guild, ok := bot.AllGuilds[socketUpdate.GuildID]; ok {
					//this automatically updates the game state message on connect or disconnect
					guild.GameStateMsg.Edit(dg, gameStateResponse(guild))
				}
				break

			case worldUpdate := <-*globalUpdates:
				if guild, ok := bot.AllGuilds[guildID]; ok {
					if worldUpdate.Type == GRACEFUL_SHUTDOWN {
						go bot.gracefulShutdownWorker(dg, guild, worldUpdate.Data, worldUpdate.Message)

						bot.LinkCodeLock.Lock()
						for i, v := range bot.LinkCodes {
							//delete the association between the link code and the guild
							if v == guildID {
								delete(bot.LinkCodes, i)
								break
							}
						}
						bot.LinkCodeLock.Unlock()
						guild.Linked = false
					}
				}

			case lobbyUpdate := <-*lobbyUpdates:
				if guild, ok := bot.AllGuilds[lobbyUpdate.GuildID]; ok {
					guild.Linked = true
					guild.AmongUsData.SetRoomRegion(lobbyUpdate.Lobby.LobbyCode, lobbyUpdate.Lobby.Region.ToString()) // Set new room code
					guild.GameStateMsg.Edit(dg, gameStateResponse(guild))                                             // Update game state message
				}
			}
		}
	}
}

func (bot *Bot) gracefulShutdownWorker(s *discordgo.Session, guild *GuildState, seconds int, message string) {
	if guild.GameStateMsg.message != nil {
		log.Printf("**Received graceful shutdown message, shutting down in %d seconds**", seconds)

		sendMessage(s, guild.GameStateMsg.message.ChannelID, message)
	}

	time.Sleep(time.Duration(seconds) * time.Second)

	bot.handleGameEndMessage(guild, s)
}

// Gets called whenever a voice state change occurs
func (bot *Bot) voiceStateChange() func(s *discordgo.Session, m *discordgo.VoiceStateUpdate) {
	return func(s *discordgo.Session, m *discordgo.VoiceStateUpdate) {
		for id, socketGuild := range bot.AllGuilds {
			if id == m.GuildID {
				socketGuild.voiceStateChange(s, m)
				break
			}
		}
	}
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the authenticated bot has access to.
func (bot *Bot) messageCreate() func(s *discordgo.Session, m *discordgo.MessageCreate) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate) {
		for id, socketGuild := range bot.AllGuilds {
			if id == m.GuildID {
				bot.handleMessageCreate(socketGuild, s, m)
				break
			}
		}
	}
}

//this function is called whenever a reaction is created in a guild
func (bot *Bot) reactionCreate() func(s *discordgo.Session, m *discordgo.MessageReactionAdd) {
	return func(s *discordgo.Session, m *discordgo.MessageReactionAdd) {
		for id, socketGuild := range bot.AllGuilds {
			if id == m.GuildID {
				bot.handleReactionGameStartAdd(socketGuild, s, m)
				break
			}
		}
	}
}

func (bot *Bot) newGuild(emojiGuildID string) func(s *discordgo.Session, m *discordgo.GuildCreate) {
	return func(s *discordgo.Session, m *discordgo.GuildCreate) {

		var gs *storage.GuildSettings = nil

		data, err := bot.StorageInterface.GetGuildSettings(m.Guild.ID)
		if err != nil {
			log.Printf("Couldn't load guild data for %s from storageDriver; using default config instead\n", m.Guild.ID)
			log.Printf("Exact error: %s", err)
		} else {
			gs = data
		}
		if gs == nil {
			gs = storage.MakeGuildSettings(m.Guild.ID, m.Guild.Name)
			err := bot.StorageInterface.WriteGuildSettings(m.ID, gs)
			if err != nil {
				log.Printf("Error writing %s guild settings to storage interface: %s\n", m.Guild.ID, err)
			} else {
				log.Printf("Successfully wrote %s guild settings to Storage interface!", m.Guild.ID)
			}

		}

		userSettingsUpdateChannel := make(chan storage.UserSettingsUpdate)

		log.Printf("Added to new Guild, id %s, name %s", m.Guild.ID, m.Guild.Name)

		f, err := os.Create(path.Join(bot.logPath, m.Guild.ID+"_log.txt"))
		w := io.MultiWriter(os.Stdout)
		if err != nil {
			log.Println("Couldn't create logger for " + m.Guild.ID + "; only using stdout for logging")
		} else {
			w = io.MultiWriter(f, os.Stdout)
		}
		bot.AllGuilds[m.ID] = &GuildState{
			Linked: false,

			UserData: MakeUserDataSet(),
			Tracking: MakeTracking(),

			GameStateMsg: MakeGameStateMessage(),

			StatusEmojis:  emptyStatusEmojis(),
			SpecialEmojis: map[string]Emoji{},

			AmongUsData: game.NewAmongUsData(),
			GameRunning: false,

			guildSettings:             gs,
			userSettingsUpdateChannel: userSettingsUpdateChannel,

			logger: log.New(w, fmt.Sprintf("[%s | %s] ", m.Guild.ID, m.Guild.Name), log.LstdFlags|log.Lmsgprefix),
		}

		if emojiGuildID == "" {
			log.Println("[This is not an error] No explicit guildID provided for emojis; using the current guild default")
			emojiGuildID = m.Guild.ID
		}
		allEmojis, err := s.GuildEmojis(emojiGuildID)
		if err != nil {
			log.Println(err)
		} else {
			bot.AllGuilds[m.Guild.ID].addAllMissingEmojis(s, m.Guild.ID, true, allEmojis)

			bot.AllGuilds[m.Guild.ID].addAllMissingEmojis(s, m.Guild.ID, false, allEmojis)

			bot.AllGuilds[m.Guild.ID].addSpecialEmojis(s, m.Guild.ID, allEmojis)
		}

		socketUpdates := make(chan SocketStatus)
		playerUpdates := make(chan game.Player)
		phaseUpdates := make(chan game.Phase)
		lobbyUpdates := make(chan LobbyStatus)
		globalUpdates := make(chan BroadcastMessage)

		bot.ChannelsMapLock.Lock()
		bot.SocketUpdateChannels[m.Guild.ID] = &socketUpdates
		bot.PlayerUpdateChannels[m.Guild.ID] = &playerUpdates
		bot.GamePhaseUpdateChannels[m.Guild.ID] = &phaseUpdates
		bot.LobbyUpdateChannels[m.Guild.ID] = &lobbyUpdates
		bot.GlobalBroadcastChannels[m.Guild.ID] = &globalUpdates
		bot.ChannelsMapLock.Unlock()

		go bot.updatesListener()(s, m.Guild.ID, &socketUpdates, &phaseUpdates, &playerUpdates, &lobbyUpdates, &globalUpdates)

		go bot.userSettingsUpdateWorker(userSettingsUpdateChannel)
	}
}

func (bot *Bot) userSettingsUpdateWorker(channel chan storage.UserSettingsUpdate) {
	for {
		select {
		case update := <-channel:
			log.Println("Storage worker received update: " + update.UserID)
			user, found := bot.UserSettings.GetUser(update.UserID)
			already := false
			if found {
				if update.Type == storage.GAME_NAME {
					for _, v := range user.GameNames {
						if v == update.Value {
							already = true
							break
						}
					}
					if !already {
						user.GameNames = append(user.GameNames, update.Value)
					}
				}
			} else {
				user = storage.UserSettings{
					UserID: update.UserID,
					//TODO no good way to get ahold of this username :/
					UserName:  "",
					GameNames: []string{update.Value},
				}
			}
			if !already {
				bot.UserSettings.UpdateUser(update.UserID, user)
				err := bot.StorageInterface.WriteUserSettings(update.UserID, &user)
				if err != nil {
					log.Println(err)
				}
			}
		}
	}
}

func (bot *Bot) newAltGuild(s *discordgo.Session, m *discordgo.GuildCreate) {
	bot.SessionManager.RegisterGuildSecondSession(m.Guild.ID)
}

func (bot *Bot) handleMessageCreate(guild *GuildState, s *discordgo.Session, m *discordgo.MessageCreate) {
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
	prefix := guild.CommandPrefix()

	if strings.HasPrefix(contents, prefix) {
		//either BOTH the admin/roles are empty, or the user fulfills EITHER perm "bucket"
		perms := guild.EmptyAdminAndRolePerms()
		if !perms {
			perms = guild.HasAdminPerms(m.Author) || guild.HasRolePerms(m.Member)
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
				bot.HandleCommand(guild, s, g, bot.StorageInterface, m, args)
			}

		}
		//Just deletes messages starting with .au

		if guild.GameStateMsg.SameChannel(m.ChannelID) {
			deleteMessage(s, m.ChannelID, m.Message.ID)
		}
	}

}

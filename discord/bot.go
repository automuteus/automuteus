package discord

import (
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"github.com/denverquane/amongusdiscord/storage"
	socketio "github.com/googollee/go-socket.io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const MaintenancePort = "5000"

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
	Type BcastMsgType
	Data int
}

// AllConns mapping of socket IDs to guilds
var AllConns = map[string]string{}

// AllGuilds mapping of guild IDs to GuildState references
var AllGuilds = map[string]*GuildState{}

// LinkCodes maps the game or lobby codes to the guildID
var LinkCodes = map[GameOrLobbyCode]string{}

// LinkCodeLock mutex for above
var LinkCodeLock = sync.RWMutex{}

// GamePhaseUpdateChannels
var GamePhaseUpdateChannels = make(map[string]*chan game.Phase)

var PlayerUpdateChannels = make(map[string]*chan game.Player)

var SocketUpdateChannels = make(map[string]*chan SocketStatus)

var GlobalBroadcastChannels = make(map[string]*chan BroadcastMessage)

var ChannelsMapLock = sync.RWMutex{}

type SocketStatus struct {
	GuildID   string
	Connected bool
}

var BotUrl string
var BotPort string

var Version string

var StorageInterface storage.StorageInterface

// MakeAndStartBot does what it sounds like
//TODO collapse these fields into proper structs?
func MakeAndStartBot(version, token, url, port, emojiGuildID string, numShards, shardID int, storageClient storage.StorageInterface) {
	Version = version
	BotPort = port
	BotUrl = url
	StorageInterface = storageClient

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Println("error creating Discord session,", err)
		return
	}

	if numShards > 0 && shardID > -1 {
		log.Printf("Identifying to the Discord API with %d total shards, and shard ID=%d\n", numShards, shardID)
		dg.ShardCount = numShards
		dg.ShardID = shardID
	}

	dg.AddHandler(voiceStateChange)
	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(messageCreate)
	dg.AddHandler(reactionCreate)
	dg.AddHandler(newGuild(emojiGuildID))

	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildVoiceStates | discordgo.IntentsGuildMessages | discordgo.IntentsGuilds | discordgo.IntentsGuildMessageReactions)

	//Open a websocket connection to Discord and begin listening.
	err = dg.Open()

	if err != nil {
		log.Println("Could not connect Bot to the Discord Servers with error:", err)
		return
	}

	// Wait here until CTRL-C or other term signal is received.
	log.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)

	go socketioServer(port)

	go messagesServer(MaintenancePort)

	<-sc

	StorageInterface.Close()
	dg.Close()
}

func socketioServer(port string) {
	server, err := socketio.NewServer(nil)
	if err != nil {
		log.Fatal(err)
	}
	server.OnConnect("/", func(s socketio.Conn) error {
		s.SetContext("")
		log.Println("connected:", s.ID())
		return nil
	})
	server.OnEvent("/", "connect", func(s socketio.Conn, msg string) {
		log.Println("set connect code:", msg)
		guildID := ""
		LinkCodeLock.RLock()
		for codes, gid := range LinkCodes {
			if codes.gameCode == msg || codes.connectCode == msg {
				guildID = gid
				break
			}
		}
		LinkCodeLock.RUnlock()
		if guildID == "" {
			log.Printf("No guild has the current connect code of %s\n", msg)
			return
		}
		//only link the socket to guilds that we actually have a record of
		for gid, guild := range AllGuilds {
			if gid == guildID {
				AllConns[s.ID()] = gid
				guild.LinkCode = ""

				ChannelsMapLock.RLock()
				*SocketUpdateChannels[gid] <- SocketStatus{
					GuildID:   gid,
					Connected: true,
				}
				ChannelsMapLock.RUnlock()
			}
		}

		log.Printf("Associated websocket id %s with guildID %s using code %s\n", s.ID(), guildID, msg)
		s.Emit("reply", "set guildID successfully")
	})
	server.OnEvent("/", "lobby", func(s socketio.Conn, msg string) {
		log.Println("lobby:", msg)
		lobby := game.Lobby{}
		err := json.Unmarshal([]byte(msg), &lobby)
		if err != nil {
			log.Println(err)
		} else {
			lobby.ReduceLobbyCode()
			//TODO race condition
			if gid, ok := AllConns[s.ID()]; ok {
				if gid != "" {
					ChannelsMapLock.RLock()
					*SocketUpdateChannels[gid] <- SocketStatus{
						GuildID:   gid,
						Connected: true,
					}
					ChannelsMapLock.RUnlock()
					log.Println("Associated lobby with existing game!")
				} else {
					log.Println("Couldn't find existing game; use `.au new " + lobby.LobbyCode + "` to connect")
				}
				AllConns[s.ID()] = gid
			} else {
				LinkCodes[GameOrLobbyCode{
					gameCode:    "",
					connectCode: lobby.LobbyCode,
				}] = ""
				log.Println("Couldn't find existing game; use `.au new " + lobby.LobbyCode + "` to connect")
			}
		}
	})

	server.OnEvent("/", "state", func(s socketio.Conn, msg string) {
		log.Println("phase received from capture: ", msg)
		phase, err := strconv.Atoi(msg)
		if err != nil {
			log.Println(err)
		} else {
			if gid, ok := AllConns[s.ID()]; ok && gid != "" {
				log.Println("Pushing phase event to channel")
				ChannelsMapLock.RLock()
				*GamePhaseUpdateChannels[gid] <- game.Phase(phase)
				ChannelsMapLock.RUnlock()
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
			if gid, ok := AllConns[s.ID()]; ok && gid != "" {
				ChannelsMapLock.RLock()
				*PlayerUpdateChannels[gid] <- player
				ChannelsMapLock.RUnlock()
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

		previousGid := AllConns[s.ID()]
		delete(AllConns, s.ID())
		LinkCodeLock.Lock()
		for i, v := range LinkCodes {
			//delete the association between the link code and the guild
			if v == previousGid {
				delete(LinkCodes, i)
				break
			}
		}
		LinkCodeLock.Unlock()

		for gid, guild := range AllGuilds {
			if gid == previousGid {

				code := generateConnectCode(gid) //this is unlinked
				LinkCodeLock.Lock()
				//TODO delete the old combo of link codes
				guild.LinkCode = code
				LinkCodeLock.Unlock()
				ChannelsMapLock.RLock()
				*SocketUpdateChannels[gid] <- SocketStatus{
					GuildID:   gid,
					Connected: false,
				}
				ChannelsMapLock.RUnlock()

				log.Printf("Deassociated websocket id %s with guildID %s\n", s.ID(), gid)
			}
		}
	})
	go server.Serve()
	defer server.Close()

	http.Handle("/socket.io/", server)
	log.Printf("Serving at localhost:%s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func messagesServer(port string) {

	http.HandleFunc("/graceful", func(w http.ResponseWriter, r *http.Request) {
		ChannelsMapLock.RLock()
		for _, v := range GlobalBroadcastChannels {
			*v <- BroadcastMessage{
				Type: GRACEFUL_SHUTDOWN,
				Data: 30,
			}
		}
		ChannelsMapLock.RUnlock()
	})

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func updatesListener(dg *discordgo.Session, guildID string, socketUpdates *chan SocketStatus, phaseUpdates *chan game.Phase, playerUpdates *chan game.Player, globalUpdates *chan BroadcastMessage) {
	for {
		select {

		case phase := <-*phaseUpdates:
			log.Printf("Received PhaseUpdate message for guild %s\n", guildID)
			if guild, ok := AllGuilds[guildID]; ok {
				switch phase {
				case game.MENU:
					log.Println("Detected transition to Menu; not doing anything about it yet")
				case game.LOBBY:
					if guild.AmongUsData.GetPhase() == game.LOBBY {
						break
					}
					log.Println("Detected transition to Lobby")

					delay := guild.PersistentGuildData.Delays.GetDelay(guild.AmongUsData.GetPhase(), game.LOBBY)

					guild.AmongUsData.SetAllAlive()
					guild.AmongUsData.SetPhase(phase)

					//going back to the lobby, we have no preference on who gets applied first
					guild.handleTrackedMembers(dg, delay, NoPriority)

					guild.GameStateMsg.Edit(dg, gameStateResponse(guild))
				case game.TASKS:
					if guild.AmongUsData.GetPhase() == game.TASKS {
						break
					}
					log.Println("Detected transition to Tasks")
					oldPhase := guild.AmongUsData.GetPhase()
					delay := guild.PersistentGuildData.Delays.GetDelay(oldPhase, game.TASKS)
					//when going from discussion to tasks, we should mute alive players FIRST
					priority := AlivePriority

					if oldPhase == game.LOBBY {
						//when we go from lobby to tasks, mark all users as alive to be sure
						guild.AmongUsData.SetAllAlive()
						priority = NoPriority
					}

					guild.AmongUsData.SetPhase(phase)

					guild.handleTrackedMembers(dg, delay, priority)

					guild.GameStateMsg.Edit(dg, gameStateResponse(guild))
				case game.DISCUSS:
					if guild.AmongUsData.GetPhase() == game.DISCUSS {
						break
					}
					log.Println("Detected transition to Discussion")

					delay := guild.PersistentGuildData.Delays.GetDelay(guild.AmongUsData.GetPhase(), game.DISCUSS)

					guild.AmongUsData.SetPhase(phase)

					//when going from
					guild.handleTrackedMembers(dg, delay, DeadPriority)

					guild.GameStateMsg.Edit(dg, gameStateResponse(guild))
				default:
					log.Printf("Undetected new state: %d\n", phase)
				}
			}

			// TODO prevent cases where 2 players are mapped to the same underlying in-game player data
		case player := <-*playerUpdates:
			log.Printf("Received PlayerUpdate message for guild %s\n", guildID)
			if guild, ok := AllGuilds[guildID]; ok {

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

					if player.Disconnected {
						log.Println("I detected that " + player.Name + " disconnected! " +
							"I'm removing their linked game data; they will need to relink")

						guild.UserData.ClearPlayerDataByPlayerName(player.Name)
						guild.GameStateMsg.Edit(dg, gameStateResponse(guild))
					} else {
						updated, isAliveUpdated := guild.AmongUsData.ApplyPlayerUpdate(player)

						if updated {
							//log.Println("Player update received caused an update in cached state")
							if isAliveUpdated && guild.AmongUsData.GetPhase() == game.TASKS {
								log.Println("NOT updating the discord status message; would leak info")
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
			if guild, ok := AllGuilds[socketUpdate.GuildID]; ok {
				//this automatically updates the game state message on connect or disconnect
				guild.GameStateMsg.Edit(dg, gameStateResponse(guild))
			}
			break

		case worldUpdate := <-*globalUpdates:
			if guild, ok := AllGuilds[guildID]; ok {
				if worldUpdate.Type == GRACEFUL_SHUTDOWN {
					log.Printf("Received graceful shutdown message, shutting down in %d seconds", worldUpdate.Data)

					go gracefulShutdownWorker(dg, guild, worldUpdate.Data)
				}
			}
		}
	}
}

func gracefulShutdownWorker(s *discordgo.Session, guild *GuildState, seconds int) {
	if guild.GameStateMsg.message != nil {
		sendMessage(s, guild.GameStateMsg.message.ChannelID, fmt.Sprintf("**I need to go offline to upgrade! Your game/lobby will be ended in %d seconds!**", seconds))
	}

	time.Sleep(time.Duration(seconds) * time.Second)

	guild.handleGameEndMessage(s)
}

// Gets called whenever a voice state change occurs
func voiceStateChange(s *discordgo.Session, m *discordgo.VoiceStateUpdate) {
	for id, socketGuild := range AllGuilds {
		if id == m.GuildID {
			socketGuild.voiceStateChange(s, m)
			break
		}
	}
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the authenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	for id, socketGuild := range AllGuilds {
		if id == m.GuildID {
			socketGuild.handleMessageCreate(s, m)
			break
		}
	}
}

//this function is called whenever a reaction is created in a guild
func reactionCreate(s *discordgo.Session, m *discordgo.MessageReactionAdd) {
	for id, socketGuild := range AllGuilds {
		if id == m.GuildID {
			socketGuild.handleReactionGameStartAdd(s, m)
			break
		}
	}
}

func newGuild(emojiGuildID string) func(s *discordgo.Session, m *discordgo.GuildCreate) {

	return func(s *discordgo.Session, m *discordgo.GuildCreate) {

		var pgd *PersistentGuildData = nil

		data, err := StorageInterface.GetGuildData(m.Guild.ID)
		if err != nil {
			log.Printf("Couldn't load guild data for %s from storageDriver; using default config instead\n", m.Guild.ID)
			log.Printf("Exact error: %s", err)
		} else {
			tempPgd, err := FromData(data)
			if err != nil {
				log.Printf("Couldn't marshal guild data for %s; using default config instead\n", m.Guild.ID)
			} else {
				log.Printf("Successfully loaded config from storagedriver for %s\n", m.Guild.ID)
				pgd = tempPgd
			}
		}
		if pgd == nil {
			pgd = PGDDefault(m.Guild.ID)
			data, err := pgd.ToData()
			if err != nil {
				log.Printf("Error marshalling %s PGD to map(!): %s\n", m.Guild.ID, err)
			} else {
				err := StorageInterface.WriteGuildData(m.Guild.ID, data)
				if err != nil {
					log.Printf("Error writing %s PGD to storage interface: %s\n", m.Guild.ID, err)
				} else {
					log.Printf("Successfully wrote %s PGD to Storage interface!", m.Guild.ID)
				}
			}
		}

		log.Printf("Added to new Guild, id %s, name %s", m.Guild.ID, m.Guild.Name)
		AllGuilds[m.ID] = &GuildState{
			PersistentGuildData: pgd,

			LinkCode: m.Guild.ID,

			UserData:     MakeUserDataSet(),
			Tracking:     MakeTracking(),
			GameStateMsg: MakeGameStateMessage(),

			StatusEmojis:  emptyStatusEmojis(),
			SpecialEmojis: map[string]Emoji{},

			AmongUsData: game.NewAmongUsData(),
		}

		if emojiGuildID == "" {
			log.Println("[This is not an error] No explicit guildID provided for emojis; using the current guild default")
			emojiGuildID = m.Guild.ID
		}
		allEmojis, err := s.GuildEmojis(emojiGuildID)
		if err != nil {
			log.Println(err)
		} else {
			AllGuilds[m.Guild.ID].addAllMissingEmojis(s, m.Guild.ID, true, allEmojis)

			AllGuilds[m.Guild.ID].addAllMissingEmojis(s, m.Guild.ID, false, allEmojis)

			AllGuilds[m.Guild.ID].addSpecialEmojis(s, m.Guild.ID, allEmojis)
		}

		socketUpdates := make(chan SocketStatus)
		playerUpdates := make(chan game.Player)
		phaseUpdates := make(chan game.Phase)
		globalUpdates := make(chan BroadcastMessage)

		ChannelsMapLock.Lock()
		SocketUpdateChannels[m.Guild.ID] = &socketUpdates
		PlayerUpdateChannels[m.Guild.ID] = &playerUpdates
		GamePhaseUpdateChannels[m.Guild.ID] = &phaseUpdates
		GlobalBroadcastChannels[m.Guild.ID] = &globalUpdates
		ChannelsMapLock.Unlock()

		go updatesListener(s, m.Guild.ID, &socketUpdates, &phaseUpdates, &playerUpdates, &globalUpdates)

	}
}

func (guild *GuildState) handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	g, err := s.State.Guild(guild.PersistentGuildData.GuildID)
	if err != nil {
		log.Println(err)
	}

	contents := m.Content

	if strings.HasPrefix(contents, guild.PersistentGuildData.CommandPrefix) {
		//either BOTH the admin/roles are empty, or the user fulfills EITHER perm "bucket"
		perms := len(guild.PersistentGuildData.AdminUserIDs) == 0 && len(guild.PersistentGuildData.PermissionedRoleIDs) == 0
		if !perms {
			perms = guild.HasAdminPermissions(m.Author.ID) || guild.HasRolePermissions(s, m.Author.ID)
		}
		if !perms {
			s.ChannelMessageSend(m.ChannelID, "User does not have the required permissions to execute this command!")
		}
		oldLen := len(contents)
		contents = strings.Replace(contents, guild.PersistentGuildData.CommandPrefix+" ", "", 1)
		if len(contents) == oldLen { //didn't have a space
			contents = strings.Replace(contents, guild.PersistentGuildData.CommandPrefix, "", 1)
		}

		if len(contents) == 0 {
			s.ChannelMessageSend(m.ChannelID, helpResponse(Version, guild.PersistentGuildData.CommandPrefix))
		} else {
			args := strings.Split(contents, " ")

			for i, v := range args {
				args[i] = strings.ToLower(v)
			}
			guild.HandleCommand(s, g, m, args)
		}
		//Just deletes messages starting with .au

		if guild.GameStateMsg.SameChannel(m.ChannelID) {
			deleteMessage(s, m.ChannelID, m.Message.ID)
		}

	}
}

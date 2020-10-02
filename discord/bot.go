package discord

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	socketio "github.com/googollee/go-socket.io"
)

type GuildOrLobbyId struct {
	guildID   string
	lobbyCode string
}

// AllConns mapping of socket IDs to either guild IDs or lobby codes
var AllConns = map[string]GuildOrLobbyId{}

// AllGuilds mapping of guild IDs to GuildState references
var AllGuilds = map[string]*GuildState{}

// LinkCodes maps the code to the guildID
var LinkCodes = map[string]string{}

// LinkCodeLock mutex for above
var LinkCodeLock = sync.RWMutex{}

// GamePhaseUpdateChannels
var GamePhaseUpdateChannels = make(map[string]*chan game.Phase)

var PlayerUpdateChannels = make(map[string]*chan game.Player)

var SocketUpdateChannels = make(map[string]*chan SocketStatus)

var RoomCodeUpdateChannels = make(map[string]*chan RoomCodeStatus)

var ChannelsMapLock = sync.RWMutex{}

type SocketStatus struct {
	GuildID   string
	Connected bool
}

type RoomCodeStatus struct {
	GuildID  string
	RoomCode string
}

// MakeAndStartBot does what it sounds like
func MakeAndStartBot(token string, port string, emojiGuildID string) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Println("error creating Discord session,", err)
		return
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

	<-sc

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
		for code, gid := range LinkCodes {
			if code == msg {
				guildID = gid
				break
			}
		}
		LinkCodeLock.RUnlock()
		if guildID == "" {
			log.Printf("No guild has the current connect code of %s\n", msg)
		}
		for gid, guild := range AllGuilds {
			if gid == guildID {
				if v, ok := AllConns[s.ID()]; ok {
					v.guildID = gid
					AllConns[s.ID()] = v
				} else {
					AllConns[s.ID()] = GuildOrLobbyId{
						guildID:   gid,
						lobbyCode: "",
					}
				}
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
			if v, ok := AllConns[s.ID()]; ok {
				if v.guildID != "" {
					ChannelsMapLock.RLock()
					*SocketUpdateChannels[v.guildID] <- SocketStatus{
						GuildID:   v.guildID,
						Connected: true,
					}
					ChannelsMapLock.RUnlock()
					log.Println("Associated lobby with existing game!")
				} else {
					log.Println("Couldn't find existing game; use `.au new " + lobby.LobbyCode + "` to connect")
				}
				v.lobbyCode = lobby.LobbyCode
				AllConns[s.ID()] = v
			} else {
				AllConns[s.ID()] = GuildOrLobbyId{
					guildID:   "",
					lobbyCode: lobby.LobbyCode,
				}
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
			if v, ok := AllConns[s.ID()]; ok && v.guildID != "" {
				log.Println("Pushing phase event to channel")
				ChannelsMapLock.RLock()
				*GamePhaseUpdateChannels[v.guildID] <- game.Phase(phase)
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
			if v, ok := AllConns[s.ID()]; ok && v.guildID != "" {
				ChannelsMapLock.RLock()
				*PlayerUpdateChannels[v.guildID] <- player
				ChannelsMapLock.RUnlock()
			} else {
				log.Println("This websocket is not associated with any guilds")
			}
		}
	})
	server.OnEvent("/", "roomcode", func(s socketio.Conn, msg string) {
		if gid, ok := AllConns[s.ID()]; ok && gid.guildID != "" {
			if guild, ok := AllGuilds[gid.guildID]; ok {
				log.Println("received room code", msg, "for guild", guild.PersistentGuildData.GuildID, "from capture")
				ChannelsMapLock.RLock()
				*RoomCodeUpdateChannels[gid.guildID] <- RoomCodeStatus{
					GuildID:  gid.guildID,
					RoomCode: msg,
				}
				ChannelsMapLock.RUnlock()
			}
		}
	})
	server.OnError("/", func(s socketio.Conn, e error) {
		log.Println("meet error:", e)
	})
	server.OnDisconnect("/", func(s socketio.Conn, reason string) {
		log.Println("Client connection closed: ", reason)

		previousGid := AllConns[s.ID()].guildID
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
				LinkCodes[code] = guild.PersistentGuildData.GuildID
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

func updatesListener(dg *discordgo.Session, guildID string, socketUpdates *chan SocketStatus, phaseUpdates *chan game.Phase, playerUpdates *chan game.Player, roomCodeUpdates *chan RoomCodeStatus) {
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

		case roomCodeUpdate := <-*roomCodeUpdates:
			if guild, ok := AllGuilds[roomCodeUpdate.GuildID]; ok {
				_, region := guild.AmongUsData.GetRoomRegion()
				guild.AmongUsData.SetRoomRegion(roomCodeUpdate.RoomCode, region) // Set new room code
				guild.GameStateMsg.Edit(dg, gameStateResponse(guild))            // Update game state message
			}
		}
	}
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
		filename := fmt.Sprintf("%s_config.json", m.Guild.ID)
		pgd, err := LoadPGDFromFile(filename)
		if err != nil {
			log.Printf("Couldn't load config from %s; using default config instead", filename)
			log.Printf("Exact error: %s", err)
			pgd = PGDDefault(m.Guild.ID)
			err := pgd.ToFile(filename)
			if err != nil {
				log.Println("Using default config, but could not write that default to " + filename + " with error:")
				log.Println(err)
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
			log.Println("No explicit guildID provided for emojis; using the current guild default")
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
		roomCodeUpdates := make(chan RoomCodeStatus)

		ChannelsMapLock.Lock()
		SocketUpdateChannels[m.Guild.ID] = &socketUpdates
		PlayerUpdateChannels[m.Guild.ID] = &playerUpdates
		GamePhaseUpdateChannels[m.Guild.ID] = &phaseUpdates
		RoomCodeUpdateChannels[m.Guild.ID] = &roomCodeUpdates
		ChannelsMapLock.Unlock()

		go updatesListener(s, m.Guild.ID, &socketUpdates, &phaseUpdates, &playerUpdates, &roomCodeUpdates)

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
		args := strings.Split(contents, " ")[1:]
		for i, v := range args {
			args[i] = strings.ToLower(v)
		}
		if len(args) == 0 {
			s.ChannelMessageSend(m.ChannelID, helpResponse(guild.PersistentGuildData.CommandPrefix))
		} else {
			switch args[0] {
			case "help":
				fallthrough
			case "h":
				s.ChannelMessageSend(m.ChannelID, helpResponse(guild.PersistentGuildData.CommandPrefix))
				break
			case "track":
				fallthrough
			case "t":
				if len(args[1:]) == 0 {
					//TODO print usage of this command specifically
					s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("You used this command incorrectly! Please refer to `%s help` for proper command usage", guild.PersistentGuildData.CommandPrefix))
				} else {
					// have to explicitly check for true. Otherwise, processing the 2-word VC names gets really ugly...
					forGhosts := false
					endIdx := len(args)
					if args[len(args)-1] == "true" || args[len(args)-1] == "t" {
						forGhosts = true
						endIdx--
					}

					channelName := strings.Join(args[1:endIdx], " ")

					channels, err := s.GuildChannels(m.GuildID)
					if err != nil {
						log.Println(err)
					}

					guild.trackChannelResponse(channelName, channels, forGhosts)

					guild.GameStateMsg.Edit(s, gameStateResponse(guild))
				}
				break

			case "link":
				fallthrough
			case "l":
				if len(args[1:]) < 2 {
					//TODO print usage of this command specifically
					s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("You used this command incorrectly! Please refer to `%s help` for proper command usage", guild.PersistentGuildData.CommandPrefix))
				} else {
					guild.linkPlayerResponse(args[1:])

					guild.GameStateMsg.Edit(s, gameStateResponse(guild))
				}
				break
			case "unlink":
				fallthrough
			case "ul":
				fallthrough
			case "u":
				if len(args[1:]) == 0 {
					s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("You used this command incorrectly! Please refer to `%s help` for proper command usage", guild.PersistentGuildData.CommandPrefix))
				} else {

				}
				userID, err := extractUserIDFromMention(args[1])
				if err != nil {
					log.Println(err)
				} else {

					log.Printf("Removing player %s", userID)
					guild.UserData.ClearPlayerData(userID)

					//make sure that any players we remove/unlink get auto-unmuted/undeafened
					guild.verifyVoiceStateChanges(s)

					//update the state message to reflect the player leaving
					guild.GameStateMsg.Edit(s, gameStateResponse(guild))
				}
			case "start":
				fallthrough
			case "s":
				fallthrough
			case "new":
				fallthrough
			case "n":
				room, region := getRoomAndRegionFromArgs(args[1:])

				initialTracking := make([]TrackingChannel, 0)

				//TODO need to send a message to the capture re-questing all the player/game states. Otherwise,
				//we don't have enough info to go off of when remaking the game...
				//if !guild.GameStateMsg.Exists() {
				paired := false
				for i, guildOrRoom := range AllConns {
					if guildOrRoom.lobbyCode == room {
						guildOrRoom.guildID = guild.PersistentGuildData.GuildID
						AllConns[i] = guildOrRoom
						guild.LinkCode = ""
						paired = true
						log.Println("Linked game with existing lobby!")
					}
				}
				if !paired {
					connectCode := generateConnectCode(guild.PersistentGuildData.GuildID)
					log.Println(connectCode)
					LinkCodeLock.Lock()
					LinkCodes[connectCode] = guild.PersistentGuildData.GuildID
					guild.LinkCode = connectCode
					LinkCodeLock.Unlock()
				}

				channels, err := s.GuildChannels(m.GuildID)
				if err != nil {
					log.Println(err)
				}

				for _, channel := range channels {
					if channel.Type == discordgo.ChannelTypeGuildVoice {
						if channel.ID == guild.PersistentGuildData.DefaultTrackedChannel || strings.ToLower(channel.Name) == strings.ToLower(guild.PersistentGuildData.DefaultTrackedChannel) {
							initialTracking = append(initialTracking, TrackingChannel{
								channelID:   channel.ID,
								channelName: channel.Name,
								forGhosts:   false,
							})
							log.Printf("Found initial default channel specified in config: ID %s, Name %s\n", channel.ID, channel.Name)
						}
					}
					for _, v := range g.VoiceStates {
						//if the user is detected in a voice channel
						if v.UserID == m.Author.ID {

							//once we find the channel by ID
							if channel.Type == discordgo.ChannelTypeGuildVoice {
								if channel.ID == v.ChannelID {
									initialTracking = append(initialTracking, TrackingChannel{
										channelID:   channel.ID,
										channelName: channel.Name,
										forGhosts:   false,
									})
									log.Printf("User that typed new is in the \"%s\" voice channel; using that for tracking", channel.Name)
								}
							}

						}
					}
				}

				guild.handleGameStartMessage(s, m, room, region, initialTracking)
				break
			case "end":
				fallthrough
			case "e":
				fallthrough
			case "endgame":
				guild.handleGameEndMessage(s)

				//have to explicitly delete here, because if we use the default delete below, the channelID
				//for the game state message doesn't exist anymore...
				deleteMessage(s, m.ChannelID, m.Message.ID)
				break
			case "force":
				fallthrough
			case "f":
				phase := getPhaseFromArgs(args[1:])
				if phase == game.UNINITIALIZED {
					s.ChannelMessageSend(m.ChannelID, "Sorry, I didn't understand the game phase you tried to force")
				} else {
					//TODO this is ugly, but only for debug really
					ChannelsMapLock.RLock()
					*GamePhaseUpdateChannels[m.GuildID] <- phase
					ChannelsMapLock.RUnlock()
				}

				break
			case "refresh":
				fallthrough
			case "r":
				guild.GameStateMsg.Delete(s) //delete the old message

				//create a new instance of the new one
				guild.GameStateMsg.CreateMessage(s, gameStateResponse(guild), m.ChannelID)

				//add the emojis to the refreshed message
				for _, e := range guild.StatusEmojis[true] {
					guild.GameStateMsg.AddReaction(s, e.FormatForReaction())
				}
				guild.GameStateMsg.AddReaction(s, "❌")
			default:
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Sorry, I didn't understand that command! Please see `%s help` for commands", guild.PersistentGuildData.CommandPrefix))

			}
		}
		//Just deletes messages starting with .au

		if guild.GameStateMsg.SameChannel(m.ChannelID) {
			deleteMessage(s, m.ChannelID, m.Message.ID)
		}

	}
}

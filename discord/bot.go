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

type GameOrLobbyCode struct {
	gameCode    string
	connectCode string
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

var LobbyUpdateChannels = make(map[string]*chan LobbyStatus)

var ChannelsMapLock = sync.RWMutex{}

type SocketStatus struct {
	GuildID   string
	Connected bool
}

var BotUrl string
var BotPort string

type LobbyStatus struct {
	GuildID string
	Lobby   game.Lobby
}

var Version string

// MakeAndStartBot does what it sounds like
func MakeAndStartBot(version, token, url, port, emojiGuildID string, numShards, shardID int) {
	Version = version
	BotPort = port
	BotUrl = url
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
					if guild, ok := AllGuilds[gid]; ok { // Game is connected -> update its room code
						log.Println("received room code", msg, "for guild", guild.PersistentGuildData.GuildID, "from capture")
						ChannelsMapLock.RLock()
						*LobbyUpdateChannels[gid] <- LobbyStatus{
							GuildID: gid,
							Lobby:   lobby,
						}
						ChannelsMapLock.RUnlock()
					} else { // No game connected
						ChannelsMapLock.RLock()
						*SocketUpdateChannels[gid] <- SocketStatus{
							GuildID:   gid,
							Connected: true,
						}
						ChannelsMapLock.RUnlock()
						log.Println("Associated lobby with existing game!")
					}
				} else {
					log.Println("Couldn't find existing game; use `.au new " + lobby.LobbyCode + "` to connect")
				}
				//TODO should probably provide this info via a channel and/or "socketupdatechannels" event
				//this is probably overly coupled
				if v, ok := AllGuilds[gid]; ok {
					v.AmongUsData.SetRoomRegion(lobby.LobbyCode, lobby.Region.ToString())
					AllGuilds[gid] = v
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

func updatesListener(dg *discordgo.Session, guildID string, socketUpdates *chan SocketStatus, phaseUpdates *chan game.Phase, playerUpdates *chan game.Player, lobbyUpdates *chan LobbyStatus) {
	for {
		select {

		case phase := <-*phaseUpdates:
			log.Printf("Received PhaseUpdate message for guild %s\n", guildID)
			if guild, ok := AllGuilds[guildID]; ok {
				switch phase {
				case game.MENU:
					log.Println("Detected transition to Menu")
					guild.UserData.ClearAllPlayerData()
					guild.AmongUsData.ClearAllPlayerData()
					guild.AmongUsData.SetRoomRegion("Unprovided", "Unprovided")
					guild.GameStateMsg.Edit(dg, gameStateResponse(guild))
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

					if player.Disconnected || player.Action == game.LEFT {
						log.Println("I detected that " + player.Name + " disconnected! " +
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

		case lobbyUpdate := <-*lobbyUpdates:
			if guild, ok := AllGuilds[lobbyUpdate.GuildID]; ok {
				guild.AmongUsData.SetRoomRegion(lobbyUpdate.Lobby.LobbyCode, lobbyUpdate.Lobby.Region.ToString()) // Set new room code
				guild.GameStateMsg.Edit(dg, gameStateResponse(guild))                                             // Update game state message
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
		lobbyUpdates := make(chan LobbyStatus)

		ChannelsMapLock.Lock()
		SocketUpdateChannels[m.Guild.ID] = &socketUpdates
		PlayerUpdateChannels[m.Guild.ID] = &playerUpdates
		GamePhaseUpdateChannels[m.Guild.ID] = &phaseUpdates
		LobbyUpdateChannels[m.Guild.ID] = &lobbyUpdates
		ChannelsMapLock.Unlock()

		go updatesListener(s, m.Guild.ID, &socketUpdates, &phaseUpdates, &playerUpdates, &lobbyUpdates)

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
		} else {

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
				switch GetCommandType(args[0]) {
				case Help:
					s.ChannelMessageSend(m.ChannelID, helpResponse(Version, guild.PersistentGuildData.CommandPrefix))
					break
				case Track:
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

				case Link:
					if len(args[1:]) < 2 {
						//TODO print usage of this command specifically
						s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("You used this command incorrectly! Please refer to `%s help` for proper command usage", guild.PersistentGuildData.CommandPrefix))
					} else {
						guild.linkPlayerResponse(s, args[1:])

						guild.GameStateMsg.Edit(s, gameStateResponse(guild))
					}
					break
				case Unlink:
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
				case New:
					room, region := getRoomAndRegionFromArgs(args[1:])

					initialTracking := make([]TrackingChannel, 0)

					//TODO need to send a message to the capture re-questing all the player/game states. Otherwise,
					//we don't have enough info to go off of when remaking the game...
					//if !guild.GameStateMsg.Exists() {
					connectCode := generateConnectCode(guild.PersistentGuildData.GuildID)
					log.Println(connectCode)
					LinkCodeLock.Lock()
					LinkCodes[GameOrLobbyCode{
						gameCode:    "",
						connectCode: connectCode,
					}] = guild.PersistentGuildData.GuildID
					guild.LinkCode = connectCode
					LinkCodeLock.Unlock()

					hyperlink := fmt.Sprintf("aucapture://%s:%s/%s?insecure", BotUrl, BotPort, connectCode)

					var embed = discordgo.MessageEmbed{
						URL:         "",
						Type:        "",
						Title:       "You just started a game!",
						Description: fmt.Sprintf("Click the following link to link your capture: \n <%s>", hyperlink),
						Timestamp:   "",
						Color:       3066993, //GREEN
						Image:       nil,
						Thumbnail:   nil,
						Video:       nil,
						Provider:    nil,
						Author:      nil,
					}

					sendMessageDM(s, m.Author.ID, &embed)

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
					}
					for _, v := range g.VoiceStates {
						//if the user is detected in a voice channel
						if v.UserID == m.Author.ID {

							for _, channel := range g.Channels {
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
				case End:
					guild.handleGameEndMessage(s)

					//have to explicitly delete here, because if we use the default delete below, the channelID
					//for the game state message doesn't exist anymore...
					deleteMessage(s, m.ChannelID, m.Message.ID)
					break
				case Force:
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
				case Refresh:
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
}

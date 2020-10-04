package discord

import (
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	socketio "github.com/googollee/go-socket.io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
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

var ChannelsMapLock = sync.RWMutex{}

type SocketStatus struct {
	GuildID   string
	Connected bool
}

var Version string

// MakeAndStartBot does what it sounds like
func MakeAndStartBot(version, token, port, emojiGuildID string, numShards, shardID int) {
	Version = version
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

func updatesListener(dg *discordgo.Session, guildID string, socketUpdates *chan SocketStatus, phaseUpdates *chan game.Phase, playerUpdates *chan game.Player) {
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
							if isAliveUpdated && guild.AmongUsData.GetPhase() == game.TASKS && !guild.PersistentGuildData.UnmuteDeadDuringTasks {
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

		ChannelsMapLock.Lock()
		SocketUpdateChannels[m.Guild.ID] = &socketUpdates
		PlayerUpdateChannels[m.Guild.ID] = &playerUpdates
		GamePhaseUpdateChannels[m.Guild.ID] = &phaseUpdates
		ChannelsMapLock.Unlock()

		go updatesListener(s, m.Guild.ID, &socketUpdates, &phaseUpdates, &playerUpdates)

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
						guild.linkPlayerResponse(s, m.GuildID, args[1:])

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
				case End:
					guild.handleGameEndMessage(s)

					//have to explicitly delete here, because if we use the default delete below, the channelID
					//for the game state message doesn't exist anymore...
					deleteMessage(s, m.ChannelID, m.Message.ID)
					break
				case Force:
					phase := getPhaseFromString(args[1])
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
				case Settings:
					// if no arg passed, send them list of possible settings to change
					if len(args) == 1 {
						s.ChannelMessageSend(m.ChannelID, "The list of possible settings are:\n"+
							"•`CommandPrefix [prefix]`: Change the bot's prefix in this server\n"+
							"•`DefaultTrackedChannel [voiceChannel]`: Change the voice channel the bot tracks by default\n"+
							"•`AdminUserIDs [user 1] [user 2] [etc]`: Add or remove bot admins a.k.a users that can use commands with the bot\n"+
							"•`ApplyNicknames [true/false]`: Whether the bot should change the nicknames of the players to reflect the player's color\n"+
							"•`UnmuteDeadDuringTasks [true/false]`: Whether the bot should unmute dead players immediately when they die (**WARNING**: reveals information)\n"+
							"•`Delays [old game phase] [new game phase] [delay]`: Change the delay between changing game phase and muting/unmuting players\n"+
							"•`VoiceRules [mute/deaf] [game phase] [alive/dead] [true/false]`: Whether to mute/deafen alive/dead players during that game phase")
						return
					}
					// if command invalid, no need to reapply changes to json file
					isValid := false
					switch args[1] {
					case "commandprefix":
						if len(args) == 2 {
							s.ChannelMessageSend(m.ChannelID, "`CommandPrefix [prefix]`: Change the bot's prefix in this server.")
							return
						}
						if len(args[2]) > 10 {
							// prevent someone from setting something ridiculous lol
							s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Sorry, the prefix `%s` is too long (%d characters, max 10). Try something shorter.", args[2], len(args[2])))
						} else {
							s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Guild prefix changed from `%s` to `%s`. Use that from now on!",
								guild.PersistentGuildData.CommandPrefix, args[2]))
							guild.PersistentGuildData.CommandPrefix = args[2]
							isValid = true
						}
					case "defaulttrackedchannel":
						if len(args) == 2 {
							// give them both command syntax and current voice channel
							channelList, _ := s.GuildChannels(m.GuildID)
							for _, c := range channelList {
								if c.ID == guild.PersistentGuildData.DefaultTrackedChannel {
									s.ChannelMessageSend(m.ChannelID, "`DefaultTrackedChannel [voiceChannel]`: Change the voice channel the bot tracks by default.\n"+
										fmt.Sprintf("Currently, I'm tracking the `%s` voice channel", c.Name))
									return
								}
							}
							s.ChannelMessageSend(m.ChannelID, "`DefaultTrackedChannel [voiceChannel]`: Change the voice channel the bot tracks by default.\n"+
								"Currently, I'm not tracking any voice channel. Either the ID is invalid or you didn't give me one.")
							return
						}
						// now to find the channel they are referencing
						channelID := ""
						channelName := "" // we track name to confirm to the user they selected the right channel
						channelList, _ := s.GuildChannels(m.GuildID)
						for _, c := range channelList {
							// Check if channel is a voice channel
							if c.Type != discordgo.ChannelTypeGuildVoice {
								continue
							}
							// check if this is the right channel
							if strings.ToLower(c.Name) == args[2] || c.ID == args[2] {
								channelID = c.ID
								channelName = c.Name
								break
							}
						}
						// check if channel was found
						if channelID != "" {
							s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Default voice channel changed to `%s`. Use that from now on!",
								channelName))
							guild.PersistentGuildData.DefaultTrackedChannel = channelID
							isValid = true
						} else {
							s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Could not find the voice channel `%s`! Pass in the name or the ID, and make sure the bot can see it.", args[2]))
						}
					case "adminuserids":
						if len(args) == 2 {
							adminCount := len(guild.PersistentGuildData.AdminUserIDs) // caching for optimisation
							// make a nicely formatted string of all the admins: "user1, user2, user3 and user4"
							if adminCount == 0 {
								s.ChannelMessageSend(m.ChannelID, "`AdminUserIDs [user 1] [user 2] [etc]`: Add or remove bot admins, a.k.a users that can use commands with the bot.\n"+
									"Currently, there are no bot admins.")
							} else if adminCount == 1 {
								members, _ := s.GuildMembers(m.GuildID, "", 500)
								for _, member := range members {
									if guild.HasAdminPermissions(member.User.ID) {
										// mention user without pinging
										s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
											Content: "`AdminUserIDs [user 1] [user 2] [etc]`: Add or remove bot admins, a.k.a users that can use commands with the bot.\n" +
												fmt.Sprintf("Currently, the only admin is %s.", member.Mention()),
											AllowedMentions: &discordgo.MessageAllowedMentions{Users: nil},
										})
									}
								}
							} else {
								adminsAccountedFor := 0 // to help formatting
								listOfAdmins := ""
								members, _ := s.GuildMembers(m.GuildID, "", 500)
								for _, member := range members {
									if guild.HasAdminPermissions(member.User.ID) {
										adminsAccountedFor++
										if adminsAccountedFor == adminCount {
											listOfAdmins += " and " + member.Mention()
										} else if adminsAccountedFor == 1 {
											listOfAdmins += member.Mention()
										} else {
											listOfAdmins += ", " + member.Mention()
										}
									}
								}
								// mention users without pinging
								s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
									Content: "`AdminUserIDs [user 1] [user 2] [etc]`: Add or remove bot admins, a.k.a users that can use commands with the bot.\n" +
										fmt.Sprintf("Currently, the admins are %s.", listOfAdmins),
									AllowedMentions: &discordgo.MessageAllowedMentions{Users: nil},
								})
							}
							return
						}
						// users to get admin-ed (adminated?)
						var userIDs []string

						for _, userName := range args[2:len(args)] {
							ID := getMemberFromString(s, m.GuildID, userName)
							if ID == "" {
								s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Sorry, I don't know who `%s` is. You can pass in ID, username, username#XXXX, nickname or @mention", userName))
							} else {
								userIDs = append(userIDs, ID)
							}
						}

						// the index of admins to remove from AdminUserIDs
						var removeAdmins []int

						for _, ID := range userIDs {
							// can't use guild.HasAdminPermissions() because we also need index
							for index, adminID := range guild.PersistentGuildData.AdminUserIDs {
								if ID == adminID {
									// add ID to IDs to be deleted
									removeAdmins = append(removeAdmins, index)
									ID = "" // indicate to other loop this ID has been dealt with
									break
								}
							}
							if ID != "" {
								guild.PersistentGuildData.AdminUserIDs = append(guild.PersistentGuildData.AdminUserIDs, ID)
								// mention user without pinging
								s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
									Content:         fmt.Sprintf("<@%s> is now a bot admin!", ID),
									AllowedMentions: &discordgo.MessageAllowedMentions{Users: nil},
								})
								isValid = true
							}
						}

						for _, indexToRemove := range removeAdmins {
							s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
								Content:         fmt.Sprintf("<@%s> is no longer a bot admin, RIP", guild.PersistentGuildData.AdminUserIDs[indexToRemove]),
								AllowedMentions: &discordgo.MessageAllowedMentions{Users: nil},
							})
							guild.PersistentGuildData.AdminUserIDs = append(guild.PersistentGuildData.AdminUserIDs[:indexToRemove],
								guild.PersistentGuildData.AdminUserIDs[indexToRemove+1:]...)
							isValid = true
						}
					case "applynicknames":
						if len(args) == 2 {
							if guild.PersistentGuildData.ApplyNicknames {
								s.ChannelMessageSend(m.ChannelID, "`ApplyNicknames [true/false]`: Whether the bot should change the nicknames of the players to reflect the player's color.\n"+
									"Currently the bot does change nicknames.")
							} else {
								s.ChannelMessageSend(m.ChannelID, "`ApplyNicknames [true/false]`: Whether the bot should change the nicknames of the players to reflect the player's color.\n"+
									"Currently the bot does **not** change nicknames.")
							}
							return
						}
						if args[2] == "true" {
							if guild.PersistentGuildData.ApplyNicknames {
								s.ChannelMessageSend(m.ChannelID, "It's already true!")
							} else {
								s.ChannelMessageSend(m.ChannelID, "I will now rename the players in the voice chat.")
								guild.PersistentGuildData.ApplyNicknames = true
							}
						} else if args[2] == "false" {
							if guild.PersistentGuildData.ApplyNicknames {
								s.ChannelMessageSend(m.ChannelID, "I will no longer  rename the players in the voice chat.")
								guild.PersistentGuildData.ApplyNicknames = false
							} else {
								s.ChannelMessageSend(m.ChannelID, "It's already false!")
							}
						} else {
							s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Sorry, `%s` is neither `true` nor `false`.", args[2]))
						}
						isValid = true
					case "unmutedeadduringtasks":
						if len(args) == 2 {
							if guild.PersistentGuildData.UnmuteDeadDuringTasks {
								s.ChannelMessageSend(m.ChannelID, "`UnmuteDeadDuringTasks [true/false]`: Whether the bot should unmute dead players immediately when they die. "+
									"**WARNING**: reveals who died before discussion begins! Use at your own risk.\n"+
									"Currently the bot does unmute the players immediately after dying.")
							} else {
								s.ChannelMessageSend(m.ChannelID, "`UnmuteDeadDuringTasks [true/false]`: Whether the bot should unmute dead players immediately when they die. "+
									"**WARNING**: reveals who died before discussion begins! Use at your own risk.\n"+
									"Currently the bot does **not** unmute the players immediately after dying.")
							}
							return
						}
						if args[2] == "true" {
							if guild.PersistentGuildData.UnmuteDeadDuringTasks {
								s.ChannelMessageSend(m.ChannelID, "It's already true!")
							} else {
								s.ChannelMessageSend(m.ChannelID, "I will now unmute the dead people immediately after they die. Careful, this reveals who died during the match!")
								guild.PersistentGuildData.UnmuteDeadDuringTasks = true
							}
						} else if args[2] == "false" {
							if guild.PersistentGuildData.UnmuteDeadDuringTasks {
								s.ChannelMessageSend(m.ChannelID, "I will no longer immediately unmute dead people. Good choice!")
								guild.PersistentGuildData.UnmuteDeadDuringTasks = false
							} else {
								s.ChannelMessageSend(m.ChannelID, "It's already false!")
							}
						} else {
							s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Sorry, `%s` is neither `true` nor `false`.", args[2]))
						}
						isValid = true
					case "delays":
						if len(args) == 2 {
							s.ChannelMessageSend(m.ChannelID, "`Delays [old game phase] [new game phase] [delay]`: Change the delay between changing game phase and muting/unmuting players.")
							return
						}
						// user passes phase name, phase name and new delay value
						if len(args) < 4 {
							// user didn't pass 2 phases, tell them the list of game phases
							s.ChannelMessageSend(m.ChannelID, "The list of game phases are `Lobby`, `Tasks` and `Discussion`.\n"+
								"You need to type both phases the game is transitioning from and to to change the delay.") // find a better wording for this at some point
							break
						}
						// now to find the actual game state from the string they passed
						var gamePhase1 = getPhaseFromString(args[2])
						var gamePhase2 = getPhaseFromString(args[3])
						if gamePhase1 == game.UNINITIALIZED {
							s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("I don't know what `%s` is. The list of game phases are `Lobby`, `Tasks` and `Discussion`.", args[2]))
							break
						} else if gamePhase2 == game.UNINITIALIZED {
							s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("I don't know what `%s` is. The list of game phases are `Lobby`, `Tasks` and `Discussion`.", args[3]))
							break
						}
						oldDelay := guild.PersistentGuildData.Delays.GetDelay(gamePhase1, gamePhase2)
						if len(args) == 4 {
							// no number was passed, user was querying the delay
							s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Currently, the delay when passing from `%s` to `%s` is %d.", args[2], args[3], oldDelay))
							break
						}
						newDelay, err := strconv.Atoi(args[4])
						if err != nil || newDelay < 0 {
							s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("`%d` is not a valid number! Please try again", args[4]))
							break
						}
						guild.PersistentGuildData.Delays.Delays[game.PhaseNames[gamePhase1]][game.PhaseNames[gamePhase2]] = newDelay
						s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("The delay when passing from `%s` to `%s` changed from %d to %d.", args[2], args[3], oldDelay, newDelay))
						isValid = true
					case "voicerules":
						if len(args) == 2 {
							s.ChannelMessageSend(m.ChannelID, "`VoiceRules [mute/deaf] [game phase] [alive/dead] [true/false]`: Whether to mute/deafen alive/dead players during that game phase.")
							return
						}
						// now for a bunch of input checking
						if len(args) < 5 {
							// user didn't pass enough args
							s.ChannelMessageSend(m.ChannelID, "You didn't pass enough arguments! Correct syntax is: `VoiceRules [mute/deaf] [game phase] [alive/dead] [true/false]`")
							return
						}
						if args[2] == "deaf" {
							args[2] = "deafened" // for formatting later on
						} else if args[2] == "mute" {
							args[2] = "muted" // same here
						} else {
							s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("`%s` is neither `mute` nor `deaf`!", args[2]))
							return
						}
						gamePhase := getPhaseFromString(args[3])
						if gamePhase == game.UNINITIALIZED {
							s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("I don't know what %s is. The list of game phases are `Lobby`, `Tasks` and `Discussion`.", args[3]))
						}
						if args[4] != "alive" && args[4] != "dead" {
							s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("`%s` is neither `alive` or `dead`!", args[4]))
							return
						}
						var oldValue bool
						if args[2] == "muted" {
							oldValue = guild.PersistentGuildData.VoiceRules.MuteRules[game.PhaseNames[gamePhase]][args[4]]
						} else {
							oldValue = guild.PersistentGuildData.VoiceRules.DeafRules[game.PhaseNames[gamePhase]][args[4]]
						}
						if len(args) == 5 {
							// user was only querying
							if oldValue {
								s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("When in `%s` phase, %s players are currently %s.", args[3], args[4], args[2]))
							} else {
								s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("When in `%s` phase, %s players are currently NOT %s.", args[3], args[4], args[2]))
							}
							break
						}
						var newValue bool
						if args[5] == "true" {
							newValue = true
						} else if args[5] == "false" {
							newValue = false
						} else {
							s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("`%s` is neither `true` or `false`!", args[5]))
							return
						}
						if newValue == oldValue {
							if newValue {
								s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("When in `%s` phase, %s players are already %s!", args[3], args[4], args[2]))
							} else {
								s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("When in `%s` phase, %s players are already un%s!", args[3], args[4], args[2]))
							}
							return
						}
						if args[2] == "muted" {
							guild.PersistentGuildData.VoiceRules.MuteRules[game.PhaseNames[gamePhase]][args[4]] = newValue
						} else {
							guild.PersistentGuildData.VoiceRules.DeafRules[game.PhaseNames[gamePhase]][args[4]] = newValue
						}
						if newValue {
							s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("From now on, when in `%s` phase, %s players will be %s.", args[3], args[4], args[2]))
						} else {
							s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("From now on, when in `%s` phase, %s players will be un%s.", args[3], args[4], args[2]))
						}
						isValid = true
					case "permissiondroleids":
						// this setting is not actually used anywhere
						s.ChannelMessageSend(m.ChannelID, "Sorry, not supported yet! You need to edit the JSON file and restart the bot.")
					default:
						s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Sorry, `%s` is not a valid setting!\n"+
							"Valid settings include `CommandPrefix`, `DefaultTrackedChannel`, `AdminUserIDs`, `ApplyNicknames`, `UnmuteDeadDuringTasks`, `Delays` and `VoiceRules`.", args[1]))
					}
					if isValid {
						// TODO apply changes to JSON file
						// currently changes are lost once bot is restarted
					}
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

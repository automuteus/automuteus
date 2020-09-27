package discord

import (
	"crypto/sha256"
	"encoding/hex"
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
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	socketio "github.com/googollee/go-socket.io"
)

// AllConns mapping of socket IDs to guild IDs
var AllConns = map[string]string{}

// AllGuilds mapping of guild IDs to GuildState references
var AllGuilds = map[string]*GuildState{}

// LinkCodes maps the code to the guildID
var LinkCodes = map[string]string{}

// LinkCodeLock mutex for above
var LinkCodeLock = sync.RWMutex{}

// GamePhaseUpdateChannel this should not be global
var GamePhaseUpdateChannel chan game.PhaseUpdate

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

	GamePhaseUpdateChannel = make(chan game.PhaseUpdate)

	playerUpdateChannel := make(chan game.PlayerUpdate)

	go socketioServer(GamePhaseUpdateChannel, playerUpdateChannel, port)

	go discordListener(dg, GamePhaseUpdateChannel, playerUpdateChannel)

	<-sc

	dg.Close()
}

func socketioServer(gamePhaseUpdateChannel chan<- game.PhaseUpdate, playerUpdateChannel chan<- game.PlayerUpdate, port string) {
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
				AllConns[s.ID()] = gid
				guild.LinkCode = ""
			}
		}

		log.Printf("Associated websocket id %s with guildID %s using code %s\n", s.ID(), guildID, msg)
		s.Emit("reply", "set guildID successfully")
	})
	server.OnEvent("/", "state", func(s socketio.Conn, msg string) {
		log.Println("phase received from capture: ", msg)
		phase, err := strconv.Atoi(msg)
		if err != nil {
			log.Println(err)
		} else {
			if v, ok := AllConns[s.ID()]; ok {
				log.Println("Pushing phase event to channel")
				gamePhaseUpdateChannel <- game.PhaseUpdate{
					Phase:   game.Phase(phase),
					GuildID: v,
				}
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
			if v, ok := AllConns[s.ID()]; ok {
				playerUpdateChannel <- game.PlayerUpdate{
					Player:  player,
					GuildID: v,
				}
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
		AllConns[s.ID()] = "" //deassociate the link between guild and WS
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

				//TODO when the client disconnects, we don't update the status message in discord

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

func discordListener(dg *discordgo.Session, phaseUpdateChannel <-chan game.PhaseUpdate, playerUpdateChannel <-chan game.PlayerUpdate) {
	for {
		select {
		case phaseUpdate := <-phaseUpdateChannel:
			log.Printf("Received PhaseUpdate message for guild %s\n", phaseUpdate.GuildID)
			if guild, ok := AllGuilds[phaseUpdate.GuildID]; ok {
				switch phaseUpdate.Phase {
				case game.MENU:
					log.Println("Detected transition to Menu; not doing anything about it yet")
				case game.LOBBY:
					if guild.AmongUsData.GetPhase() == game.LOBBY {
						break
					}
					log.Println("Detected transition to Lobby")

					delay := guild.PersistentGuildData.Delays.GetDelay(guild.AmongUsData.GetPhase(), game.LOBBY)
					if delay > 0 {
						log.Printf("Sleeping for %d secs before changes\n", delay)
					}

					guild.AmongUsData.SetAllAlive()
					guild.AmongUsData.SetPhase(phaseUpdate.Phase)

					guild.handleTrackedMembers(dg, delay)

					//add back the emojis AFTER we do any mute/unmutes
					//for _, e := range guild.StatusEmojis[true] {
					//	if guild.GameStateMessage != nil {
					//		addReaction(dg, guild.GameStateMessage.ChannelID, guild.GameStateMessage.ID, e.FormatForReaction())
					//	}
					//}

					guild.GameStateMsg.Edit(dg, gameStateResponse(guild))
				case game.TASKS:
					if guild.AmongUsData.GetPhase() == game.TASKS {
						break
					}
					log.Println("Detected transition to Tasks")
					oldPhase := guild.AmongUsData.GetPhase()
					delay := guild.PersistentGuildData.Delays.GetDelay(oldPhase, game.TASKS)
					if delay > 0 {
						log.Printf("Sleeping for %d secs before changes\n", delay)
					}

					if oldPhase == game.LOBBY {
						//when we go from lobby to tasks, mark all users as alive to be sure
						guild.AmongUsData.SetAllAlive()
						//if we went from lobby to tasks, remove all the emojis from the game start message
						//guild.handleReactionsGameStartRemoveAll(dg)
					}

					guild.AmongUsData.SetPhase(phaseUpdate.Phase)

					guild.handleTrackedMembers(dg, delay)

					guild.GameStateMsg.Edit(dg, gameStateResponse(guild))
				case game.DISCUSS:
					if guild.AmongUsData.GetPhase() == game.DISCUSS {
						break
					}
					log.Println("Detected transition to Discussion")

					delay := guild.PersistentGuildData.Delays.GetDelay(guild.AmongUsData.GetPhase(), game.DISCUSS)
					if delay > 0 {
						log.Printf("Sleeping for %d secs before changes\n", delay)
					}

					guild.AmongUsData.SetPhase(phaseUpdate.Phase)

					guild.handleTrackedMembers(dg, delay)

					guild.GameStateMsg.Edit(dg, gameStateResponse(guild))
				default:
					log.Printf("Undetected new state: %d\n", phaseUpdate.Phase)
				}
			}

			// TODO prevent cases where 2 players are mapped to the same underlying in-game player data
		case playerUpdate := <-playerUpdateChannel:
			log.Printf("Received PlayerUpdate message for guild %s\n", playerUpdate.GuildID)
			if guild, ok := AllGuilds[playerUpdate.GuildID]; ok {

				//	this updates the copies in memory
				//	(player's associations to amongus data are just pointers to these structs)
				if playerUpdate.Player.Name != "" {
					if playerUpdate.Player.Action == game.EXILED {
						log.Println("Detected player EXILE event, marking as dead")
						playerUpdate.Player.IsDead = true
					}
					if playerUpdate.Player.IsDead == true && guild.AmongUsData.GetPhase() == game.LOBBY {
						log.Println("Received a dead event, but we're in the Lobby, so I'm ignoring it")
						playerUpdate.Player.IsDead = false
					}

					if playerUpdate.Player.Disconnected {
						log.Println("I detected that " + playerUpdate.Player.Name + " disconnected! " +
							"I'm removing their linked game data; they will need to relink")

						guild.UserData.ClearPlayerDataByPlayerName(playerUpdate.Player.Name)
						guild.GameStateMsg.Edit(dg, gameStateResponse(guild))
					} else {
						updated, isAliveUpdated := guild.AmongUsData.ApplyPlayerUpdate(playerUpdate.Player)

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

				initialTracking := TrackingChannel{}

				//TODO need to send a message to the capture re-questing all the player/game states. Otherwise,
				//we don't have enough info to go off of when remaking the game...
				//if !guild.GameStateMsg.Exists() {
				connectCode := generateConnectCode(guild.PersistentGuildData.GuildID)
				log.Println(connectCode)
				LinkCodeLock.Lock()
				LinkCodes[connectCode] = guild.PersistentGuildData.GuildID
				guild.LinkCode = connectCode
				LinkCodeLock.Unlock()

				for _, v := range g.VoiceStates {
					//if the user is detected in a voice channel
					if v.UserID == m.Author.ID {
						for _, channel := range g.Channels {
							//once we find the channel by ID
							if channel.ID == v.ChannelID {
								initialTracking = TrackingChannel{
									channelID:   channel.ID,
									channelName: channel.Name,
									forGhosts:   false,
								}
								log.Printf("User that typed new is in the \"%s\" voice channel; using that for tracking", channel.Name)
							}
						}
					}
				}
				//}

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
					GamePhaseUpdateChannel <- game.PhaseUpdate{
						Phase:   phase,
						GuildID: m.GuildID,
					}
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

func getPhaseFromArgs(args []string) game.Phase {
	if len(args) == 0 {
		return game.UNINITIALIZED
	}

	phase := strings.ToLower(args[0])
	switch phase {
	case "lobby":
		fallthrough
	case "l":
		return game.LOBBY
	case "task":
		fallthrough
	case "t":
		fallthrough
	case "tasks":
		fallthrough
	case "game":
		fallthrough
	case "g":
		return game.TASKS
	case "discuss":
		fallthrough
	case "disc":
		fallthrough
	case "d":
		fallthrough
	case "discussion":
		return game.DISCUSS
	default:
		return game.UNINITIALIZED

	}
}

// GetRoomAndRegionFromArgs does what it sounds like
func getRoomAndRegionFromArgs(args []string) (string, string) {
	if len(args) == 0 {
		return "Unprovided", "Unprovided"
	}
	room := strings.ToUpper(args[0])
	if len(args) == 1 {
		return room, "Unprovided"
	}
	region := strings.ToLower(args[1])
	switch region {
	case "na":
		fallthrough
	case "us":
		fallthrough
	case "usa":
		fallthrough
	case "north":
		region = "North America"
	case "eu":
		fallthrough
	case "europe":
		region = "Europe"
	case "as":
		fallthrough
	case "asia":
		region = "Asia"
	}
	return room, region
}

func generateConnectCode(guildID string) string {
	h := sha256.New()
	h.Write([]byte(guildID))
	//add some "randomness" with the current time
	h.Write([]byte(time.Now().String()))
	hashed := strings.ToUpper(hex.EncodeToString(h.Sum(nil))[0:6])
	//TODO replace common problematic characters?
	return strings.ReplaceAll(strings.ReplaceAll(hashed, "I", "1"), "O", "0")
}

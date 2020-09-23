package discord

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
func MakeAndStartBot(token string, moveDeadPlayers bool, port string) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Println("error creating Discord session,", err)
		return
	}

	dg.AddHandler(voiceStateChange)
	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(messageCreate)
	dg.AddHandler(reactionCreate)
	dg.AddHandler(newGuild(moveDeadPlayers))

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
		AllConns[s.ID()] = "" //deassociate the link

		for gid, guild := range AllGuilds {
			if gid == previousGid {
				//guild.UserDataLock.Lock()
				guild.LinkCode = generateConnectCode(gid) //this is unlinked
				//guild.UserDataLock.Unlock()

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
					if guild.GamePhase == game.LOBBY {
						break
					}
					log.Println("Detected transition to Lobby")

					guild.modifyCachedAmongUsDataAlive(true)
					guild.GamePhase = phaseUpdate.Phase

					guild.handleTrackedMembers(dg)

					//add back the emojis AFTER we do any mute/unmutes
					//for _, e := range guild.StatusEmojis[true] {
					//	if guild.GameStateMessage != nil {
					//		addReaction(dg, guild.GameStateMessage.ChannelID, guild.GameStateMessage.ID, e.FormatForReaction())
					//	}
					//}

					guild.handleGameStateMessage(dg)
				case game.TASKS:
					if guild.GamePhase == game.TASKS {
						break
					}
					log.Println("Detected transition to Tasks")

					if guild.GamePhase == game.LOBBY {
						//if we went from lobby to tasks, remove all the emojis from the game start message
						//guild.handleReactionsGameStartRemoveAll(dg)
					} else if guild.GamePhase == game.DISCUSS {

					}

					guild.GamePhase = phaseUpdate.Phase

					guild.handleTrackedMembers(dg)

					guild.handleGameStateMessage(dg)
				case game.DISCUSS:
					if guild.GamePhase == game.DISCUSS {
						break
					}
					log.Println("Detected transition to Discussion")
					guild.GamePhase = phaseUpdate.Phase

					guild.handleTrackedMembers(dg)

					guild.handleGameStateMessage(dg)
				default:
					log.Printf("Undetected new state: %d\n", phaseUpdate.Phase)
				}
			}

			//TODO prevent cases where 2 players are mapped to the same underlying in-game player data
		case playerUpdate := <-playerUpdateChannel:
			log.Printf("Received PlayerUpdate message for guild %s\n", playerUpdate.GuildID)
			if guild, ok := AllGuilds[playerUpdate.GuildID]; ok {

				//this updates the copies in memory
				//(player's associations to amongus data are just pointers to these structs)
				if playerUpdate.Player.Name != "" {
					updated, isAliveUpdated := guild.updateCachedAmongUsData(playerUpdate.Player)

					if updated {
						//log.Println("Player update received caused an update in cached state")
						if isAliveUpdated && guild.GamePhase == game.TASKS {
							log.Println("NOT updating the discord status message; would leak info")
						} else {
							guild.handleGameStateMessage(dg)
						}
					} else {
						//log.Println("Player update received did not cause an update in cached state")
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

func newGuild(moveDeadPlayers bool) func(s *discordgo.Session, m *discordgo.GuildCreate) {
	return func(s *discordgo.Session, m *discordgo.GuildCreate) {
		log.Printf("Added to new Guild, id %s, name %s", m.Guild.ID, m.Guild.Name)
		AllGuilds[m.ID] = &GuildState{
			ID:            m.ID,
			CommandPrefix: ".au",
			LinkCode:      m.Guild.ID,

			UserData:         make(map[string]UserData),
			Tracking:         make(map[string]Tracking),
			GameStateMessage: nil,
			Delays:           GameDelays{},
			StatusEmojis:     emptyStatusEmojis(),
			SpecialEmojis:    map[string]Emoji{},
			UserDataLock:     sync.RWMutex{},

			AmongUsData:     map[string]*AmongUserData{},
			GamePhase:       game.LOBBY,
			Room:            "",
			Region:          "",
			AmongUsDataLock: sync.RWMutex{},

			MoveDeadPlayers: moveDeadPlayers,
		}
		mems, err := s.GuildMembers(m.Guild.ID, "", 1000)
		if err != nil {
			log.Println(err)
		}
		//AllGuilds[m.ID].UserDataLock.Lock()
		for _, v := range mems {
			AllGuilds[m.ID].UserData[v.User.ID] = UserData{
				user: User{
					nick:          v.Nick,
					userID:        v.User.ID,
					userName:      v.User.Username,
					discriminator: v.User.Discriminator,
				},
				auData: nil,
			}
		}
		//AllGuilds[m.ID].UserDataLock.Unlock()
		//AllGuilds[m.ID].updateVoiceStatusCache(s)
		log.Println("Updated members for guild " + m.Guild.ID)

		allEmojis, err := s.GuildEmojis(m.Guild.ID)
		if err != nil {
			log.Println(err)
		} else {
			AllGuilds[m.ID].addAllMissingEmojis(s, m.Guild.ID, true, allEmojis)

			AllGuilds[m.ID].addAllMissingEmojis(s, m.Guild.ID, false, allEmojis)

			AllGuilds[m.ID].addSpecialEmojis(s, m.Guild.ID, allEmojis)
		}
	}
}

func (guild *GuildState) handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	contents := m.Content
	if strings.HasPrefix(contents, guild.CommandPrefix) {
		args := strings.Split(contents, " ")[1:]
		for i, v := range args {
			args[i] = strings.ToLower(v)
		}
		if len(args) == 0 {
			s.ChannelMessageSend(m.ChannelID, helpResponse(guild.CommandPrefix))
		} else {
			switch args[0] {
			case "help":
				fallthrough
			case "h":
				s.ChannelMessageSend(m.ChannelID, helpResponse(guild.CommandPrefix))
				break
			case "track":
				fallthrough
			case "t":
				if len(args[1:]) == 0 {
					//TODO print usage of this command specifically
					s.ChannelMessageSend(m.ChannelID, "You used this command incorrectly! Please refer to `.au help` for proper command usage")
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

					//guild.UserDataLock.Lock()
					guild.trackChannelResponse(channelName, channels, forGhosts)
					//guild.UserDataLock.Unlock()

					guild.handleGameStateMessage(s)
				}
				break

			case "link":
				fallthrough
			case "l":
				if len(args[1:]) < 2 {
					//TODO print usage of this command specifically
					s.ChannelMessageSend(m.ChannelID, "You used this command incorrectly! Please refer to `.au help` for proper command usage")
				} else {
					//guild.UserDataLock.Lock()
					guild.linkPlayerResponse(args[1:], guild.AmongUsData)

					//guild.UserDataLock.Unlock()

					guild.handleGameStateMessage(s)
				}
				break
			case "unlink":
				fallthrough
			case "ul":
				fallthrough
			case "u":
				if len(args[1:]) == 0 {
					s.ChannelMessageSend(m.ChannelID, "You used this command incorrectly! Please refer to `.au help` for proper command usage")
				} else {

				}
				userID, err := extractUserIDFromMention(args[1])
				if err != nil {
					log.Println(err)
				} else {

					guild.handlePlayerRemove(s, userID)

					//make sure that any players we remove/unlink get auto-unmuted/undeafened
					guild.verifyVoiceStateChanges(s)

					//update the state message to reflect the player leaving
					guild.handleGameStateMessage(s)
				}
			case "start":
				fallthrough
			case "s":
				fallthrough
			case "new":
				fallthrough
			case "newgame":
				fallthrough
			case "n":
				room, region := getRoomAndRegionFromArgs(args[1:])

				connectCode := generateConnectCode(guild.ID)
				log.Println(connectCode)
				LinkCodeLock.Lock()
				LinkCodes[connectCode] = guild.ID
				guild.LinkCode = connectCode
				LinkCodeLock.Unlock()

				guild.handleGameStartMessage(s, m, room, region)
				break
			case "end":
				fallthrough
			case "e":
				fallthrough
			case "endgame":
				//delete the player's message as well
				if guild.GameStateMessage.ChannelID == m.ChannelID {
					deleteMessage(s, m.ChannelID, m.Message.ID)
				}

				guild.handleGameEndMessage(s, m)

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
			default:
				s.ChannelMessageSend(m.ChannelID, "Sorry, I didn't understand that command! Please see `.au help` for commands")

			}
		}
		//Just deletes messages starting with .au
		if guild.GameStateMessage != nil {
			if guild.GameStateMessage.ChannelID == m.ChannelID {
				deleteMessage(s, m.ChannelID, m.Message.ID)
			}
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

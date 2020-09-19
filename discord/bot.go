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
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	socketio "github.com/googollee/go-socket.io"
)

// AmongUsDefaultName const
const AmongUsDefaultName = "Player"

// AmongUsDefaultColor const
const AmongUsDefaultColor = "Cyan"

// CommandPrefix const
const CommandPrefix = ".au"

// GameDelays struct
type GameDelays struct {
	GameStartDelay           int
	GameResumeDelay          int
	DiscussStartDelay        int
	DiscordMuteDelayOffsetMs int
}

// AllConns mapping of socket IDs to guild IDs
var AllConns map[string]string

// AllGuilds var
var AllGuilds map[string]*GuildState

// MakeAndStartBot does what it sounds like
func MakeAndStartBot(token string, moveDeadPlayers bool) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	dg.AddHandler(voiceStateChange)
	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(messageCreate)
	dg.AddHandler(newGuild(moveDeadPlayers))

	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildVoiceStates | discordgo.IntentsGuildMessages | discordgo.IntentsGuilds)

	//Open a websocket connection to Discord and begin listening.
	err = dg.Open()

	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)

	AllGuilds = make(map[string]*GuildState)
	AllConns = make(map[string]string)

	gamePhaseUpdateChannel := make(chan game.GamePhaseUpdate)

	playerUpdateChannel := make(chan game.PlayerUpdate)

	go socketioServer(gamePhaseUpdateChannel, playerUpdateChannel)

	go discordListener(dg, gamePhaseUpdateChannel, playerUpdateChannel)

	<-sc

	dg.Close()
}

func socketioServer(gamePhaseUpdateChannel chan<- game.GamePhaseUpdate, playerUpdateChannel chan<- game.PlayerUpdate) {
	server, err := socketio.NewServer(nil)
	if err != nil {
		log.Fatal(err)
	}
	server.OnConnect("/", func(s socketio.Conn) error {
		s.SetContext("")
		fmt.Println("connected:", s.ID())
		return nil
	})
	server.OnEvent("/", "guildID", func(s socketio.Conn, msg string) {
		fmt.Println("set guildID:", msg)
		for gid := range AllGuilds {
			if gid == msg {
				AllConns[s.ID()] = gid //associate the socket with the guild
				log.Printf("Associated websocket id %s with guildID %s\n", s.ID(), gid)
				s.Emit("reply", "set guildID successfully")
			}
		}
	})
	server.OnEvent("/", "state", func(s socketio.Conn, msg string) {
		fmt.Println("phase: ", msg)
		phase, err := strconv.Atoi(msg)
		if err != nil {
			log.Println(err)
		} else {
			if v, ok := AllConns[s.ID()]; ok {
				gamePhaseUpdateChannel <- game.GamePhaseUpdate{
					Phase:   game.GamePhase(phase),
					GuildID: v,
				}
			} else {
				log.Println("This websocket is not associated with any guilds")
			}
		}

	})
	server.OnEvent("/", "player", func(s socketio.Conn, msg string) {
		fmt.Println("player: ", msg)
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
		fmt.Println("meet error:", e)
	})
	server.OnDisconnect("/", func(s socketio.Conn, reason string) {
		fmt.Println("closed", reason)
	})
	go server.Serve()
	defer server.Close()

	http.Handle("/socket.io/", server)
	log.Println("Serving at localhost:8123...")
	log.Fatal(http.ListenAndServe(":8123", nil))
}

func discordListener(dg *discordgo.Session, phaseUpdateChannel <-chan game.GamePhaseUpdate, playerUpdateChannel <-chan game.PlayerUpdate) {
	for {
		select {
		case phaseUpdate := <-phaseUpdateChannel:
			log.Printf("Received PhaseUpdate message for guild %s\n", phaseUpdate.GuildID)
			if guild, ok := AllGuilds[phaseUpdate.GuildID]; ok {
				handleGameStateMessage(guild, dg)
				switch phaseUpdate.Phase {
				case game.LOBBY:
					log.Println("Detected transition to lobby")

					guild.AmongUsDataLock.Lock()
					guild.modifyCachedAmongUsDataAlive(true)
					guild.AmongUsDataLock.Unlock()

					guild.handleTrackedMembers(dg, false, false)
					guild.GamePhaseLock.Lock()
					guild.GamePhase = phaseUpdate.Phase
					guild.GamePhaseLock.Unlock()
				case game.TASKS:
					log.Println("Detected transition to tasks")
					delay := 0
					guild.GamePhaseLock.RLock()
					if guild.GamePhase == game.LOBBY {
						delay = guild.delays.GameStartDelay
					} else if guild.GamePhase == game.DISCUSS {
						delay = guild.delays.GameResumeDelay
					}
					guild.GamePhaseLock.RUnlock()

					time.Sleep(time.Second * time.Duration(delay))
					guild.handleTrackedMembers(dg, true, false)

					guild.GamePhaseLock.Lock()
					guild.GamePhase = phaseUpdate.Phase
					guild.GamePhaseLock.Unlock()
				case game.DISCUSS:
					log.Println("Detected transition to discussion")
					time.Sleep(time.Second * time.Duration(guild.delays.DiscussStartDelay))
					guild.GamePhaseLock.Lock()
					guild.GamePhase = phaseUpdate.Phase
					guild.GamePhaseLock.Unlock()
					guild.handleTrackedMembers(dg, false, true)
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
				updated := guild.updateCachedAmongUsData(playerUpdate.Player)

				if updated {
					log.Println("Player update received caused an update in cached state")
				} else {
					log.Println("Player update received did not cause an update in cached state")
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

func newGuild(moveDeadPlayers bool) func(s *discordgo.Session, m *discordgo.GuildCreate) {
	return func(s *discordgo.Session, m *discordgo.GuildCreate) {
		log.Printf("Added to new Guild, id %s, name %s", m.Guild.ID, m.Guild.Name)
		AllGuilds[m.ID] = &GuildState{
			ID:              m.ID,
			delays:          GameDelays{},
			UserData:        make(map[string]UserData),
			UserDataLock:    sync.RWMutex{},
			GamePhase:       game.LOBBY,
			GamePhaseLock:   sync.RWMutex{},
			AmongUsData:     map[string]*AmongUserData{},
			AmongUsDataLock: sync.RWMutex{},
			Tracking:        make(map[string]Tracking),
			TrackingLock:    sync.RWMutex{},
			TextChannelID:   "",
			MoveDeadPlayers: moveDeadPlayers,
		}
		mems, err := s.GuildMembers(m.Guild.ID, "", 1000)
		if err != nil {
			log.Println(err)
		}
		AllGuilds[m.ID].UserDataLock.Lock()
		for _, v := range mems {
			AllGuilds[m.ID].UserData[v.User.ID] = UserData{
				user: User{
					nick:          v.Nick,
					userID:        v.User.ID,
					userName:      v.User.Username,
					discriminator: v.User.Discriminator,
				},
				voiceState: discordgo.VoiceState{},
				tracking:   false,
				auData:     nil,
			}
		}
		AllGuilds[m.ID].UserDataLock.Unlock()
		AllGuilds[m.ID].updateVoiceStatusCache(s)
		log.Println("Updated members for guild " + m.Guild.ID)
	}
}

func (guild *GuildState) handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	guild.updateVoiceStatusCache(s)

	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}

	//TODO This should check VOICE channels, not TEXT channels
	contents := m.Content
	if strings.HasPrefix(contents, CommandPrefix) {
		args := strings.Split(contents, " ")[1:]
		for i, v := range args {
			args[i] = strings.ToLower(v)
		}
		if len(args) == 0 {
			s.ChannelMessageSend(m.ChannelID, helpResponse())
		} else {
			switch args[0] {
			case "help":
				fallthrough
			case "h":
				s.ChannelMessageSend(m.ChannelID, helpResponse())
				break
			case "track":
				fallthrough
			case "t":
				if len(args[1:]) == 0 {
					//TODO print usage of this command specifically
					s.ChannelMessageSend(m.ChannelID, "You used this command incorrectly! Please refer to `.au help` for proper command usage")
				} else {
					// if anything is given in the second slot then we consider that a true
					forGhosts := len(args[2:]) >= 1
					channelName := strings.Join(args[1:2], " ")

					channels, err := s.GuildChannels(m.GuildID)
					if err != nil {
						log.Println(err)
					}

					guild.TrackingLock.Lock()
					resp := guild.trackChannelResponse(channelName, channels, forGhosts)
					guild.TrackingLock.Unlock()
					_, err = s.ChannelMessageSend(m.ChannelID, resp)
					if err != nil {
						log.Println(err)
					}
				}
				break
			case "list":
				fallthrough
			case "ls":
				handlePlayerListMessage(guild, s, m)
				break

			case "link":
				fallthrough
			case "l":
				if len(args[1:]) < 2 {
					//TODO print usage of this command specifically
					s.ChannelMessageSend(m.ChannelID, "You used this command incorrectly! Please refer to `.au help` for proper command usage")
				} else {
					guild.AmongUsDataLock.Lock()
					guild.UserDataLock.Lock()
					resp := guild.linkPlayerResponse(args[1:], guild.AmongUsData)
					guild.UserDataLock.Unlock()
					guild.AmongUsDataLock.Unlock()
					_, err := s.ChannelMessageSend(m.ChannelID, resp)
					if err != nil {
						log.Println(err)
					}
				}
				break
			case "broadcast":
				fallthrough
			case "bcast":
				fallthrough
			case "b":
				if len(args[1:]) == 0 {
					//TODO print usage of this command specifically
					s.ChannelMessageSend(m.ChannelID, "You used this command incorrectly! Please refer to `.au help` for proper command usage")
				} else {
					str, err := guild.broadcastResponse(args[1:])
					if err != nil {
						log.Println(err)
					}
					s.ChannelMessageSend(m.ChannelID, str)
				}
				break
			case "start":
				fallthrough
			case "s":
				handleGameStartMessage(guild, s, m)
				break
			default:
				s.ChannelMessageSend(m.ChannelID, "Sorry, I didn't understand that command! Please see `.au help` for commands")
			}
		}
	}
}

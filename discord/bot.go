package discord

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
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

// var VoiceStatusCache = make(map[string]UserData)
// var VoiceStatusCacheLock = sync.RWMutex{}
//
// var GameState = game.GameState{Phase: game.LOBBY}
// var GameStateLock = sync.RWMutex{}
//

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

	//if channel != "" {
	//	dg.ChannelMessageSend(channel, "Bot is Online!")
	//}
	AllGuilds = make(map[string]*GuildState)
	AllConns = make(map[string]string)

	gamePhaseUpdateChannel := make(chan game.GamePhaseUpdate)

	playerUpdateChannel := make(chan game.PlayerUpdate)

	go socketioServer(gamePhaseUpdateChannel, playerUpdateChannel)

	go discordListener(dg, gamePhaseUpdateChannel, playerUpdateChannel)

	<-sc

	//if channel != "" {
	//	dg.ChannelMessageSend(channel, "Bot is going Offline!")
	//}

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
		//s.SetContext(msg)
		//s.Emit("hi", "status "+msg)
	})
	//server.OnEvent("/", "bye", func(s socketio.Conn) string {
	//	last := s.Context().(string)
	//	s.Emit("bye", last)
	//	s.Close()
	//	return last
	//})
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
				//log.Printf("Unmarshalled state object: %s\n", newState.ToString())
				switch phaseUpdate.Phase {
				case game.LOBBY:
					log.Println("Detected transition to lobby")
					//if ExclusiveChannelId != "" {
					//	dg.ChannelMessageSend(ExclusiveChannelId, fmt.Sprintf("Game over! Unmuting players!"))
					//}
					//Loop through and reset players (game over = everyone alive again)

					guild.AmongUsDataLock.Lock()
					for i := range guild.AmongUsData {
						guild.AmongUsData[i].IsAlive = true
					}
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
					//if ExclusiveChannelId != "" {
					//	dg.ChannelMessageSend(ExclusiveChannelId, fmt.Sprintf("Game starting; muting players in %d second(s)!", delay))
					//}
					guild.GamePhaseLock.RUnlock()

					time.Sleep(time.Second * time.Duration(delay))
					guild.handleTrackedMembers(dg, true, false)

					guild.GamePhaseLock.Lock()
					guild.GamePhase = phaseUpdate.Phase
					guild.GamePhaseLock.Unlock()
				case game.DISCUSS:
					log.Println("Detected transition to discussion")
					//if ExclusiveChannelId != "" {
					//	dg.ChannelMessageSend(ExclusiveChannelId, fmt.Sprintf("Starting discussion; unmuting alive players in %d second(s)!", DiscussStartDelay))
					//}
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
			AmongUsData:     MakeAllEmptyAmongUsData(),
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

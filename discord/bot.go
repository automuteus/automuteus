package discord

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const AmongUsDefaultName = "Player"
const AmongUsDefaultColor = "Cyan"

const CommandPrefix = ".au"

type DiscordUser struct {
	nick          string
	userID        string
	userName      string
	discriminator string
}

type UserData struct {
	user         DiscordUser
	voiceState   discordgo.VoiceState
	tracking     bool
	amongUsColor string
	amongUsName  string
	amongUsAlive bool
}

//var VoiceStatusCache = make(map[string]UserData)
//var VoiceStatusCacheLock = sync.RWMutex{}
//
//var GameState = game.GameState{Phase: game.LOBBY}
//var GameStateLock = sync.RWMutex{}
//
type GameDelays struct {
	GameStartDelay           int
	GameResumeDelay          int
	DiscussStartDelay        int
	DiscordMuteDelayOffsetMs int
}

//all open websockets (might not be mapped to any guilds, yet)
var AllConns map[*websocket.Conn]string

var AllGuilds map[string]*GuildState

//
//var GlobalDelays GameDelays
//
//var ExclusiveChannelId = ""
//
//var TrackingVoiceId = ""
//var TrackingVoiceName = ""

func MakeAndStartBot(token string) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	dg.AddHandler(voiceStateChange)
	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(messageCreate)
	dg.AddHandler(newGuild)

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

	//mems, err := dg.GuildMembers(guild, "", 1000)
	//VoiceStatusCacheLock.Lock()
	//for _, v := range mems {
	//	VoiceStatusCache[v.User.ID] = UserData{
	//		user: DiscordUser{
	//			nick:          v.Nick,
	//			userID:        v.User.ID,
	//			userName:      v.User.Username,
	//			discriminator: v.User.Discriminator,
	//		},
	//		voiceState:   discordgo.VoiceState{},
	//		tracking:     false,
	//		amongUsColor: "NoColor",
	//		amongUsName:  "NoName",
	//		amongUsAlive: true,
	//	}
	//}
	//VoiceStatusCacheLock.Unlock()

	//if channel != "" {
	//	dg.ChannelMessageSend(channel, "Bot is Online!")
	//}
	AllGuilds = make(map[string]*GuildState)
	AllConns = make(map[*websocket.Conn]string)

	gameStateChannel := make(chan game.GenericWSMessage)

	go websocketServer(gameStateChannel)

	go discordListener(dg, gameStateChannel)

	<-sc

	//if channel != "" {
	//	dg.ChannelMessageSend(channel, "Bot is going Offline!")
	//}

	dg.Close()
}

var addr = flag.String("addr", "localhost:8080", "http service address")

var upgrader = websocket.Upgrader{} // use default options

type ChannelHttpWrapper struct {
	gameStateChannel chan<- game.GenericWSMessage
}

func (wrapper *ChannelHttpWrapper) handler(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		//log.Printf("Server recv: %s", message)
		msg := game.GenericWSMessage{}
		err = json.Unmarshal(message, &msg)
		if err != nil {
			log.Println(err)
			break
		}
		if msg.GuildID != "" {
			for gId, _ := range AllGuilds {
				if msg.GuildID == gId {
					AllConns[c] = gId
					log.Printf("Associated %s guild id w/ a websocket connection\n", gId)
				}
			}
		} else {
			if gId, ok := AllConns[c]; ok {
				msg.GuildID = gId
				wrapper.gameStateChannel <- msg
			}
		}

		//err = c.WriteMessage(mt, message)
		//if err != nil {
		//	log.Println("write:", err)
		//	break
		//}
	}
}

func websocketServer(gameStateChannel chan game.GenericWSMessage) {
	wrapper := ChannelHttpWrapper{gameStateChannel: gameStateChannel}

	http.HandleFunc("/status", wrapper.handler)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func discordListener(dg *discordgo.Session, newStateChannel <-chan game.GenericWSMessage) {
	for {
		newStateMsg := <-newStateChannel
		log.Printf("Received message for guild %s\n", newStateMsg.GuildID)
		if guild, ok := AllGuilds[newStateMsg.GuildID]; ok {
			newState := game.GameState{}
			err := json.Unmarshal(newStateMsg.Payload, &newState)
			if err != nil {
				log.Println(err)
				break
			}
			//log.Printf("Unmarshalled state object: %s\n", newState.ToString())
			switch newState.Phase {
			case game.LOBBY:
				log.Println("Detected transition to lobby")
				//if ExclusiveChannelId != "" {
				//	dg.ChannelMessageSend(ExclusiveChannelId, fmt.Sprintf("Game over! Unmuting players!"))
				//}
				//Loop through and reset players (game over = everyone alive again)
				guild.voiceStatusCacheLock.Lock()
				for i, v := range guild.VoiceStatusCache {
					v.amongUsAlive = true
					guild.VoiceStatusCache[i] = v
				}
				guild.voiceStatusCacheLock.Unlock()
				guild.muteAllTrackedMembers(dg, false, false)
				guild.gameStateLock.Lock()
				guild.GameState = newState
				guild.gameStateLock.Unlock()
			case game.TASKS:
				log.Println("Detected transition to tasks")
				delay := 0
				guild.gameStateLock.RLock()
				if guild.GameState.Phase == game.LOBBY {
					delay = guild.delays.GameStartDelay
				} else if guild.GameState.Phase == game.DISCUSS {
					delay = guild.delays.GameResumeDelay
				}
				//if ExclusiveChannelId != "" {
				//	dg.ChannelMessageSend(ExclusiveChannelId, fmt.Sprintf("Game starting; muting players in %d second(s)!", delay))
				//}
				guild.gameStateLock.RUnlock()

				time.Sleep(time.Second * time.Duration(delay))
				guild.muteAllTrackedMembers(dg, true, false)

				guild.gameStateLock.Lock()
				guild.GameState = newState
				guild.gameStateLock.Unlock()
			case game.DISCUSS:
				log.Println("Detected transition to discussion")
				//if ExclusiveChannelId != "" {
				//	dg.ChannelMessageSend(ExclusiveChannelId, fmt.Sprintf("Starting discussion; unmuting alive players in %d second(s)!", DiscussStartDelay))
				//}
				time.Sleep(time.Second * time.Duration(guild.delays.DiscussStartDelay))
				guild.gameStateLock.Lock()
				guild.GameState = newState
				guild.gameStateLock.Unlock()
				guild.muteAllTrackedMembers(dg, false, true)
			default:
				log.Println("Undetected new state!")
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

func newGuild(s *discordgo.Session, m *discordgo.GuildCreate) {
	log.Printf("Added to new Guild, id %s, name %s", m.Guild.ID, m.Guild.Name)
	AllGuilds[m.ID] = &GuildState{
		ID:                   m.ID,
		delays:               GameDelays{},
		VoiceStatusCache:     make(map[string]UserData),
		voiceStatusCacheLock: sync.RWMutex{},
		GameState:            game.GameState{Phase: game.UNINITIALIZED},
		gameStateLock:        sync.RWMutex{},
		Tracking:             make(map[string]Tracking),
		TextChannelId:        "",
	}
}

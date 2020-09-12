package discord

import (
	"bytes"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/capture"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

const CommandPrefix = ".au"

type DiscordUser struct {
	nick          string
	userID        string
	userName      string
	discriminator string
}

var GuildMembersCache = make(map[string]DiscordUser)

type UserData struct {
	user       DiscordUser
	voiceState discordgo.VoiceState

	amongUsColor string
	amongUsName  string
}

var VoiceStatusCache = make(map[string]UserData)

var ExclusiveChannelId = ""

func MakeAndStartBot(token, guild, channel string, results chan capture.GameState) {
	ExclusiveChannelId = channel
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	dg.AddHandler(voiceStateChange)
	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(messageCreate)
	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildVoiceStates | discordgo.IntentsGuildMessages)

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

	mems, err := dg.GuildMembers(guild, "", 1000)
	for _, v := range mems {
		GuildMembersCache[v.User.ID] = DiscordUser{
			nick:          v.Nick,
			userID:        v.User.ID,
			userName:      v.User.Username,
			discriminator: v.User.Discriminator,
		}
	}

	go discordListener(dg, guild, results)

	<-sc

	//kill the worker before we terminate the worker forcibly
	results <- capture.KILL

	dg.Close()
}

func discordListener(dg *discordgo.Session, guild string, res <-chan capture.GameState) {
	for {
		msg := <-res
		switch msg {
		case capture.KILL:
			return
		case capture.PREGAME:
			muteAllGuildMembers(dg, guild, false)
		case capture.GAME:
			muteAllGuildMembers(dg, guild, true)
		case capture.DISCUSS:
			muteAllGuildMembers(dg, guild, false)
		}
	}
}

func muteAllGuildMembers(dg *discordgo.Session, guildId string, mute bool) {

	for user, _ := range VoiceStatusCache {
		buf := bytes.NewBuffer([]byte{})
		if mute {
			buf.WriteString("Muting ")
		} else {
			buf.WriteString("Unmuting ")
		}
		buf.WriteString(user)
		//buf.WriteString(v.User.Username)
		//if v.Nick != "" {
		//buf.WriteString(fmt.Sprintf(" (%s)", v.Nick))
		//}
		log.Println(buf.String())
		err := guildMemberMute(dg, guildId, user, mute)
		if err != nil {
			log.Println(err)
		}
	}
}

func guildMemberMute(session *discordgo.Session, guildID string, userID string, mute bool) (err error) {
	data := struct {
		Mute bool `json:"mute"`
	}{mute}

	_, err = session.RequestWithBucketID("PATCH", discordgo.EndpointGuildMember(guildID, userID), data, discordgo.EndpointGuildMember(guildID, ""))
	return
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the authenticated bot has access to.
func voiceStateChange(s *discordgo.Session, m *discordgo.VoiceStateUpdate) {
	log.Println("Members Chunk Update")
	//if the user is already in the voice status cache
	if v, ok := VoiceStatusCache[m.UserID]; ok {
		v.voiceState = *m.VoiceState
		VoiceStatusCache[m.UserID] = v
	} else {
		user := DiscordUser{}
		//if we know of the user from the more general cache (we should)
		if v, ok := GuildMembersCache[m.UserID]; ok {
			user = v
		} else {
			//otherwise, construct a small record just of the userid
			user = DiscordUser{
				nick:          "",
				userID:        m.UserID,
				userName:      "",
				discriminator: "",
			}
		}

		VoiceStatusCache[m.UserID] = UserData{
			user:         user,
			voiceState:   *m.VoiceState,
			amongUsColor: "",
			amongUsName:  "",
		}
	}
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the authenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}

	if ExclusiveChannelId == "" || (ExclusiveChannelId == m.ChannelID) {
		contents := m.Content
		if strings.HasPrefix(contents, CommandPrefix) {
			args := strings.Split(contents, " ")[1:]
			for i, v := range args {
				args[i] = strings.ToLower(v)
			}

			if len(args) == 0 || args[0] == "help" || args[0] == "h" {
				_, err := s.ChannelMessageSend(m.ChannelID, helpResponse())
				if err != nil {
					log.Println(err)
				}
			} else if args[0] == "add" || args[0] == "a" {
				if len(args[1:]) == 0 {
					//TODO print usage of this command specifically
				} else {

					for _, v := range args[1:] {
						if strings.HasPrefix(v, "<@!") && strings.HasSuffix(v, ">") {
							//strip the special characters off front and end
							idLookup := v[3 : len(v)-1]
							for id, user := range GuildMembersCache {
								if id == idLookup {
									VoiceStatusCache[id] = UserData{
										user:         user,
										voiceState:   discordgo.VoiceState{},
										amongUsColor: "",
										amongUsName:  "",
									}
									if user.nick != "" {
										s.ChannelMessageSend(m.ChannelID, "Added "+user.userName+" ("+user.nick+") to game!")
									} else {
										s.ChannelMessageSend(m.ChannelID, "Added "+user.userName+" to game!")
									}
									break
								}
							}
						} else {
							s.ChannelMessageSend(m.ChannelID, "I don't currently support any syntax beyond `@` direct mentions, sorry!")
						}
					}
				}
			}
		}
	}

}

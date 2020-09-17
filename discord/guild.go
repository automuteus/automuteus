package discord

import (
	"bytes"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"log"
	"strings"
	"sync"
	"time"
)

type Tracking struct {
	channelId   string
	channelName string
}

type GuildState struct {
	ID     string
	delays GameDelays

	UserData     map[string]UserData
	UserDataLock sync.RWMutex

	GamePhase     game.GamePhase
	GamePhaseLock sync.RWMutex

	Tracking map[string]Tracking

	//UNUSED right now
	TextChannelId string
}

func (guild *GuildState) muteAllTrackedMembers(dg *discordgo.Session, mute bool, checkAlive bool) {
	skipExec := false
	guild.UserDataLock.RLock()
	for user, v := range guild.UserData {
		if v.tracking {
			buf := bytes.NewBuffer([]byte{})
			if mute {
				buf.WriteString("Muting ")
			} else {
				if checkAlive {
					if v.amongUsAlive {
						buf.WriteString("Unmuting (alive) ")
					} else {
						buf.WriteString("Not Unmuting (dead) ")
						skipExec = true
					}
				} else {
					buf.WriteString("Unmuting ")
				}
			}
			buf.WriteString(fmt.Sprintf("Username: %s, Nickname: %s, ID: %s", v.user.userName, v.user.nick, user))
			//buf.WriteString(v.User.Username)
			//if v.Nick != "" {
			//buf.WriteString(fmt.Sprintf(" (%s)", v.Nick))
			//}
			log.Println(buf.String())
			if !skipExec {
				err := guildMemberMute(dg, guild.ID, user, mute)
				if err != nil {
					log.Println(err)
				}
				log.Printf("Sleeping for %dms between mutes to avoid being rate-limited by Discord\n", guild.delays.DiscordMuteDelayOffsetMs)
				time.Sleep(time.Duration(guild.delays.DiscordMuteDelayOffsetMs) * time.Millisecond)
			}
		}
	}
	guild.UserDataLock.RUnlock()
}
func guildMemberMute(session *discordgo.Session, guildID string, userID string, mute bool) (err error) {
	log.Println("Issuing mute request to discord")
	data := struct {
		Mute bool `json:"mute"`
	}{mute}

	_, err = session.RequestWithBucketID("PATCH", discordgo.EndpointGuildMember(guildID, userID), data, discordgo.EndpointGuildMember(guildID, ""))
	return
}

func (guild *GuildState) voiceStateChange(s *discordgo.Session, m *discordgo.VoiceStateUpdate) {

	guild.updateVoiceStatusCache(s)

	//if the user is already in the voice status cache, only update if we don't know the voice channel to track,
	//or the user has ENTERED this voice channel
	guild.UserDataLock.Lock()
	if v, ok := guild.UserData[m.UserID]; ok {
		v.voiceState = *m.VoiceState

		//only track if we have no tracked channel so far, or the user is in the tracked channel
		v.tracking = guild.isTracked(m.ChannelID)

		guild.UserData[m.UserID] = v
		log.Printf("Saw a cached \"%s\" user's voice status change, tracking: %v\n", v.user.userName, v.tracking)
		//unmute the member if they left the chat while muted
		if !v.tracking && m.Mute {
			log.Println("Untracked mute")
			guildMemberMute(s, m.GuildID, m.UserID, false)

			//if the user rejoins, only mute if the game is going, or if it's discussion and they're dead
		} else {
			guild.GamePhaseLock.RLock()
			if v.tracking && !m.Mute && (guild.GamePhase == game.TASKS || (guild.GamePhase == game.DISCUSS && !v.amongUsAlive)) {
				log.Println("Tracked mute")
				log.Printf("Current game state: %d, alive: %v", guild.GamePhase, v.amongUsAlive)
				guildMemberMute(s, m.GuildID, m.UserID, true)
			}
			guild.GamePhaseLock.RUnlock()
		}
	} else {
		user := DiscordUser{
			nick:          "",
			userID:        m.UserID,
			userName:      "",
			discriminator: "",
		}
		//only track if we have no tracked channel so far, or the user is in the tracked channel. Otherwise, don't track
		tracking := guild.isTracked(m.ChannelID)
		log.Printf("Saw \"%s\" user's voice status change, tracking: %v\n", user.userName, tracking)
		guild.UserData[m.UserID] = UserData{
			user:         user,
			voiceState:   *m.VoiceState,
			tracking:     tracking,
			amongUsColor: AmongUsDefaultColor,
			amongUsName:  AmongUsDefaultName,
			amongUsAlive: true,
		}
	}
	guild.UserDataLock.Unlock()
}

func (guild *GuildState) updateVoiceStatusCache(s *discordgo.Session) {
	g, err := s.State.Guild(guild.ID)
	if err != nil {
		log.Println(err)
	}

	//make sure all the people in the voice status cache are still in voice
	guild.UserDataLock.Lock()
	for id, v := range guild.UserData {
		foundUser := false
		for _, state := range g.VoiceStates {
			if state.UserID == id {
				foundUser = true
				break
			}
		}
		//TODO can you server unmute someone not in voice? Prob not...
		if !foundUser {
			v.tracking = false
			guild.UserData[id] = v
		}
	}
	guild.UserDataLock.Unlock()

	for _, state := range g.VoiceStates {
		guild.UserDataLock.Lock()
		//update the voicestatus of the user
		if v, ok := guild.UserData[state.UserID]; ok {
			v.voiceState = *state
			v.tracking = guild.isTracked(state.ChannelID)
			guild.UserData[state.UserID] = v
		} else { //add the user we haven't seen in our cache before
			user := DiscordUser{
				userID: state.UserID,
			}

			guild.UserData[state.UserID] = UserData{
				user:         user,
				voiceState:   *state,
				tracking:     guild.isTracked(state.ChannelID),
				amongUsColor: "Cyan",
				amongUsName:  "Player",
				amongUsAlive: true,
			}
		}
		guild.UserDataLock.Unlock()
	}
}
func (guild *GuildState) handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	guild.updateVoiceStatusCache(s)

	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}

	if guild.isTracked(m.ChannelID) {
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
				case "add":
					fallthrough
				case "a":
					if len(args[1:]) == 0 {
						//TODO print usage of this command specifically
						s.ChannelMessageSend(m.ChannelID, "You used this command incorrectly! Please refer to `.au help` for proper command usage")
					} else {
						responses := guild.processAddUsersArgs(args[1:])
						buf := bytes.NewBuffer([]byte("Results:\n"))
						for name, msg := range responses {
							buf.WriteString(fmt.Sprintf("`%s`: %s\n", name, msg))
						}
						s.ChannelMessageSend(m.ChannelID, buf.String())
					}
					break
				case "track":
					fallthrough
				case "t":
					if len(args[1:]) == 0 {
						//TODO print usage of this command specifically
						s.ChannelMessageSend(m.ChannelID, "You used this command incorrectly! Please refer to `.au help` for proper command usage")
					} else {
						channelName := strings.Join(args[1:], " ")

						channels, err := s.GuildChannels(m.GuildID)
						if err != nil {
							log.Println(err)
						}

						resp := guild.processTrackChannelArg(channelName, channels)
						s.ChannelMessageSend(m.ChannelID, resp)
					}
					break
				case "list":
					fallthrough
				case "l":
					resp := guild.playerListResponse()
					s.ChannelMessageSend(m.ChannelID, resp)
					break
				case "reset":
					fallthrough
				case "r":
					guild.UserDataLock.Lock()
					for i, v := range guild.UserData {
						v.tracking = false
						v.amongUsAlive = true
						guild.UserData[i] = v
					}
					guild.UserDataLock.Unlock()
					s.ChannelMessageSend(m.ChannelID, "Reset Player List!")
					break
				case "dead":
					fallthrough
				case "d":
					if len(args[1:]) == 0 {
						//TODO print usage of this command specifically
						s.ChannelMessageSend(m.ChannelID, "You used this command incorrectly! Please refer to `.au help` for proper command usage")
					} else {
						responses := guild.processMarkAliveUsers(s, args[1:], false)
						buf := bytes.NewBuffer([]byte("Results:\n"))
						for name, msg := range responses {
							buf.WriteString(fmt.Sprintf("`%s`: %s\n", name, msg))
						}
						s.ChannelMessageSend(m.ChannelID, buf.String())
					}
					break
				case "alive":
					fallthrough
				case "al":
					if len(args[1:]) == 0 {
						//TODO print usage of this command specifically
						s.ChannelMessageSend(m.ChannelID, "You used this command incorrectly! Please refer to `.au help` for proper command usage")
					} else {
						responses := guild.processMarkAliveUsers(s, args[1:], true)
						buf := bytes.NewBuffer([]byte("Results:\n"))
						for name, msg := range responses {
							buf.WriteString(fmt.Sprintf("`%s`: %s\n", name, msg))
						}
						s.ChannelMessageSend(m.ChannelID, buf.String())
					}
					break
				case "unmuteall":
					fallthrough
				case "ua":
					s.ChannelMessageSend(m.ChannelID, "Forcibly unmuting ALL players!")
					guild.UserDataLock.RLock()
					for id, _ := range guild.UserData {
						err := guildMemberMute(s, m.GuildID, id, false)
						if err != nil {
							log.Println(err)
						}
					}
					guild.UserDataLock.RUnlock()
					break
				case "muteall":
					fallthrough
				case "ma":
					s.ChannelMessageSend(m.ChannelID, "Forcibly muting ALL players!")
					guild.UserDataLock.RLock()
					for id, _ := range guild.UserData {
						err := guildMemberMute(s, m.GuildID, id, true)
						if err != nil {
							log.Println(err)
						}

					}
					guild.UserDataLock.RUnlock()
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
						str, err := guild.processBroadcastArgs(args[1:])
						if err != nil {
							log.Println(err)
						}
						s.ChannelMessageSend(m.ChannelID, str)
					}
				}
			}
		}
	}
}

func (guild *GuildState) processBroadcastArgs(args []string) (string, error) {
	buf := bytes.NewBuffer([]byte{})
	code, region := "", ""
	//just the room code
	code = strings.ToUpper(args[0])

	if len(args) > 1 {
		region = strings.ToLower(args[1])
		switch region {
		case "na":
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
	}
	guild.UserDataLock.RLock()
	for _, player := range guild.UserData {
		if player.tracking {
			buf.WriteString(fmt.Sprintf("<@!%s> ", player.user.userID))
		}
	}
	guild.UserDataLock.RUnlock()
	buf.WriteString(fmt.Sprintf("\nThe Room Code is **%s**\n", code))

	if region == "" {
		buf.WriteString("I wasn't told the Region, though :cry:")
	} else {
		buf.WriteString(fmt.Sprintf("The Region is **%s**\n", region))
	}
	return buf.String(), nil
}

func (guild *GuildState) processTrackChannelArg(channelName string, allChannels []*discordgo.Channel) string {
	for _, c := range allChannels {
		if (strings.ToLower(c.Name) == strings.ToLower(channelName) || c.ID == channelName) && c.Type == 2 {
			//TODO check duplicates (for logging)
			guild.Tracking[c.ID] = Tracking{
				channelId:   c.ID,
				channelName: c.Name,
			}
			return fmt.Sprintf("Now tracking \"%s\" Voice Channel for Automute!", c.Name)
		}
	}
	return fmt.Sprintf("No channel found by the name %s!\n", channelName)
}

func (guild *GuildState) playerListResponse() string {
	buf := bytes.NewBuffer([]byte{})
	//TODO print the tracked again
	//if TrackingVoiceId != "" {
	//	buf.WriteString(fmt.Sprintf("Currently tracking \"%s\" Voice Channel:\n", TrackingVoiceName))
	//} else {
	//	buf.WriteString("Not tracking a Voice Channel; all players will be Automuted (use `.au t` to track)\n")
	//}

	buf.WriteString("Player List:\n")
	guild.UserDataLock.RLock()
	for _, player := range guild.UserData {
		if player.tracking {
			emoji := ":heart:"
			if !player.amongUsAlive {
				emoji = ":skull:"
			}
			buf.WriteString(fmt.Sprintf("<@!%s>: %s (%s) %s\n", player.user.userID, player.amongUsName, player.amongUsColor, emoji))
		}
	}
	guild.UserDataLock.RUnlock()
	return buf.String()
}

func (guild *GuildState) processMarkAliveUsers(dg *discordgo.Session, args []string, markAlive bool) map[string]string {
	responses := make(map[string]string)
	for _, v := range args {
		if strings.HasPrefix(v, "<@!") && strings.HasSuffix(v, ">") {
			//strip the special characters off front and end
			idLookup := v[3 : len(v)-1]
			guild.UserDataLock.Lock()
			for id, user := range guild.UserData {
				if id == idLookup {
					temp := guild.UserData[id]
					temp.amongUsAlive = markAlive
					guild.UserData[id] = temp

					nameIdx := user.user.userName
					if user.user.nick != "" {
						nameIdx = user.user.userName + " (" + user.user.nick + ")"
					}
					if markAlive {
						responses[nameIdx] = "Marked Alive"
					} else {
						responses[nameIdx] = "Marked Dead"
					}

					guild.GamePhaseLock.RLock()
					if guild.GamePhase == game.DISCUSS {
						err := guildMemberMute(dg, guild.ID, id, !markAlive)
						if err != nil {
							log.Printf("Error muting/unmuting %s: %s\n", user.user.userName, err)
						}
						if markAlive {
							responses[nameIdx] = "Marked Alive and Unmuted"
						} else {
							responses[nameIdx] = "Marked Dead and Muted"
						}

					}
					guild.GamePhaseLock.RUnlock()
				}
			}
			guild.UserDataLock.Unlock()
		} else {
			responses[v] = "Not currently supporting non-`@` direct mentions, sorry!"
		}
	}
	return responses
}

func (guild *GuildState) processAddUsersArgs(args []string) map[string]string {
	responses := make(map[string]string)
	for _, v := range args {
		if strings.HasPrefix(v, "<@!") && strings.HasSuffix(v, ">") {
			//strip the special characters off front and end
			idLookup := v[3 : len(v)-1]
			guild.UserDataLock.Lock()
			for id, user := range guild.UserData {
				if id == idLookup {
					guild.UserData[id] = UserData{
						user:         user.user,
						voiceState:   discordgo.VoiceState{},
						tracking:     true, //always assume true if we're adding users manually
						amongUsColor: AmongUsDefaultColor,
						amongUsName:  AmongUsDefaultName,
						amongUsAlive: true,
					}
					nameIdx := user.user.userName
					if user.user.nick != "" {
						nameIdx = user.user.userName + " (" + user.user.nick + ")"
					}
					responses[nameIdx] = "Added successfully!"
				}
			}
			guild.UserDataLock.Unlock()
		} else {
			responses[v] = "Not currently supporting non-`@` direct mentions, sorry!"
		}
	}
	return responses
}

func (guild *GuildState) isTracked(channelID string) bool {
	if len(guild.Tracking) == 0 {
		return true
	} else {
		for _, v := range guild.Tracking {
			if v.channelId == channelID {
				return true
			}
		}
	}
	return false
}

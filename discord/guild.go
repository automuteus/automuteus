package discord

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
)

// Tracking struct
type Tracking struct {
	channelID   string
	channelName string
	forGhosts   bool
}

// GuildState struct
type GuildState struct {
	ID     string
	delays GameDelays

	VoiceStatusCache     map[string]UserData
	voiceStatusCacheLock sync.RWMutex

	GameState     game.GameState
	gameStateLock sync.RWMutex

	Tracking map[string]Tracking

	//UNUSED right now
	TextChannelID string

	// For voice channel movement
	MoveDeadPlayers bool
}

// TrackedMemberAction struct
type TrackedMemberAction struct {
	mute          bool
	move          bool
	message       string
	targetChannel Tracking
}

func (guild *GuildState) handleTrackedMembers(dg *discordgo.Session, inGame bool, inDiscussion bool) {
	// first we need to determine if we care about moving people at all
	if guild.MoveDeadPlayers {
		guild.moveAndMuteAllTrackedMembers(dg, inGame, inDiscussion)
		return
	}

	guild.muteAllTrackedMembers(dg, inGame, inDiscussion)
}

func (guild *GuildState) moveAndMuteAllTrackedMembers(dg *discordgo.Session, inGame bool, inDiscussion bool) {
	guild.voiceStatusCacheLock.RLock()

	deadUserChannel, deadUserChannelError := guild.findVoiceChannel(true)
	if deadUserChannelError != nil {
		log.Printf("Missing a voice channel for dead users!")
		return
	}
	gameChannel, gameChannelError := guild.findVoiceChannel(false)
	if gameChannelError != nil {
		log.Printf("Missing a voice channel for alive users!")
		return
	}

	// inGame, inDiscussion, isAlive is the key order
	actions := map[bool]map[bool]map[bool]TrackedMemberAction{
		// inGame
		true: map[bool]map[bool]TrackedMemberAction{
			// not inDiscussion
			false: map[bool]TrackedMemberAction{
				// isAlive
				true: {true, false, "Not moving, and muting ", gameChannel},
				// not isAlive
				false: {false, true, "Moving to dead channel, and unmuting ", deadUserChannel},
			},
		},
		// not inGame
		false: map[bool]map[bool]TrackedMemberAction{
			// inDiscussion
			true: map[bool]TrackedMemberAction{
				// isAlive
				true: {false, false, "Not moving, and unmuting ", gameChannel},
				// not isAlive
				false: {true, true, "Moving to game channel, and muting", gameChannel},
			},
		},
	}

	for user, v := range guild.VoiceStatusCache {
		if v.tracking {
			action := actions[inGame][inDiscussion][v.amongUsAlive]

			log.Println(fmt.Sprintf("%s Username: %s, Nickname: %s, ID: %s", action.message, v.user.userName, v.user.nick, user))

			if action.move {
				moveErr := guildMemberMove(dg, guild.ID, user, &action.targetChannel.channelID)
				if moveErr != nil {
					log.Println(moveErr)
					continue
				}
			}

			err := guildMemberMute(dg, guild.ID, user, action.mute)
			if err != nil {
				log.Println(err)
			}
			log.Printf("Sleeping for %dms between actions to avoid being rate-limited by Discord\n", guild.delays.DiscordMuteDelayOffsetMs)
			time.Sleep(time.Duration(guild.delays.DiscordMuteDelayOffsetMs) * time.Millisecond)
		}
	}
	guild.voiceStatusCacheLock.RUnlock()
}

func (guild *GuildState) muteAllTrackedMembers(dg *discordgo.Session, mute bool, checkAlive bool) {
	skipExec := false
	guild.voiceStatusCacheLock.RLock()
	for user, v := range guild.VoiceStatusCache {
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
			log.Println(buf.String())
			if !skipExec {
				err := guildMemberMute(dg, guild.ID, user, mute)
				if err != nil {
					log.Println(err)
				}
				log.Printf("Sleeping for %dms between actions to avoid being rate-limited by Discord\n", guild.delays.DiscordMuteDelayOffsetMs)
				time.Sleep(time.Duration(guild.delays.DiscordMuteDelayOffsetMs) * time.Millisecond)
			}
		}
	}
	guild.voiceStatusCacheLock.RUnlock()
}

func guildMemberMove(session *discordgo.Session, guildID string, userID string, channelID *string) (err error) {
	log.Println("Issuing move channel request to discord")
	data := struct {
		ChannelID *string `json:"channel_id"`
	}{channelID}

	_, err = session.RequestWithBucketID("PATCH", discordgo.EndpointGuildMember(guildID, userID), data, discordgo.EndpointGuildMember(guildID, ""))
	return
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
	guild.voiceStatusCacheLock.Lock()
	if v, ok := guild.VoiceStatusCache[m.UserID]; ok {
		v.voiceState = *m.VoiceState

		//only track if we have no tracked channel so far, or the user is in the tracked channel
		v.tracking = guild.isTracked(m.ChannelID)

		guild.VoiceStatusCache[m.UserID] = v
		log.Printf("Saw a cached \"%s\" user's voice status change, tracking: %v\n", v.user.userName, v.tracking)
		//unmute the member if they left the chat while muted
		if !v.tracking && m.Mute {
			log.Println("Untracked mute")
			guildMemberMute(s, m.GuildID, m.UserID, false)

			//if the user rejoins, only mute if the game is going, or if it's discussion and they're dead
		} else {
			guild.gameStateLock.RLock()
			if v.tracking && !m.Mute && (guild.GameState.Phase == game.TASKS || (guild.GameState.Phase == game.DISCUSS && !v.amongUsAlive)) {
				log.Println("Tracked mute")
				log.Printf("Current game state: %v, alive: %v", guild.GameState, v.amongUsAlive)
				guildMemberMute(s, m.GuildID, m.UserID, true)
			}
			guild.gameStateLock.RUnlock()
		}
	} else {
		user := User{
			nick:          "",
			userID:        m.UserID,
			userName:      "",
			discriminator: "",
		}
		//only track if we have no tracked channel so far, or the user is in the tracked channel. Otherwise, don't track
		tracking := guild.isTracked(m.ChannelID)
		log.Printf("Saw \"%s\" user's voice status change, tracking: %v\n", user.userName, tracking)
		guild.VoiceStatusCache[m.UserID] = UserData{
			user:         user,
			voiceState:   *m.VoiceState,
			tracking:     tracking,
			amongUsColor: AmongUsDefaultColor,
			amongUsName:  AmongUsDefaultName,
			amongUsAlive: true,
		}
	}
	guild.voiceStatusCacheLock.Unlock()
}

func (guild *GuildState) updateVoiceStatusCache(s *discordgo.Session) {
	g, err := s.State.Guild(guild.ID)
	if err != nil {
		log.Println(err)
	}

	//make sure all the people in the voice status cache are still in voice
	guild.voiceStatusCacheLock.Lock()
	for id, v := range guild.VoiceStatusCache {
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
			guild.VoiceStatusCache[id] = v
		}
	}
	guild.voiceStatusCacheLock.Unlock()

	for _, state := range g.VoiceStates {
		guild.voiceStatusCacheLock.Lock()
		//update the voicestatus of the user
		if v, ok := guild.VoiceStatusCache[state.UserID]; ok {
			v.voiceState = *state
			v.tracking = guild.isTracked(state.ChannelID)
			guild.VoiceStatusCache[state.UserID] = v
		} else { //add the user we haven't seen in our cache before
			user := User{
				userID: state.UserID,
			}

			guild.VoiceStatusCache[state.UserID] = UserData{
				user:         user,
				voiceState:   *state,
				tracking:     guild.isTracked(state.ChannelID),
				amongUsColor: "Cyan",
				amongUsName:  "Player",
				amongUsAlive: true,
			}
		}
		guild.voiceStatusCacheLock.Unlock()
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
						// if anything is given in the second slot then we consider that a true
						forGhosts := len(args[2:]) >= 1
						channelName := strings.Join(args[1:2], " ")

						channels, err := s.GuildChannels(m.GuildID)
						if err != nil {
							log.Println(err)
						}

						resp := guild.processTrackChannelArg(channelName, channels, forGhosts)
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
					guild.voiceStatusCacheLock.Lock()
					for i, v := range guild.VoiceStatusCache {
						v.tracking = false
						v.amongUsAlive = true
						guild.VoiceStatusCache[i] = v
					}
					guild.voiceStatusCacheLock.Unlock()
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
					guild.voiceStatusCacheLock.RLock()
					for id := range guild.VoiceStatusCache {
						err := guildMemberMute(s, m.GuildID, id, false)
						if err != nil {
							log.Println(err)
						}
					}
					guild.voiceStatusCacheLock.RUnlock()
					break
				case "muteall":
					fallthrough
				case "ma":
					s.ChannelMessageSend(m.ChannelID, "Forcibly muting ALL players!")
					guild.voiceStatusCacheLock.RLock()
					for id := range guild.VoiceStatusCache {
						err := guildMemberMute(s, m.GuildID, id, true)
						if err != nil {
							log.Println(err)
						}

					}
					guild.voiceStatusCacheLock.RUnlock()
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
					break
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
	guild.voiceStatusCacheLock.RLock()
	for _, player := range guild.VoiceStatusCache {
		if player.tracking {
			buf.WriteString(fmt.Sprintf("<@!%s> ", player.user.userID))
		}
	}
	guild.voiceStatusCacheLock.RUnlock()
	buf.WriteString(fmt.Sprintf("\nThe Room Code is **%s**\n", code))

	if region == "" {
		buf.WriteString("I wasn't told the Region, though :cry:")
	} else {
		buf.WriteString(fmt.Sprintf("The Region is **%s**\n", region))
	}
	return buf.String(), nil
}

func (guild *GuildState) processTrackChannelArg(channelName string, allChannels []*discordgo.Channel, forGhosts bool) string {
	for _, c := range allChannels {
		if (strings.ToLower(c.Name) == strings.ToLower(channelName) || c.ID == channelName) && c.Type == 2 {
			//TODO check duplicates (for logging)
			guild.Tracking[c.ID] = Tracking{
				channelID:   c.ID,
				channelName: c.Name,
				forGhosts:   forGhosts,
			}
			return fmt.Sprintf("Now tracking \"%s\" Voice Channel for Automute (for ghosts? %v)!", c.Name, forGhosts)
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
	guild.voiceStatusCacheLock.RLock()
	for _, player := range guild.VoiceStatusCache {
		if player.tracking {
			emoji := ":heart:"
			if !player.amongUsAlive {
				emoji = ":skull:"
			}
			buf.WriteString(fmt.Sprintf("<@!%s>: %s (%s) %s\n", player.user.userID, player.amongUsName, player.amongUsColor, emoji))
		}
	}
	guild.voiceStatusCacheLock.RUnlock()
	return buf.String()
}

func (guild *GuildState) processMarkAliveUsers(dg *discordgo.Session, args []string, markAlive bool) map[string]string {
	responses := make(map[string]string)
	for _, v := range args {
		if strings.HasPrefix(v, "<@!") && strings.HasSuffix(v, ">") {
			//strip the special characters off front and end
			idLookup := v[3 : len(v)-1]
			guild.voiceStatusCacheLock.Lock()
			for id, user := range guild.VoiceStatusCache {
				if id == idLookup {
					temp := guild.VoiceStatusCache[id]
					temp.amongUsAlive = markAlive
					guild.VoiceStatusCache[id] = temp

					nameIdx := user.user.userName
					if user.user.nick != "" {
						nameIdx = user.user.userName + " (" + user.user.nick + ")"
					}
					if markAlive {
						responses[nameIdx] = "Marked Alive"
					} else {
						responses[nameIdx] = "Marked Dead"
					}

					guild.gameStateLock.RLock()
					if guild.GameState.Phase == game.DISCUSS {
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
					guild.gameStateLock.RUnlock()
				}
			}
			guild.voiceStatusCacheLock.Unlock()
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
			guild.voiceStatusCacheLock.Lock()
			for id, user := range guild.VoiceStatusCache {
				if id == idLookup {
					guild.VoiceStatusCache[id] = UserData{
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
			guild.voiceStatusCacheLock.Unlock()
		} else {
			responses[v] = "Not currently supporting non-`@` direct mentions, sorry!"
		}
	}
	return responses
}

func (guild *GuildState) isTracked(channelID string) bool {
	if len(guild.Tracking) == 0 {
		return true
	}

	for _, v := range guild.Tracking {
		if v.channelID == channelID {
			return true
		}
	}
	return false
}

func (guild *GuildState) findVoiceChannel(forGhosts bool) (Tracking, error) {
	for _, v := range guild.Tracking {
		if v.forGhosts == forGhosts {
			return v, nil
		}
	}

	return Tracking{}, fmt.Errorf("No voice channel found forGhosts: %v", forGhosts)
}

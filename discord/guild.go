package discord

import (
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

	UserData     map[string]UserData
	UserDataLock sync.RWMutex

	//indexed by color
	AmongUsData     []AmongUserData
	AmongUsDataLock sync.RWMutex

	GamePhase     game.GamePhase
	GamePhaseLock sync.RWMutex

	Tracking     map[string]Tracking
	TrackingLock sync.RWMutex

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
	guild.UserDataLock.RLock()

	deadUserChannel, deadUserChannelError := guild.findVoiceChannel(true)
	if guild.MoveDeadPlayers && deadUserChannelError != nil {
		log.Printf("MoveDeadPlayers is true, but I'm missing a voice channel for dead users!")
		return
	}
	gameChannel, gameChannelError := guild.findVoiceChannel(false)
	if guild.MoveDeadPlayers && gameChannelError != nil {
		log.Printf("MoveDeadPlayers is true, but I'm missing a voice channel for alive users!")
		return
	}
	var moveToDeadText, moveToAliveText string

	//Change the log message depending on if we're moving members or not
	if guild.MoveDeadPlayers {
		moveToDeadText = "Moving to dead channel, and unmuting "
		moveToAliveText = "Moving to game channel, and muting "
	} else {
		moveToDeadText = "Not moving, and unmuting "
		moveToAliveText = "Not moving, and muting "
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
				false: {false, guild.MoveDeadPlayers, moveToDeadText, deadUserChannel},
			},
		},
		// not inGame
		false: map[bool]map[bool]TrackedMemberAction{
			// inDiscussion
			true: map[bool]TrackedMemberAction{
				// isAlive
				true: {false, false, "Not moving, and unmuting ", gameChannel},
				// not isAlive
				false: {true, guild.MoveDeadPlayers, moveToAliveText, gameChannel},
			},
		},
	}

	for user, v := range guild.UserData {
		if v.tracking {
			action := actions[inGame][inDiscussion][v.IsAlive()]

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
	guild.UserDataLock.RUnlock()
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
			if v.tracking && !m.Mute && (guild.GamePhase == game.TASKS || (guild.GamePhase == game.DISCUSS && !v.IsAlive())) {
				log.Println("Tracked mute")
				log.Printf("Current game state: %d, alive: %v", guild.GamePhase, v.IsAlive())
				guildMemberMute(s, m.GuildID, m.UserID, true)
			}
			guild.GamePhaseLock.RUnlock()
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
		guild.UserData[m.UserID] = UserData{
			user:       user,
			voiceState: *m.VoiceState,
			tracking:   tracking,
			auData:     nil,
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
			user := User{
				userID: state.UserID,
			}

			guild.UserData[state.UserID] = UserData{
				user:       user,
				voiceState: *state,
				tracking:   guild.isTracked(state.ChannelID),
				auData:     nil,
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

	//TODO This should check VOICE channels, not TEXT channels
	//if guild.isTracked(m.ChannelID) {
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
			//case "add":
			//	fallthrough
			//case "a":
			//	if len(args[1:]) == 0 {
			//		//TODO print usage of this command specifically
			//		s.ChannelMessageSend(m.ChannelID, "You used this command incorrectly! Please refer to `.au help` for proper command usage")
			//	} else {
			//		responses := guild.processAddUsersArgs(args[1:])
			//		buf := bytes.NewBuffer([]byte("Results:\n"))
			//		for name, msg := range responses {
			//			buf.WriteString(fmt.Sprintf("`%s`: %s\n", name, msg))
			//		}
			//		s.ChannelMessageSend(m.ChannelID, buf.String())
			//	}
			//	break
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
				resp := guild.playerListResponse()
				_, err := s.ChannelMessageSend(m.ChannelID, resp)
				if err != nil {
					log.Println(err)
				}
				break

			case "link":
				if len(args[1:]) < 2 {
					//TODO print usage of this command specifically
					s.ChannelMessageSend(m.ChannelID, "You used this command incorrectly! Please refer to `.au help` for proper command usage")
				} else {
					guild.AmongUsDataLock.Lock()
					guild.UserDataLock.Lock()
					resp := guild.linkPlayerResponse(args[1:], &guild.AmongUsData)
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
			default:
				s.ChannelMessageSend(m.ChannelID, "Sorry, I didn't understand that command! Please see `.au help` for commands")
			}
		}
		//}
	}
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

func (guild *GuildState) updateCachedAmongUsData(update game.Player) bool {
	guild.AmongUsDataLock.Lock()
	isUpdate := guild.AmongUsData[update.Color].isDifferent(update)
	if isUpdate {
		guild.AmongUsData[update.Color] = AmongUserData{
			Color:   update.Color,
			Name:    update.Name,
			IsAlive: !update.IsDead,
		}
		log.Printf("Updated %s", guild.AmongUsData[update.Color].ToString())
	}
	guild.AmongUsDataLock.Unlock()
	return isUpdate
}

func (guild *GuildState) modifyCachedAmongUsDataAlive(alive bool) {
	for i := range guild.AmongUsData {
		guild.AmongUsData[i].IsAlive = alive
	}
}

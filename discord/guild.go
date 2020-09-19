package discord

import (
	"fmt"
	"log"
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

	//indexed by amongusname
	AmongUsData     map[string]*AmongUserData
	AmongUsDataLock sync.RWMutex

	GamePhase     game.GamePhase
	GamePhaseLock sync.RWMutex

	Tracking     map[string]Tracking
	TrackingLock sync.RWMutex

	//UNUSED right now
	TextChannelID string

	// For voice channel movement
	MoveDeadPlayers bool

	GameStateMessage     *discordgo.Message
	GameStateMessageLock sync.RWMutex
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
	defer guild.AmongUsDataLock.Unlock()

	if _, ok := guild.AmongUsData[update.Name]; !ok {
		guild.AmongUsData[update.Name] = &AmongUserData{
			Color:   update.Color,
			Name:    update.Name,
			IsAlive: !update.IsDead,
		}
		log.Printf("Added new player instance for %s\n", update.Name)
		return true
	}
	guildDataTempPtr := guild.AmongUsData[update.Name]
	isUpdate := guildDataTempPtr.isDifferent(update)
	if isUpdate {
		(*guild.AmongUsData[update.Name]).Color = update.Color
		(*guild.AmongUsData[update.Name]).Name = update.Name
		(*guild.AmongUsData[update.Name]).IsAlive = !update.IsDead

		log.Printf("Updated %s", (*guild.AmongUsData[update.Name]).ToString())
	}

	return isUpdate
}

func (guild *GuildState) modifyCachedAmongUsDataAlive(alive bool) {
	for i := range guild.AmongUsData {
		guildDataPtr := guild.AmongUsData[i]
		guildDataPtr.IsAlive = alive

		//TODO my pointer knowledge is failing me; this isn't needed, right?
		guild.AmongUsData[i] = guildDataPtr
	}
}

// ToString returns a simple string representation of the current state of the guild
func (guild *GuildState) ToString() string {
	return fmt.Sprintf("%v", guild)
}

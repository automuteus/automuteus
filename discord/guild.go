package discord

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"log"
	"sync"
)

// Tracking struct
type Tracking struct {
	channelID   string
	channelName string
	forGhosts   bool
}

// GameDelays struct
type GameDelays struct {
	GameStartDelay           int
	GameResumeDelay          int
	DiscussStartDelay        int
	DiscordMuteDelayOffsetMs int
}

// GuildState struct
type GuildState struct {
	ID            string
	CommandPrefix string
	LinkCode      string

	UserData map[string]UserData
	Tracking map[string]Tracking
	//use this to refer to the same state message and update it on ls
	GameStateMessage *discordgo.Message
	delays           GameDelays
	UserDataLock     sync.RWMutex

	//indexed by amongusname
	AmongUsData map[string]*AmongUserData
	//what current phase the game is in (lobby, tasks, discussion)
	GamePhase       game.Phase
	Room            string
	Region          string
	AmongUsDataLock sync.RWMutex

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
	defer guild.UserDataLock.RUnlock()

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

			log.Println(fmt.Sprintf("%s Username: %s, Nickname: %s, ID: %s, Alive: %v", action.message, v.user.userName, v.user.nick, user, v.IsAlive()))
			log.Println(fmt.Sprintf("InGame: %v, InDiscussion: %v", inGame, inDiscussion))
			log.Println(action)
			if action.move {
				moveErr := guildMemberMove(dg, guild.ID, user, &action.targetChannel.channelID)
				if moveErr != nil {
					log.Println(moveErr)
					continue
				}
			}

			if v.voiceState.Mute != action.mute {
				err := guildMemberMute(dg, guild.ID, user, action.mute)
				if err != nil {
					log.Println(err)
				}
			} else {
				if action.mute {
					log.Printf("Not muting %s because they're already muted\n", v.user.userName)
				} else {
					log.Printf("Not unmuting %s because they're already unmuted\n", v.user.userName)
				}
			}

			//log.Printf("Sleeping for %dms between actions to avoid being rate-limited by Discord\n", guild.delays.DiscordMuteDelayOffsetMs)
			//time.Sleep(time.Duration(guild.delays.DiscordMuteDelayOffsetMs) * time.Millisecond)
		}
	}
	log.Println("Reached end of handleTrackedMembers")
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

//voiceStateChange is responsible for detecting how players need to be muted/unmuted according to the game state,
//and the most recent voice status data (by calling updateVoiceStatusCache)
func (guild *GuildState) voiceStateChange(s *discordgo.Session, m *discordgo.VoiceStateUpdate) {

	guild.updateVoiceStatusCache(s)

	g, err := s.State.Guild(guild.ID)
	if err != nil {
		log.Println(err)
	}

	//if the user is already in the voice status cache, only update if we don't know the voice channel to track,
	//or the user has ENTERED this voice channel
	guild.UserDataLock.Lock()
	if v, ok := guild.UserData[m.UserID]; ok {
		v.voiceState = *m.VoiceState

		//only track if we have no tracked channel so far, or the user is in the tracked channel
		v.tracking = guild.isTracked(g.VoiceStates, m.UserID, m.ChannelID, v.auData)

		guild.UserData[m.UserID] = v
		log.Printf("Saw a cached \"%s\" user's voice status change, tracking: %v\n", v.user.userName, v.tracking)

		//unmute the member if they left the chat while muted
		if !v.tracking && m.Mute {
			log.Println("Untracked unmute of a muted player")
			err := guildMemberMute(s, m.GuildID, m.UserID, false)
			if err != nil {
				log.Println(err)
			}

			//if the user rejoins, only mute if the game is going, or if it's discussion and they're dead
		} else {
			if v.tracking && !m.Mute && (guild.GamePhase == game.TASKS || (guild.GamePhase == game.DISCUSS && !v.IsAlive())) {
				log.Println("Tracked mute of an unmuted player")
				log.Printf("Current game state: %d, alive: %v", guild.GamePhase, v.IsAlive())
				err := guildMemberMute(s, m.GuildID, m.UserID, true)
				if err != nil {
					log.Println(err)
				}
			}
		}
	} else {
		user := User{
			nick:          "",
			userID:        m.UserID,
			userName:      "",
			discriminator: "",
		}
		//only track if we have no tracked channel so far, or the user is in the tracked channel. Otherwise, don't track
		tracking := guild.isTracked(g.VoiceStates, m.UserID, m.ChannelID, nil)
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

//updateVoiceStatusCache updates the local records about users in voice channels (or if they're NOT). So if a
func (guild *GuildState) updateVoiceStatusCache(s *discordgo.Session) {
	g, err := s.State.Guild(guild.ID)
	if err != nil {
		log.Println(err)
	}

	updatedAnyStatuses := false

	//make sure all the people in the voice status cache are still in voice
	guild.UserDataLock.Lock()
	for id, v := range guild.UserData {
		if v.tracking {
			foundUser := false
			for _, state := range g.VoiceStates {
				if state.UserID == id {
					foundUser = true
					break
				}
			}
			//TODO can you server unmute someone not in voice? Prob not...
			if !foundUser {
				log.Printf("Untracking %s because they are now disconnected from the voice channel", v.user.userName)
				v.tracking = false
				guild.UserData[id] = v
				updatedAnyStatuses = true
			}
		}
	}
	guild.UserDataLock.Unlock()

	for _, state := range g.VoiceStates {
		guild.UserDataLock.Lock()
		//update the voicestatus of the user
		if v, ok := guild.UserData[state.UserID]; ok {
			v.voiceState = *state
			updated := guild.updateTrackedStatus(g.VoiceStates, state.UserID, state.ChannelID)
			if updated {
				updatedAnyStatuses = true
			}

		} else { //add the user we haven't seen in our cache before
			user := User{
				userID: state.UserID,
			}

			guild.UserData[state.UserID] = UserData{
				user:       user,
				voiceState: *state,
				tracking:   guild.isTracked(g.VoiceStates, state.ChannelID, state.UserID, v.auData),
				auData:     nil,
			}
			updatedAnyStatuses = true
		}
		guild.UserDataLock.Unlock()
	}

	//only update the larger status message if a user changed tracking status
	if updatedAnyStatuses {
		guild.handleGameStateMessage(s)
	}

}

// TODO this probably deals with too much direct state-changing;
//probably want to bubble it up to some higher authority?
func (guild *GuildState) handleReactionGameStartAdd(s *discordgo.Session, m *discordgo.MessageReactionAdd) {
	g, err := s.State.Guild(guild.ID)
	if err != nil {
		log.Println(err)
	}

	if guild.GameStateMessage != nil {

		//verify that the user is reacting to the state/status message
		if IsUserReactionToStateMsg(m, guild.GameStateMessage) {
			for color, e := range AlivenessColoredEmojis[true] {
				if e.ID == m.Emoji.ID {
					log.Printf("Player %s reacted with color %s", m.UserID, GetColorStringForInt(color))

					//pair up the discord user with the relevant in-game data, matching by the color
					_, matched := guild.matchByColor(m.UserID, GetColorStringForInt(color), guild.AmongUsData)

					//make sure the player's "tracked" status is updated when applying ANY emoji, valid or not
					guild.updateTrackedStatus(g.VoiceStates, m.UserID, "")

					//then remove the player's reaction if we matched, or if we didn't
					err = s.MessageReactionRemove(m.ChannelID, m.MessageID, e.FormatForReaction(), m.UserID)
					if err != nil {
						log.Println(err)
					}

					if matched {
						guild.handleGameStateMessage(s)

						//Don't remove the bot's reaction; more likely to misclick when the emojis move, and it doesn't
						//allow users to change their color pairing if they messed up

						//remove the bot's reaction
						//err := s.MessageReactionRemove(m.ChannelID, m.MessageID, e.FormatForReaction(), guild.GameStateMessage.Author.ID)
						//if err != nil {
						//	log.Println(err)
						//}
					}
				}
			}
		}
	}
}

func IsUserReactionToStateMsg(m *discordgo.MessageReactionAdd, sm *discordgo.Message) bool {
	return m.ChannelID == sm.ChannelID && m.MessageID == sm.ID && m.UserID != sm.Author.ID
}

func (guild *GuildState) handleReactionsGameStartRemoveAll(s *discordgo.Session) {
	if guild.GameStateMessage != nil {
		removeAllReactions(s, guild.GameStateMessage.ChannelID, guild.GameStateMessage.ID)
	}
}

func (guild *GuildState) updateTrackedStatus(voiceStates []*discordgo.VoiceState, userID, channelID string) bool {
	if v, ok := guild.UserData[userID]; ok {
		shouldTrack := guild.isTracked(voiceStates, userID, channelID, v.auData)
		if shouldTrack != v.tracking {
			v.tracking = shouldTrack
			guild.UserData[userID] = v
			return true
		}
	}
	return false
}

func (guild *GuildState) isTracked(voiceStates []*discordgo.VoiceState, userID, channelID string, auData *AmongUserData) bool {

	foundUserInVoice := false
	for _, f := range voiceStates {
		if f.UserID == userID {
			foundUserInVoice = true
			break
		}
	}
	if !foundUserInVoice {
		return false
	}

	//if the player isn't linked to in-game data
	if auData == nil {
		return false
	}

	//not tracking, or we weren't provided a channel to explicitly check
	if len(guild.Tracking) == 0 || channelID == "" {
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

//first bool is whether the update is truly an update, 2nd bool is if the update is "sensitive" (leaks info to players)
func (guild *GuildState) updateCachedAmongUsData(update game.Player) (bool, bool) {
	guild.AmongUsDataLock.Lock()
	defer guild.AmongUsDataLock.Unlock()

	if _, ok := guild.AmongUsData[update.Name]; !ok {
		guild.AmongUsData[update.Name] = &AmongUserData{
			Color:   update.Color,
			Name:    update.Name,
			IsAlive: !update.IsDead,
		}
		log.Printf("Added new player instance for %s\n", update.Name)
		return true, false
	}
	guildDataTempPtr := guild.AmongUsData[update.Name]
	isUpdate := guildDataTempPtr.isDifferent(update)
	isAliveUpdate := (*guild.AmongUsData[update.Name]).IsAlive != !update.IsDead
	if isUpdate {
		(*guild.AmongUsData[update.Name]).Color = update.Color
		(*guild.AmongUsData[update.Name]).Name = update.Name
		(*guild.AmongUsData[update.Name]).IsAlive = !update.IsDead

		log.Printf("Updated %s", (*guild.AmongUsData[update.Name]).ToString())
	}

	return isUpdate, isAliveUpdate
}

func (guild *GuildState) modifyCachedAmongUsDataAlive(alive bool) {
	for i := range guild.AmongUsData {
		(*guild.AmongUsData[i]).IsAlive = alive
	}
}

// ToString returns a simple string representation of the current state of the guild
func (guild *GuildState) ToString() string {
	return fmt.Sprintf("%v", guild)
}

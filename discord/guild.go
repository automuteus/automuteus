package discord

import (
	"fmt"
	"log"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
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
	Delays           GameDelays
	StatusEmojis     AlivenessEmojis
	SpecialEmojis    map[string]Emoji
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

//this is thread-safe
func (guild *GuildState) updateUserInMap(userID string, userdata UserData) {
	guild.UserDataLock.Lock()
	guild.UserData[userID] = userdata
	guild.UserDataLock.Unlock()
}

//this is thread-safe
func (guild *GuildState) addUserToMap(userID string) {
	user := UserData{
		user: User{
			nick:          "",
			userID:        userID,
			userName:      "",
			discriminator: "",
		},
		pendingVoiceUpdate: false,
		auData:             nil,
	}
	guild.UserDataLock.Lock()
	guild.UserData[userID] = user
	guild.UserDataLock.Unlock()
}

func (guild *GuildState) addFullUserToMap(g *discordgo.Guild, userID string) {
	for _, v := range g.Members {
		if v.User.ID == userID {
			userdata := UserData{
				user: User{
					nick:          "",
					userID:        v.User.ID,
					userName:      v.User.Username,
					discriminator: v.User.Discriminator,
				},
				pendingVoiceUpdate: false,
				auData:             nil,
			}
			guild.UserDataLock.Lock()
			guild.UserData[userID] = userdata
			guild.UserDataLock.Unlock()
			return
		}
	}
	guild.addUserToMap(userID)
}

//handleTrackedMembers moves/mutes players according to the current game state
func (guild *GuildState) handleTrackedMembers(dg *discordgo.Session) {

	g := guild.verifyVoiceStateChanges(dg)

	updateMade := false
	for _, voiceState := range g.VoiceStates {

		guild.UserDataLock.RLock()
		if userData, ok := guild.UserData[voiceState.UserID]; ok {
			shouldMute, shouldDeaf := getVoiceStateChanges(guild, userData, voiceState.ChannelID)

			//only issue a change if the user isn't in the right state already
			if shouldMute != voiceState.Mute || shouldDeaf != voiceState.Deaf {

				//only issue the req to discord if we're not waiting on another one
				if !userData.pendingVoiceUpdate {
					guild.UserDataLock.RUnlock()
					//wait until it goes through
					userData.pendingVoiceUpdate = true

					go guild.updateUserInMap(voiceState.UserID, userData)

					go guildMemberMuteAndDeafen(dg, guild.ID, voiceState.UserID, shouldMute, shouldDeaf)
					updateMade = true
					guild.UserDataLock.RLock()
				}

			} else {
				if shouldMute {
					log.Printf("Not muting %s because they're already muted\n", userData.user.userName)
				} else {
					log.Printf("Not unmuting %s because they're already unmuted\n", userData.user.userName)
				}
			}
		} else { //the user doesn't exist in our userdata cache; add them
			guild.UserDataLock.RUnlock()

			guild.addFullUserToMap(g, voiceState.UserID)

			guild.UserDataLock.RLock()

		}
		guild.UserDataLock.RUnlock()
	}
	if updateMade {
		log.Println("Updating state message")
		guild.handleGameStateMessage(dg)
	}
}

func (guild *GuildState) verifyVoiceStateChanges(s *discordgo.Session) *discordgo.Guild {
	g, err := s.State.Guild(guild.ID)
	if err != nil {
		log.Println(err)
	}

	for _, voiceState := range g.VoiceStates {
		guild.UserDataLock.RLock()
		if userData, ok := guild.UserData[voiceState.UserID]; ok {
			mute, deaf := getVoiceStateChanges(guild, userData, voiceState.ChannelID)
			if userData.pendingVoiceUpdate && voiceState.Mute == mute && voiceState.Deaf == deaf {
				userData.pendingVoiceUpdate = false

				guild.UserDataLock.RUnlock()
				//this one prob doesn't gain anything by being in a goroutine
				guild.updateUserInMap(voiceState.UserID, userData)
				guild.UserDataLock.RLock()

				log.Println("Successfully updated pendingVoice")
			}
		} else { //the user doesn't exist in our userdata cache; add them
			guild.UserDataLock.RUnlock()
			guild.addFullUserToMap(g, voiceState.UserID)
			guild.UserDataLock.RLock()
		}

		guild.UserDataLock.RUnlock()
	}
	return g

}

//voiceStateChange handles more edge-case behavior for users moving between voice channels, and catches when
//relevant discord api requests are fully applied successfully. Otherwise, we can issue multiple requests for
//the same mute/unmute, erroneously
func (guild *GuildState) voiceStateChange(s *discordgo.Session, m *discordgo.VoiceStateUpdate) {
	g := guild.verifyVoiceStateChanges(s)

	updateMade := false

	guild.UserDataLock.RLock()
	//fetch the user from our user data cache
	if user, ok := guild.UserData[m.UserID]; ok {

		shouldMute, shouldDeaf := getVoiceStateChanges(guild, user, m.ChannelID)
		if !user.pendingVoiceUpdate && (shouldMute != m.Mute || shouldDeaf != m.Deaf) {
			guild.UserDataLock.RUnlock()
			user.pendingVoiceUpdate = true

			go guild.updateUserInMap(m.UserID, user)

			go guildMemberMuteAndDeafen(s, m.GuildID, m.UserID, shouldMute, shouldDeaf)

			log.Println("Applied deaf/undeaf mute/unmute via voiceStateChange")

			updateMade = true
			guild.UserDataLock.RLock()
		}
	} else { //the user doesn't exist in our userdata cache; add them
		guild.UserDataLock.RUnlock()
		guild.addFullUserToMap(g, m.UserID)
		guild.UserDataLock.RLock()
	}
	guild.UserDataLock.RUnlock()

	if updateMade {
		log.Println("Updating state message")
		guild.handleGameStateMessage(s)
	}
}

// TODO this probably deals with too much direct state-changing;
//probably want to bubble it up to some higher authority?
func (guild *GuildState) handleReactionGameStartAdd(s *discordgo.Session, m *discordgo.MessageReactionAdd) {
	//g, err := s.State.Guild(guild.ID)
	//if err != nil {
	//	log.Println(err)
	//}

	if guild.GameStateMessage != nil {

		//verify that the user is reacting to the state/status message
		if IsUserReactionToStateMsg(m, guild.GameStateMessage) {
			idMatched := false
			for color, e := range guild.StatusEmojis[true] {
				if e.ID == m.Emoji.ID {
					idMatched = true
					log.Printf("Player %s reacted with color %s", m.UserID, GetColorStringForInt(color))

					//pair up the discord user with the relevant in-game data, matching by the color
					str, _ := guild.matchByColor(m.UserID, GetColorStringForInt(color), guild.AmongUsData)
					log.Println(str)

					//then remove the player's reaction if we matched, or if we didn't
					err := s.MessageReactionRemove(m.ChannelID, m.MessageID, e.FormatForReaction(), m.UserID)
					if err != nil {
						log.Println(err)
					}
					break
				}
			}
			if !idMatched {
				//log.Println(m.Emoji.Name)
				if m.Emoji.Name == "❌" {
					guild.handlePlayerRemove(s, m.UserID)
					err := s.MessageReactionRemove(m.ChannelID, m.MessageID, "❌", m.UserID)
					if err != nil {
						log.Println(err)
					}
					idMatched = true
				}
			}
			//make sure to update any voice changes if they occurred
			if idMatched {
				guild.handleTrackedMembers(s)
				guild.handleGameStateMessage(s)
			}
		}
	}

}

// IsUserReactionToStateMsg func
func IsUserReactionToStateMsg(m *discordgo.MessageReactionAdd, sm *discordgo.Message) bool {
	return m.ChannelID == sm.ChannelID && m.MessageID == sm.ID && m.UserID != sm.Author.ID
}

func (guild *GuildState) handleReactionsGameStartRemoveAll(s *discordgo.Session) {
	if guild.GameStateMessage != nil {
		removeAllReactions(s, guild.GameStateMessage.ChannelID, guild.GameStateMessage.ID)
	}
}

func (guild *GuildState) isTracked(channelID string) bool {

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
	guild.AmongUsDataLock.Lock()
	defer guild.AmongUsDataLock.Unlock()

	for i := range guild.AmongUsData {
		(*guild.AmongUsData[i]).IsAlive = alive
	}
}

// ToString returns a simple string representation of the current state of the guild
func (guild *GuildState) ToString() string {
	return fmt.Sprintf("%v", guild)
}

func (guild *GuildState) clearGameTracking(s *discordgo.Session) {
	guild.UserDataLock.Lock()
	defer guild.UserDataLock.Unlock()

	for i, v := range guild.UserData {
		v.auData = nil
		guild.UserData[i] = v
	}
	//reset all the tracking channels
	guild.Tracking = map[string]Tracking{}
	if guild.GameStateMessage != nil {
		deleteMessage(s, guild.GameStateMessage.ChannelID, guild.GameStateMessage.ID)
	}
	guild.GameStateMessage = nil
	guild.GamePhase = game.LOBBY
}

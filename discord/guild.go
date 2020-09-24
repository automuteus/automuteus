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
	GameStartDelay    int
	GameResumeDelay   int
	DiscussStartDelay int
}

// GuildState struct
type GuildState struct {
	ID            string
	CommandPrefix string
	LinkCode      string

	UserData map[string]game.UserData
	Tracking map[string]Tracking
	//use this to refer to the same state message and update it on ls
	GameStateMessage     *discordgo.Message
	GameStateMessageLock sync.RWMutex

	Delays        GameDelays
	StatusEmojis  AlivenessEmojis
	SpecialEmojis map[string]Emoji
	UserDataLock  sync.RWMutex

	AmongUsData game.AmongUsData

	VoiceRules VoiceRules

	// For voice channel movement
	//MoveDeadPlayers bool

	//if the users should be nick-named using the in-game names
	ApplyNicknames bool
}

// TrackedMemberAction struct
type TrackedMemberAction struct {
	mute          bool
	move          bool
	message       string
	targetChannel Tracking
}

//this is thread-safe
func (guild *GuildState) updateUserInMap(userID string, userdata game.UserData) {
	guild.UserDataLock.Lock()
	guild.UserData[userID] = userdata
	guild.UserDataLock.Unlock()
}

//this is thread-safe
func (guild *GuildState) addUserToMap(userID string) {
	guild.UserDataLock.Lock()
	guild.UserData[userID] = game.MakeMinimalUserData(userID)
	guild.UserDataLock.Unlock()
}

func (guild *GuildState) addFullUserToMap(g *discordgo.Guild, userID string) {
	for _, v := range g.Members {
		if v.User.ID == userID {
			guild.UserDataLock.Lock()
			guild.UserData[userID] = game.MakeUserDataFromDiscordUser(v.User, v.Nick)
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
			tracked := isVoiceChannelTracked(voiceState.ChannelID, guild.Tracking)
			//only actually tracked if we're in a tracked channel AND linked to a player
			tracked = tracked && userData.IsLinked()
			shouldMute, shouldDeaf := guild.VoiceRules.GetVoiceState(userData.IsAlive(), tracked, guild.AmongUsData.GetPhase())

			nick := userData.GetPlayerName()
			if !guild.ApplyNicknames {
				nick = ""
			}

			//only issue a change if the user isn't in the right state already
			//nicksmatch can only be false if the in-game data is != nil, so the reference to .audata below is safe
			if shouldMute != voiceState.Mute || shouldDeaf != voiceState.Deaf || (nick != "" && userData.GetNickName() != userData.GetPlayerName()) {

				//only issue the req to discord if we're not waiting on another one
				if !userData.IsPendingVoiceUpdate() {
					guild.UserDataLock.RUnlock()
					//wait until it goes through
					userData.SetPendingVoiceUpdate(true)

					go guild.updateUserInMap(voiceState.UserID, userData)

					go guildMemberUpdate(dg, guild.ID, voiceState.UserID, UserPatchParameters{shouldMute, shouldDeaf, nick})

					updateMade = true
					guild.UserDataLock.RLock()
				}

			} else {
				if shouldMute {
					log.Printf("Not muting %s because they're already muted\n", userData.GetUserName())
				} else {
					log.Printf("Not unmuting %s because they're already unmuted\n", userData.GetUserName())
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
			tracked := isVoiceChannelTracked(voiceState.ChannelID, guild.Tracking)
			//only actually tracked if we're in a tracked channel AND linked to a player
			tracked = tracked && userData.IsLinked()
			mute, deaf := guild.VoiceRules.GetVoiceState(userData.IsAlive(), tracked, guild.AmongUsData.GetPhase())
			if userData.IsPendingVoiceUpdate() && voiceState.Mute == mute && voiceState.Deaf == deaf {
				userData.SetPendingVoiceUpdate(false)

				guild.UserDataLock.RUnlock()
				//this one prob doesn't gain anything by being in a goroutine
				guild.updateUserInMap(voiceState.UserID, userData)
				guild.UserDataLock.RLock()

				//log.Println("Successfully updated pendingVoice")
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
	//fetch the userData from our userData data cache
	if userData, ok := guild.UserData[m.UserID]; ok {
		tracked := isVoiceChannelTracked(m.ChannelID, guild.Tracking)
		//only actually tracked if we're in a tracked channel AND linked to a player
		tracked = tracked && userData.IsLinked()
		mute, deaf := guild.VoiceRules.GetVoiceState(userData.IsAlive(), tracked, guild.AmongUsData.GetPhase())
		if !userData.IsPendingVoiceUpdate() && (mute != m.Mute || deaf != m.Deaf) {
			guild.UserDataLock.RUnlock()
			userData.SetPendingVoiceUpdate(true)

			go guild.updateUserInMap(m.UserID, userData)
			nick := userData.GetPlayerName()
			if !guild.ApplyNicknames {
				nick = ""
			}

			go guildMemberUpdate(s, m.GuildID, m.UserID, UserPatchParameters{mute, deaf, nick})

			log.Println("Applied deaf/undeaf mute/unmute via voiceStateChange")

			updateMade = true
			guild.UserDataLock.RLock()
		}
	} else { //the userData doesn't exist in our userdata cache; add them
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
					log.Printf("Player %s reacted with color %s", m.UserID, game.GetColorStringForInt(color))

					playerData := guild.AmongUsData.GetByColor(game.GetColorStringForInt(color))
					if playerData != nil {
						if v, ok := guild.UserData[m.UserID]; ok {
							v.SetPlayerData(playerData)
							guild.UserData[m.UserID] = v
						}
					}

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

// ToString returns a simple string representation of the current state of the guild
func (guild *GuildState) ToString() string {
	return fmt.Sprintf("%v", guild)
}

func (guild *GuildState) clearGameTracking(s *discordgo.Session) {
	guild.UserDataLock.Lock()
	defer guild.UserDataLock.Unlock()

	for i, v := range guild.UserData {
		v.SetPlayerData(nil)
		guild.UserData[i] = v
	}
	//reset all the tracking channels
	guild.Tracking = map[string]Tracking{}
	if guild.GameStateMessage != nil {
		deleteMessage(s, guild.GameStateMessage.ChannelID, guild.GameStateMessage.ID)
	}
	guild.GameStateMessage = nil
	guild.AmongUsData.SetPhase(game.LOBBY)
}

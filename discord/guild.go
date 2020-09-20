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

	g, err := dg.State.Guild(guild.ID)
	if err != nil {
		log.Println(err)
	}

	for _, voiceState := range g.VoiceStates {
		guild.UserDataLock.Lock()
		if userData, ok := guild.UserData[voiceState.UserID]; ok {

			//if the user isn't linked to any in-game data, then skip them
			if userData.auData == nil {
				continue
			}
			action := actions[inGame][inDiscussion][userData.IsAlive()]

			if action.move {
				//TODO use the pendingUpdate here, too
				moveErr := guildMemberMove(dg, guild.ID, voiceState.UserID, &action.targetChannel.channelID)
				if moveErr != nil {
					log.Println(moveErr)
					continue
				}
			}

			//only issue a mute if the user isn't muted already
			if action.mute != voiceState.Mute {
				//only apply actions to users in a tracked channel (edge case behavior is for the "update" handler)
				if guild.isTracked(voiceState.ChannelID) {
					//only issue the req to discord if we're not waiting on another one
					if !userData.pendingVoiceUpdate {
						err := guildMemberMute(dg, guild.ID, voiceState.UserID, action.mute)
						if err != nil {
							log.Println(err)
						}
						//now wait until it goes through
						userData.pendingVoiceUpdate = true
						guild.UserData[voiceState.UserID] = userData
					}
				}
			} else {
				if action.mute {
					log.Printf("Not muting %s because they're already muted\n", userData.user.userName)
				} else {
					log.Printf("Not unmuting %s because they're already unmuted\n", userData.user.userName)
				}
			}
		}
		guild.UserDataLock.Unlock()
	}
}

//voiceStateChange handles more edge-case behavior for users moving between voice channels, and catches when
//relevant discord api requests are fully applied successfully. Otherwise, we can issue multiple requests for
//the same mute/unmute erroneously
func (guild *GuildState) voiceStateChange(s *discordgo.Session, m *discordgo.VoiceStateUpdate) {
	guild.UserDataLock.Lock()
	//fetch the user from our user data cache
	if user, ok := guild.UserData[m.UserID]; ok {
		//if the channel isn't tracked
		if !guild.isTracked(m.ChannelID) {
			//but the user is still muted, then they should be unmuted
			if m.Mute {
				if !user.pendingVoiceUpdate {
					err := guildMemberMute(s, m.GuildID, m.UserID, false)
					if err != nil {
						log.Println(err)
					}
					user.pendingVoiceUpdate = true
					guild.UserData[m.UserID] = user
				}
			} else { //the expected result occurred; the player is unmuted in the non-tracked channel
				user.pendingVoiceUpdate = false
				guild.UserData[m.UserID] = user
			}
		} else { //the channel IS tracked
			mute := shouldBeMuted(guild.GamePhase, user.IsAlive())
			if mute != m.Mute {
				if !user.pendingVoiceUpdate {
					err := guildMemberMute(s, m.GuildID, m.UserID, mute)
					if err != nil {
						log.Println(err)
					}
					user.pendingVoiceUpdate = true
					guild.UserData[m.UserID] = user
				}
			} else {
				user.pendingVoiceUpdate = false
				guild.UserData[m.UserID] = user
			}
		}
	}
	guild.UserDataLock.Unlock()
}

func shouldBeMuted(phase game.Phase, isAlive bool) bool {
	switch phase {
	case game.LOBBY:
		return false
	case game.DISCUSS:
		return !isAlive
	case game.TASKS:
		return true
	default:
		return false
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

						//NOTE: Don't remove the bot's reaction; more likely to misclick when the emojis move, and it doesn't
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

func (guild *GuildState) updateTrackedStatus(voiceStates []*discordgo.VoiceState, userID, channelID string) {
	if v, ok := guild.UserData[userID]; ok {
		inVoice := isUserInVoice(userID, voiceStates)
		if !inVoice {
			v.tracking = false
			guild.UserData[userID] = v
			return
		}

		shouldTrack := guild.isTracked(channelID)
		if shouldTrack != v.tracking {
			v.tracking = shouldTrack
			guild.UserData[userID] = v
		}
	}
}

func isUserInVoice(userID string, voiceStates []*discordgo.VoiceState) bool {
	for _, f := range voiceStates {
		if f.UserID == userID {
			return true
		}
	}
	return false
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
	for i := range guild.AmongUsData {
		(*guild.AmongUsData[i]).IsAlive = alive
	}
}

// ToString returns a simple string representation of the current state of the guild
func (guild *GuildState) ToString() string {
	return fmt.Sprintf("%v", guild)
}

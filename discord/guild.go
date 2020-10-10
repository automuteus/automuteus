package discord

import (
	"container/heap"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"log"
	"sync"
	"time"
)

// GameDelays struct
type GameDelays struct {
	//maps from origin->new phases, with the integer number of seconds for the delay
	Delays map[game.PhaseNameString]map[game.PhaseNameString]int `json:"delays"`
}

func MakeDefaultDelays() GameDelays {
	return GameDelays{
		Delays: map[game.PhaseNameString]map[game.PhaseNameString]int{
			game.PhaseNames[game.LOBBY]: {
				game.PhaseNames[game.LOBBY]:   0,
				game.PhaseNames[game.TASKS]:   7,
				game.PhaseNames[game.DISCUSS]: 0,
			},
			game.PhaseNames[game.TASKS]: {
				game.PhaseNames[game.LOBBY]:   1,
				game.PhaseNames[game.TASKS]:   0,
				game.PhaseNames[game.DISCUSS]: 0,
			},
			game.PhaseNames[game.DISCUSS]: {
				game.PhaseNames[game.LOBBY]:   6,
				game.PhaseNames[game.TASKS]:   7,
				game.PhaseNames[game.DISCUSS]: 0,
			},
		},
	}
}

func (gd *GameDelays) GetDelay(origin, dest game.Phase) int {
	return gd.Delays[game.PhaseNames[origin]][game.PhaseNames[dest]]
}

// GuildState struct
type GuildState struct {
	PersistentGuildData *PersistentGuildData

	Linked bool

	UserData UserDataSet
	Tracking Tracking

	GameStateMsg GameStateMessage

	StatusEmojis  AlivenessEmojis
	SpecialEmojis map[string]Emoji

	AmongUsData game.AmongUsData
	GameRunning bool
}

type EmojiCollection struct {
	statusEmojis  AlivenessEmojis
	specialEmojis map[string]Emoji
	lock          sync.RWMutex
}

// TrackedMemberAction struct
type TrackedMemberAction struct {
	mute          bool
	move          bool
	message       string
	targetChannel Tracking
}

func (guild *GuildState) checkCacheAndAddUser(g *discordgo.Guild, s *discordgo.Session, userID string) (game.UserData, bool) {
	if g == nil {
		return game.UserData{}, false
	}
	//check and see if they're cached first
	for _, v := range g.Members {
		if v.User.ID == userID {
			user := game.MakeUserDataFromDiscordUser(v.User, v.Nick)
			guild.UserData.AddFullUser(user)
			return user, true
		}
	}
	mem, err := s.GuildMember(guild.PersistentGuildData.GuildID, userID)
	if err != nil {
		log.Println(err)
		return game.UserData{}, false
	}
	user := game.MakeUserDataFromDiscordUser(mem.User, mem.Nick)
	guild.UserData.AddFullUser(user)
	return user, true
}

type HandlePriority int

const (
	NoPriority    HandlePriority = 0
	AlivePriority HandlePriority = 1
	DeadPriority  HandlePriority = 2
)

type PrioritizedPatchParams struct {
	priority    int
	patchParams UserPatchParameters
}

type PatchPriority []PrioritizedPatchParams

func (h PatchPriority) Len() int { return len(h) }

//NOTE this is inversed so HIGHER numbers are pulled FIRST
func (h PatchPriority) Less(i, j int) bool { return h[i].priority > h[j].priority }
func (h PatchPriority) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *PatchPriority) Push(x interface{}) {
	// Push and Pop use pointer receivers because they modify the slice's length,
	// not just its contents.
	*h = append(*h, x.(PrioritizedPatchParams))
}

func (h *PatchPriority) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

//handleTrackedMembers moves/mutes players according to the current game state
func (guild *GuildState) handleTrackedMembers(sm *SessionManager, delay int, handlePriority HandlePriority) {

	g := guild.verifyVoiceStateChanges(sm.GetPrimarySession())

	if g == nil {
		return
	}

	priorityQueue := &PatchPriority{}
	heap.Init(priorityQueue)

	for _, voiceState := range g.VoiceStates {

		userData, err := guild.UserData.GetUser(voiceState.UserID)
		if err != nil {
			//the user doesn't exist in our userdata cache; add them
			added := false
			userData, added = guild.checkCacheAndAddUser(g, sm.GetPrimarySession(), voiceState.UserID)
			if !added {
				continue
			}
		}

		tracked := guild.Tracking.IsTracked(voiceState.ChannelID)
		//only actually tracked if we're in a tracked channel AND linked to a player
		tracked = tracked && userData.IsLinked()
		shouldMute, shouldDeaf := guild.PersistentGuildData.VoiceRules.GetVoiceState(userData.IsAlive(), tracked, guild.AmongUsData.GetPhase())

		nick := userData.GetPlayerName()
		if !guild.PersistentGuildData.ApplyNicknames {
			nick = ""
		}

		//only issue a change if the user isn't in the right state already
		//nicksmatch can only be false if the in-game data is != nil, so the reference to .audata below is safe
		//check the userdata is linked here to not accidentally undeafen music bots, for example
		if userData.IsLinked() && shouldMute != voiceState.Mute || shouldDeaf != voiceState.Deaf || (nick != "" && userData.GetNickName() != userData.GetPlayerName()) {

			//only issue the req to discord if we're not waiting on another one
			if !userData.IsPendingVoiceUpdate() {
				priority := 0

				if handlePriority != NoPriority {
					if handlePriority == AlivePriority && userData.IsAlive() {
						priority++
					} else if handlePriority == DeadPriority && !userData.IsAlive() {
						priority++
					}
				}

				params := UserPatchParameters{guild.PersistentGuildData.GuildID, userData, shouldDeaf, shouldMute, nick}

				heap.Push(priorityQueue, PrioritizedPatchParams{
					priority:    priority,
					patchParams: params,
				})
			}

		} else if userData.IsLinked() {
			if shouldMute {
				log.Printf("Not muting %s because they're already muted\n", userData.GetUserName())
			} else {
				log.Printf("Not unmuting %s because they're already unmuted\n", userData.GetUserName())
			}
		}
	}
	wg := sync.WaitGroup{}
	waitForHigherPriority := false

	if delay > 0 {
		log.Printf("Sleeping for %d seconds before applying changes to users\n", delay)
		time.Sleep(time.Second * time.Duration(delay))
	}

	for priorityQueue.Len() > 0 {
		p := heap.Pop(priorityQueue).(PrioritizedPatchParams)

		if p.priority > 0 {
			waitForHigherPriority = true
			log.Printf("User %s has higher priority: %d\n", p.patchParams.Userdata.GetID(), p.priority)
		} else if waitForHigherPriority {
			//wait for all the other users to get muted/unmuted completely, first
			//log.Println("Waiting for high priority user changes first")
			wg.Wait()
			waitForHigherPriority = false
		}

		wg.Add(1)

		//wait until it goes through
		p.patchParams.Userdata.SetPendingVoiceUpdate(true)

		guild.UserData.UpdateUserData(p.patchParams.Userdata.GetID(), p.patchParams.Userdata)

		//we can issue mutes/deafens from ANY session, not just the primary
		go muteWorker(sm.GetSessionForRequest(), &wg, p.patchParams)
	}
	wg.Wait()

	return
}

func muteWorker(s *discordgo.Session, wg *sync.WaitGroup, parameters UserPatchParameters) {
	guildMemberUpdate(s, parameters)
	wg.Done()
}

func (guild *GuildState) verifyVoiceStateChanges(s *discordgo.Session) *discordgo.Guild {
	g, err := s.State.Guild(guild.PersistentGuildData.GuildID)
	if err != nil {
		log.Println(err)
		return nil
	}

	for _, voiceState := range g.VoiceStates {
		userData, err := guild.UserData.GetUser(voiceState.UserID)

		if err != nil {
			//the user doesn't exist in our userdata cache; add them
			added := false
			userData, added = guild.checkCacheAndAddUser(g, s, voiceState.UserID)
			if !added {
				continue
			}
		}

		tracked := guild.Tracking.IsTracked(voiceState.ChannelID)
		//only actually tracked if we're in a tracked channel AND linked to a player
		tracked = tracked && userData.IsLinked()
		mute, deaf := guild.PersistentGuildData.VoiceRules.GetVoiceState(userData.IsAlive(), tracked, guild.AmongUsData.GetPhase())
		if userData.IsPendingVoiceUpdate() && voiceState.Mute == mute && voiceState.Deaf == deaf {
			userData.SetPendingVoiceUpdate(false)

			guild.UserData.UpdateUserData(voiceState.UserID, userData)

			//log.Println("Successfully updated pendingVoice")
		}

	}
	return g

}

//voiceStateChange handles more edge-case behavior for users moving between voice channels, and catches when
//relevant discord api requests are fully applied successfully. Otherwise, we can issue multiple requests for
//the same mute/unmute, erroneously
func (guild *GuildState) voiceStateChange(s *discordgo.Session, m *discordgo.VoiceStateUpdate) {
	g := guild.verifyVoiceStateChanges(s)

	if g == nil {
		return
	}

	updateMade := false

	//fetch the userData from our userData data cache
	userData, err := guild.UserData.GetUser(m.UserID)
	if err != nil {
		//the user doesn't exist in our userdata cache; add them
		userData, _ = guild.checkCacheAndAddUser(g, s, m.UserID)
	}
	tracked := guild.Tracking.IsTracked(m.ChannelID)
	//only actually tracked if we're in a tracked channel AND linked to a player
	tracked = tracked && userData.IsLinked()
	mute, deaf := guild.PersistentGuildData.VoiceRules.GetVoiceState(userData.IsAlive(), tracked, guild.AmongUsData.GetPhase())
	//check the userdata is linked here to not accidentally undeafen music bots, for example
	if userData.IsLinked() && !userData.IsPendingVoiceUpdate() && (mute != m.Mute || deaf != m.Deaf) {
		userData.SetPendingVoiceUpdate(true)

		guild.UserData.UpdateUserData(m.UserID, userData)

		nick := userData.GetPlayerName()
		if !guild.PersistentGuildData.ApplyNicknames {
			nick = ""
		}

		go guildMemberUpdate(s, UserPatchParameters{m.GuildID, userData, deaf, mute, nick})

		//log.Println("Applied deaf/undeaf mute/unmute via voiceStateChange")

		updateMade = true
	}

	if updateMade {
		//log.Println("Updating state message")
		guild.GameStateMsg.Edit(s, gameStateResponse(guild))
	}
}

func (bot *Bot) handleReactionGameStartAdd(guild *GuildState, s *discordgo.Session, m *discordgo.MessageReactionAdd) {
	g, err := s.State.Guild(guild.PersistentGuildData.GuildID)
	if err != nil {
		log.Println(err)
		return
	}

	if guild.GameStateMsg.Exists() {

		//verify that the user is reacting to the state/status message
		if guild.GameStateMsg.IsReactionTo(m) {
			idMatched := false
			for color, e := range guild.StatusEmojis[true] {
				if e.ID == m.Emoji.ID {
					idMatched = true
					log.Printf("Player %s reacted with color %s", m.UserID, game.GetColorStringForInt(color))
					//the user doesn't exist in our userdata cache; add them

					_, added := guild.checkCacheAndAddUser(g, s, m.UserID)
					if !added {
						log.Println("No users found in Discord for userID " + m.UserID)
					}

					playerData := guild.AmongUsData.GetByColor(game.GetColorStringForInt(color))
					if playerData != nil {
						guild.UserData.UpdatePlayerData(m.UserID, playerData)
					} else {
						log.Println("I couldn't find any player data for that color; is your capture linked?")
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
					log.Printf("Removing player %s", m.UserID)
					guild.UserData.ClearPlayerData(m.UserID)
					err := s.MessageReactionRemove(m.ChannelID, m.MessageID, "❌", m.UserID)
					if err != nil {
						log.Println(err)
					}
					idMatched = true
				}
			}
			//make sure to update any voice changes if they occurred
			if idMatched {
				guild.handleTrackedMembers(&bot.SessionManager, 0, NoPriority)
				guild.GameStateMsg.Edit(s, gameStateResponse(guild))
			}
		}
	}
}

func (guild *GuildState) HasAdminPermissions(userID string) bool {
	if len(guild.PersistentGuildData.AdminUserIDs) == 0 {
		return false
	}

	for _, v := range guild.PersistentGuildData.AdminUserIDs {
		if v == userID {
			return true
		}
	}
	return false
}

func (guild *GuildState) HasRolePermissions(s *discordgo.Session, userID string) bool {
	if len(guild.PersistentGuildData.PermissionedRoleIDs) == 0 {
		return false
	}

	mem, err := s.GuildMember(guild.PersistentGuildData.GuildID, userID)
	if err != nil {
		log.Println(err)
	}
	for _, role := range mem.Roles {
		for _, testRole := range guild.PersistentGuildData.PermissionedRoleIDs {
			if testRole == role {
				return true
			}
		}
	}
	return false
}

// ToString returns a simple string representation of the current state of the guild
func (guild *GuildState) ToString() string {
	return fmt.Sprintf("%v", guild)
}

func (guild *GuildState) clearGameTracking(s *discordgo.Session) {
	//clear the discord user links to underlying player data
	guild.UserData.ClearAllPlayerData()

	//clears the base-level player data in memory
	guild.AmongUsData.ClearAllPlayerData()

	//reset all the tracking channels
	guild.Tracking.Reset()

	guild.GameStateMsg.Delete(s)
}

package discord

import (
	"container/heap"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log"
	"sync"
	"time"
)

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

func (guild *GuildState) verifyVoiceStateChanges(s *discordgo.Session) *discordgo.Guild {
	g, err := s.State.Guild(guild.guildSettings.GuildID)
	if err != nil {
		guild.Logln(err.Error())
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
		mute, deaf := guild.guildSettings.GetVoiceState(userData.IsAlive(), tracked, guild.AmongUsData.GetPhase())

		//still have to check if the player is linked
		//(music bots are untracked so mute/deafen = false, but they dont have playerdata...)
		if userData.IsLinked() && userData.IsPendingVoiceUpdate() && voiceState.Mute == mute && voiceState.Deaf == deaf {
			userData.SetPendingVoiceUpdate(false)

			guild.UserData.UpdateUserData(voiceState.UserID, userData)
		}
	}
	return g
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
		shouldMute, shouldDeaf := guild.guildSettings.GetVoiceState(userData.IsAlive(), tracked, guild.AmongUsData.GetPhase())

		nick := userData.GetPlayerName()
		if !guild.guildSettings.GetApplyNicknames() {
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

				params := UserPatchParameters{guild.guildSettings.GuildID, userData, shouldDeaf, shouldMute, nick}

				heap.Push(priorityQueue, PrioritizedPatchParams{
					priority:    priority,
					patchParams: params,
				})
			}

		} else if userData.IsLinked() {
			if shouldMute {
				guild.Log(fmt.Sprintf("Not muting %s because they're already muted\n", userData.GetUserName()))
			} else {
				guild.Log(fmt.Sprintf("Not unmuting %s because they're already unmuted\n", userData.GetUserName()))
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
			guild.Log(fmt.Sprintf("User %s has higher priority: %d\n", p.patchParams.Userdata.GetID(), p.priority))
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
		go muteWorker(sm.GetSessionForRequest(p.patchParams.GuildID), &wg, p.patchParams)
	}
	wg.Wait()

	return
}

func muteWorker(s *discordgo.Session, wg *sync.WaitGroup, parameters UserPatchParameters) {
	guildMemberUpdate(s, parameters)
	wg.Done()
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
	mute, deaf := guild.guildSettings.GetVoiceState(userData.IsAlive(), tracked, guild.AmongUsData.GetPhase())
	//check the userdata is linked here to not accidentally undeafen music bots, for example
	if userData.IsLinked() && !userData.IsPendingVoiceUpdate() && (mute != m.Mute || deaf != m.Deaf) {
		userData.SetPendingVoiceUpdate(true)

		guild.UserData.UpdateUserData(m.UserID, userData)

		nick := userData.GetPlayerName()
		if !guild.guildSettings.GetApplyNicknames() {
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

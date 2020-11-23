package discord

import (
	"container/heap"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"github.com/denverquane/amongusdiscord/metrics"
	"github.com/denverquane/amongusdiscord/storage"
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

func (bot *Bot) applyToSingle(dgs *DiscordGameState, userID string, mute, deaf bool) {
	log.Println("Forcibly applying mute/deaf to " + userID)
	//userData, _ := dgs.checkCacheAndAddUser(g, bot.PrimarySession.GetPrimarySession(), userID)
	//params := UserPatchParameters{
	//	GuildID:  dgs.GuildID,
	//	Userdata: userData,
	//	Deaf:     deaf,
	//	Mute:     mute,
	//	Nick:     "",
	//}
	bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MuteDeafen, 1)
	err := bot.GalactusClient.ModifyUser(dgs.GuildID, dgs.ConnectCode, userID, mute, deaf)
	if err != nil {
		log.Println(err)
	}
	//go guildMemberUpdate(bot.PrimarySession.GetSessionForRequest(dgs.GuildID), params)
}

func (bot *Bot) applyToAll(dgs *DiscordGameState, mute, deaf bool) {
	g, err := bot.PrimarySession.State.Guild(dgs.GuildID)
	if err != nil {
		log.Println(err)
		return
	}

	for _, voiceState := range g.VoiceStates {
		userData, err := dgs.GetUser(voiceState.UserID)
		if err != nil {
			//the User doesn't exist in our userdata cache; add them
			added := false
			userData, added = dgs.checkCacheAndAddUser(g, bot.PrimarySession, voiceState.UserID)
			if !added {
				continue
			}
		}

		tracked := voiceState.ChannelID != "" && dgs.Tracking.ChannelID == voiceState.ChannelID

		_, linked := dgs.AmongUsData.GetByName(userData.InGameName)
		//only actually tracked if we're in a tracked channel AND linked to a player
		tracked = tracked && linked

		if tracked {
			log.Println("Forcibly applying mute/deaf to " + userData.User.UserID)
			//params := UserPatchParameters{
			//	GuildID:  dgs.GuildID,
			//	Userdata: userData,
			//	Deaf:     deaf,
			//	Mute:     mute,
			//	Nick:     "",
			//}
			bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MuteDeafen, 1)
			err = bot.GalactusClient.ModifyUser(dgs.GuildID, dgs.ConnectCode, userData.User.UserID, mute, deaf)
			if err != nil {
				log.Println(err)
			}
			//go guildMemberUpdate(bot.PrimarySession.GetSessionForRequest(dgs.GuildID), params)
		}
	}
}

//handleTrackedMembers moves/mutes players according to the current game state
func (bot *Bot) handleTrackedMembers(sess *discordgo.Session, sett *storage.GuildSettings, delay int, handlePriority HandlePriority, gsr GameStateRequest) {

	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
	if lock == nil {
		return
	}

	g, err := sess.State.Guild(dgs.GuildID)

	if err != nil || g == nil {
		lock.Release(ctx)
		return
	}

	priorityQueue := &PatchPriority{}
	heap.Init(priorityQueue)

	for _, voiceState := range g.VoiceStates {
		userData, err := dgs.GetUser(voiceState.UserID)
		if err != nil {
			//the User doesn't exist in our userdata cache; add them
			added := false
			userData, added = dgs.checkCacheAndAddUser(g, sess, voiceState.UserID)
			if !added {
				continue
			}
		}

		tracked := voiceState.ChannelID != "" && dgs.Tracking.ChannelID == voiceState.ChannelID

		auData, linked := dgs.AmongUsData.GetByName(userData.InGameName)
		//only actually tracked if we're in a tracked channel AND linked to a player
		tracked = tracked && (linked || userData.GetPlayerName() == game.SpectatorPlayerName)
		shouldMute, shouldDeaf := sett.GetVoiceState(auData.IsAlive, tracked, dgs.AmongUsData.GetPhase())

		//nick := userData.GetPlayerName()
		//if !sett.GetApplyNicknames() {
		//	nick = ""
		//}

		incorrectMuteDeafenState := shouldMute != userData.ShouldBeMute || shouldDeaf != userData.ShouldBeDeaf

		//only issue a change if the User isn't in the right state already
		//nicksmatch can only be false if the in-game data is != nil, so the reference to .audata below is safe
		//check the userdata is linked here to not accidentally undeafen music bots, for example
		if linked && incorrectMuteDeafenState {
			priority := 0

			if handlePriority != NoPriority {
				if handlePriority == AlivePriority && auData.IsAlive {
					priority++
				} else if handlePriority == DeadPriority && !auData.IsAlive {
					priority++
				}
			}

			params := UserPatchParameters{dgs.GuildID, userData, shouldDeaf, shouldMute}

			heap.Push(priorityQueue, PrioritizedPatchParams{
				priority:    priority,
				patchParams: params,
			})
		} else if linked {
			if shouldMute {
				log.Print(fmt.Sprintf("Not muting %s because they're already muted\n", userData.GetUserName()))
			} else {
				log.Print(fmt.Sprintf("Not unmuting %s because they're already unmuted\n", userData.GetUserName()))
			}
		}
	}
	wg := sync.WaitGroup{}
	waitForHigherPriority := false

	//we relinquish the lock while we wait
	bot.RedisInterface.SetDiscordGameState(dgs, lock)

	if delay > 0 {
		log.Printf("Sleeping for %d seconds before applying changes to users\n", delay)
		time.Sleep(time.Second * time.Duration(delay))
	}

	for priorityQueue.Len() > 0 {
		p := heap.Pop(priorityQueue).(PrioritizedPatchParams)

		if p.priority > 0 {
			waitForHigherPriority = true
			log.Print(fmt.Sprintf("User %s has higher priority: %d\n", p.patchParams.Userdata.GetID(), p.priority))
		} else if waitForHigherPriority {
			//wait for all the other users to get muted/unmuted completely, first
			//log.Println("Waiting for high priority User changes first")
			wg.Wait()
			waitForHigherPriority = false
		}

		p.patchParams.Userdata.SetShouldBeMuteDeaf(p.patchParams.Mute, p.patchParams.Deaf)

		//TODO could be a race condition?
		go bot.updateInBackground(gsr, p.patchParams.Userdata.GetID(), p.patchParams.Userdata)

		if dgs.Running {
			wg.Add(1)
			bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MuteDeafen, 1)
			//we can issue mutes/deafens from ANY session, not just the primary
			err = bot.GalactusClient.ModifyUser(dgs.GuildID, dgs.ConnectCode, p.patchParams.Userdata.GetID(), p.patchParams.Mute, p.patchParams.Deaf)
			if err != nil {
				log.Println(err)
			}
			wg.Done()
		}
	}
	wg.Wait()
}

func (bot *Bot) updateInBackground(gsr GameStateRequest, userID string, data UserData) {
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
	for lock == nil {
		lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
	}
	dgs.UpdateUserData(userID, data)
	bot.RedisInterface.SetDiscordGameState(dgs, lock)
}

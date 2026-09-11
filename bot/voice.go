package bot

import (
	"context"
	"github.com/automuteus/automuteus/v8/pkg/lock"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/automuteus/automuteus/v8/pkg/task"
	"github.com/bwmarrin/discordgo"
	"log"
	"strconv"
	"time"
)

type HandlePriority int

const (
	NoPriority    HandlePriority = 0
	AlivePriority HandlePriority = 1
	DeadPriority  HandlePriority = 2
)

func (bot *Bot) applyToSingle(dgs *GameState, premTier premium.Tier, userID string, mute, deaf bool) error {
	uid, _ := strconv.ParseUint(userID, 10, 64)
	req := task.UserModifyRequest{
		Premium: premTier,
		Users: []task.UserModify{
			{
				UserID: uid,
				Mute:   mute,
				Deaf:   deaf,
			},
		},
	}
	// nil lock because this is an override; we don't care about legitimately obtaining the lock
	return bot.voice.ModifyUsers(dgs.GuildID, dgs.ConnectCode, req, nil)
}

func (bot *Bot) applyToAll(dgs *GameState, premTier premium.Tier, mute, deaf bool) error {
	g, err := bot.guilds.Guild(dgs.GuildID)
	if err != nil {
		return err
	}

	var users []task.UserModify

	for _, voiceState := range g.VoiceStates {
		userData, err := dgs.GetUser(voiceState.UserID)
		if err != nil {
			// the User doesn't exist in our userdata cache; add them
			added := false
			userData, added = dgs.checkCacheAndAddUser(g, bot.discord, voiceState.UserID)
			if !added {
				continue
			}
		}

		tracked := voiceState.ChannelID != "" && dgs.VoiceChannel == voiceState.ChannelID

		_, linked := dgs.GameData.GetByName(userData.InGameName)
		// only actually tracked if we're in a tracked channel AND linked to a player
		tracked = tracked && linked

		if tracked {
			uid, _ := strconv.ParseUint(userData.User.UserID, 10, 64)
			users = append(users, task.UserModify{
				UserID: uid,
				Mute:   mute,
				Deaf:   deaf,
			})
			log.Println("Forcibly applying mute/deaf to " + userData.User.UserID)
		}
	}
	if len(users) > 0 {
		req := task.UserModifyRequest{
			Premium: premTier,
			Users:   users,
		}
		// nil lock because this is an override; we don't care about legitimately obtaining the lock
		return bot.voice.ModifyUsers(dgs.GuildID, dgs.ConnectCode, req, nil)
	}
	return nil
}

// computeVoiceChanges decides, for each member currently in voice, whether their mute/deafen state needs to
// change given the game state and guild settings. It returns the changes to issue, and how many entries at the
// front of that list are high-priority per handlePriority (those are sent before the rest).
//
// Users not present in dgs.UserData are skipped; callers must populate the cache first (see handleTrackedMembers).
// For every user that receives a change, dgs.UserData is updated to record the intended state, so calling this
// again with the same inputs yields no further changes.
func computeVoiceChanges(dgs *GameState, sett *settings.GuildSettings, voiceStates []*discordgo.VoiceState, handlePriority HandlePriority) ([]task.UserModify, int) {
	var users []task.UserModify
	priorityRequests := 0
	muteSpectators := sett.GetMuteSpectator()
	phase := dgs.GameData.GetPhase()

	for _, voiceState := range voiceStates {
		userData, err := dgs.GetUser(voiceState.UserID)
		if err != nil {
			continue
		}

		tracked := voiceState.ChannelID != "" && dgs.VoiceChannel == voiceState.ChannelID

		auData, found := dgs.GameData.GetByName(userData.InGameName)
		var isAlive bool
		if !muteSpectators {
			// only actually tracked if we're in the tracked channel AND linked to a player
			tracked = tracked && found
			isAlive = auData.IsAlive
		} else if found {
			isAlive = auData.IsAlive
		} else {
			// an unlinked user in the tracked channel is a spectator; treat them as dead
			isAlive = false
		}
		shouldMute, shouldDeaf := sett.GetVoiceState(isAlive, tracked, phase)

		incorrectMuteDeafenState := shouldMute != userData.ShouldBeMute || shouldDeaf != userData.ShouldBeDeaf

		// only issue a change if the user isn't in the right state already, and only for linked users (or everyone,
		// when muting spectators) so that we don't accidentally undeafen music bots, for example
		if !incorrectMuteDeafenState || !(found || muteSpectators) {
			continue
		}

		uid, _ := strconv.ParseUint(userData.User.UserID, 10, 64)
		userModify := task.UserModify{
			UserID: uid,
			Mute:   shouldMute,
			Deaf:   shouldDeaf,
		}

		if handlePriority != NoPriority && ((handlePriority == AlivePriority && isAlive) || (handlePriority == DeadPriority && !isAlive)) {
			users = append([]task.UserModify{userModify}, users...)
			priorityRequests++ // counter of how many elements on the front of the arr should be sent first
		} else {
			users = append(users, userModify)
		}
		userData.SetShouldBeMuteDeaf(shouldMute, shouldDeaf)
		dgs.UpdateUserData(userData.User.UserID, userData)
	}
	return users, priorityRequests
}

// handleTrackedMembers moves/mutes players according to the current game state
func (bot *Bot) handleTrackedMembers(sett *settings.GuildSettings, premTier premium.Tier, delay int, handlePriority HandlePriority, gsr GameStateRequest) {

	lock, dgs := bot.store.GetDiscordGameStateAndLock(gsr)
	for lock == nil {
		lock, dgs = bot.store.GetDiscordGameStateAndLock(gsr)
	}

	g, err := bot.guilds.Guild(dgs.GuildID)

	if err != nil || g == nil {
		lock.Release(ctx)
		return
	}

	// make sure every member currently in voice is in our user cache before deciding on changes
	for _, voiceState := range g.VoiceStates {
		if _, err := dgs.GetUser(voiceState.UserID); err != nil {
			dgs.checkCacheAndAddUser(g, bot.discord, voiceState.UserID)
		}
	}

	users, priorityRequests := computeVoiceChanges(dgs, sett, g.VoiceStates, handlePriority)

	// we relinquish the lock while we wait
	bot.store.SetDiscordGameState(dgs, lock)

	voiceLock := bot.store.LockVoiceChanges(dgs.ConnectCode, time.Second*time.Duration(delay+1))

	if delay > 0 {
		log.Printf("Sleeping for %d seconds before applying changes to users\n", delay)
		bot.sleep(time.Second * time.Duration(delay))
	}

	if dgs.Running && len(users) > 0 {
		if priorityRequests > 0 {
			req := task.UserModifyRequest{
				Premium: premTier,
				Users:   users[:priorityRequests],
			}
			// no lock; we're not done yet
			err := bot.issueMutesAndRecord(dgs.GuildID, dgs.ConnectCode, req, nil)
			if err != nil {
				log.Println(err)
			} else {
				log.Println("Successfully finished issuing high priority mutes")
			}
			rem := users[priorityRequests:]
			if len(rem) > 0 {
				req = task.UserModifyRequest{
					Premium: premTier,
					Users:   rem,
				}
				err := bot.issueMutesAndRecord(dgs.GuildID, dgs.ConnectCode, req, voiceLock)
				if err != nil {
					log.Println(err)
				}
			} else if voiceLock != nil {
				voiceLock.Release(context.Background())
			}
		} else {
			// no priority; issue all at once
			log.Println("Issuing mutes/deafens with no particular priority")
			req := task.UserModifyRequest{
				Premium: premTier,
				Users:   users,
			}
			err := bot.issueMutesAndRecord(dgs.GuildID, dgs.ConnectCode, req, voiceLock)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

func (bot *Bot) issueMutesAndRecord(guildID, connectCode string, req task.UserModifyRequest, lock lock.Lock) error {
	return bot.voice.ModifyUsers(guildID, connectCode, req, lock)
}

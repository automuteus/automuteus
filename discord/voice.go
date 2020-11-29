package discord

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"github.com/denverquane/amongusdiscord/metrics"
	"github.com/denverquane/amongusdiscord/storage"
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

func (bot *Bot) applyToSingle(dgs *DiscordGameState, userID string, mute, deaf bool) {
	log.Println("Forcibly applying mute/deaf to " + userID)
	prem := bot.PostgresInterface.GetGuildPremiumStatus(dgs.GuildID)
	uid, _ := strconv.ParseUint(userID, 10, 64)
	req := UserModifyRequest{
		Premium: prem,
		Users: []UserModify{
			{
				UserID: uid,
				Mute:   mute,
				Deaf:   deaf,
			},
		},
	}
	//nil lock because this is an override; we don't care about legitimately obtaining the lock
	mdsc := bot.GalactusClient.ModifyUsers(dgs.GuildID, dgs.ConnectCode, req, nil)
	if mdsc == nil {
		log.Println("Nil response from modifyUsers, probably not good...")
	} else {
		bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MuteDeafenOfficial, mdsc.Official)
		bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MuteDeafenCapture, mdsc.Capture)
		bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MuteDeafenWorker, mdsc.Worker)
	}
	//go guildMemberUpdate(bot.PrimarySession.GetSessionForRequest(dgs.GuildID), params)
}

func (bot *Bot) applyToAll(dgs *DiscordGameState, mute, deaf bool) {
	g, err := bot.PrimarySession.State.Guild(dgs.GuildID)
	if err != nil {
		log.Println(err)
		return
	}

	users := []UserModify{}

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
			uid, _ := strconv.ParseUint(userData.User.UserID, 10, 64)
			users = append(users, UserModify{
				UserID: uid,
				Mute:   mute,
				Deaf:   deaf,
			})
			log.Println("Forcibly applying mute/deaf to " + userData.User.UserID)
		}
	}
	if len(users) > 0 {
		prem := bot.PostgresInterface.GetGuildPremiumStatus(dgs.GuildID)
		req := UserModifyRequest{
			Premium: prem,
			Users:   users,
		}
		//nil lock because this is an override; we don't care about legitimately obtaining the lock
		mdsc := bot.GalactusClient.ModifyUsers(dgs.GuildID, dgs.ConnectCode, req, nil)
		if mdsc == nil {
			log.Println("Nil response from modifyUsers, probably not good...")
		} else {
			bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MuteDeafenOfficial, mdsc.Official)
			bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MuteDeafenCapture, mdsc.Capture)
			bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MuteDeafenWorker, mdsc.Worker)
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

	users := []UserModify{}

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

		incorrectMuteDeafenState := shouldMute != userData.ShouldBeMute || shouldDeaf != userData.ShouldBeDeaf

		//only issue a change if the User isn't in the right state already
		//nicksmatch can only be false if the in-game data is != nil, so the reference to .audata below is safe
		//check the userdata is linked here to not accidentally undeafen music bots, for example
		if linked && incorrectMuteDeafenState {
			uid, _ := strconv.ParseUint(userData.User.UserID, 10, 64)
			userModify := UserModify{
				UserID: uid,
				Mute:   shouldMute,
				Deaf:   shouldDeaf,
			}

			if handlePriority != NoPriority && ((handlePriority == AlivePriority && auData.IsAlive) || (handlePriority == DeadPriority && !auData.IsAlive)) {
				users = append([]UserModify{userModify}, users...)
			} else {
				users = append(users, userModify)
			}
			userData.SetShouldBeMuteDeaf(shouldMute, shouldDeaf)
			dgs.UpdateUserData(userData.User.UserID, userData)
		} else if linked {
			if shouldMute {
				log.Print(fmt.Sprintf("Not muting %s because they're already muted\n", userData.GetUserName()))
			} else {
				log.Print(fmt.Sprintf("Not unmuting %s because they're already unmuted\n", userData.GetUserName()))
			}
		}
	}

	//we relinquish the lock while we wait
	bot.RedisInterface.SetDiscordGameState(dgs, lock)

	voiceLock := bot.RedisInterface.LockVoiceChanges(dgs.ConnectCode, time.Second*time.Duration(delay+1))

	if delay > 0 {
		log.Printf("Sleeping for %d seconds before applying changes to users\n", delay)
		time.Sleep(time.Second * time.Duration(delay))
	}

	if dgs.Running && len(users) > 0 {
		prem := bot.PostgresInterface.GetGuildPremiumStatus(dgs.GuildID)
		req := UserModifyRequest{
			Premium: prem,
			Users:   users,
		}
		mdsc := bot.GalactusClient.ModifyUsers(dgs.GuildID, dgs.ConnectCode, req, voiceLock)
		if mdsc == nil {
			log.Println("Nil response from modifyUsers, probably not good...")
		} else {
			bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MuteDeafenOfficial, mdsc.Official)
			bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MuteDeafenCapture, mdsc.Capture)
			bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MuteDeafenWorker, mdsc.Worker)
		}
	}

}

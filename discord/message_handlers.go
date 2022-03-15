package discord

import (
	"github.com/automuteus/utils/pkg/settings"
	"log"
	"strconv"
	"time"

	"github.com/automuteus/utils/pkg/game"
	"github.com/automuteus/utils/pkg/premium"
	"github.com/automuteus/utils/pkg/task"
	"github.com/bsm/redislock"

	"github.com/bwmarrin/discordgo"
)

// voiceStateChange handles more edge-case behavior for users moving between voice channels, and catches when
// relevant discord api requests are fully applied successfully. Otherwise, we can issue multiple requests for
// the same mute/unmute, erroneously
func (bot *Bot) handleVoiceStateChange(s *discordgo.Session, m *discordgo.VoiceStateUpdate) {
	snowFlakeLock := bot.RedisInterface.LockSnowflake(m.ChannelID + m.UserID + m.SessionID)
	// couldn't obtain lock; bail bail bail!
	if snowFlakeLock == nil {
		return
	}
	defer snowFlakeLock.Release(ctx)

	prem, days := bot.PostgresInterface.GetGuildPremiumStatus(m.GuildID)
	premTier := premium.FreeTier
	if !premium.IsExpired(prem, days) {
		premTier = prem
	}

	sett := bot.StorageInterface.GetGuildSettings(m.GuildID)
	gsr := GameStateRequest{
		GuildID:      m.GuildID,
		VoiceChannel: m.ChannelID,
	}

	stateLock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
	if stateLock == nil {
		return
	}
	defer stateLock.Release(ctx)

	var voiceLock *redislock.Lock
	if dgs.ConnectCode != "" {
		voiceLock = bot.RedisInterface.LockVoiceChanges(dgs.ConnectCode, time.Second)
		if voiceLock == nil {
			return
		}
	}

	g, err := s.State.Guild(dgs.GuildID)

	if err != nil || g == nil {
		return
	}

	// fetch the userData from our userData data cache
	userData, err := dgs.GetUser(m.UserID)
	if err != nil {
		// the User doesn't exist in our userdata cache; add them
		userData, _ = dgs.checkCacheAndAddUser(g, s, m.UserID)
	}

	tracked := m.ChannelID != "" && dgs.VoiceChannel == m.ChannelID

	auData, found := dgs.GameData.GetByName(userData.InGameName)

	var isAlive bool

	// only actually tracked if we're in a tracked channel AND linked to a player
	if !sett.GetMuteSpectator() {
		tracked = tracked && found
		isAlive = auData.IsAlive
	} else {
		if !found {
			// we just assume the spectator is dead
			isAlive = false
		} else {
			isAlive = auData.IsAlive
		}
	}
	mute, deaf := sett.GetVoiceState(isAlive, tracked, dgs.GameData.GetPhase())
	// check the userdata is linked here to not accidentally undeafen music bots, for example
	if found && (userData.ShouldBeDeaf != deaf || userData.ShouldBeMute != mute) && (mute != m.Mute || deaf != m.Deaf) {
		userData.SetShouldBeMuteDeaf(mute, deaf)

		dgs.UpdateUserData(m.UserID, userData)

		if dgs.Running {
			uid, _ := strconv.ParseUint(m.UserID, 10, 64)
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
			mdsc := bot.GalactusClient.ModifyUsers(m.GuildID, dgs.ConnectCode, req, voiceLock)
			if mdsc == nil {
				log.Println("Nil response from modifyUsers, probably not good...")
			} else {
				go RecordDiscordRequestsByCounts(bot.RedisInterface.client, mdsc)
			}
		}
	}
	bot.RedisInterface.SetDiscordGameState(dgs, stateLock)
}

func (bot *Bot) handleGameStartMessage(guildID, textChannelID, voiceChannelID, userID string, sett *settings.GuildSettings, g *discordgo.Guild, connCode string) {
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(GameStateRequest{
		GuildID:     guildID,
		TextChannel: textChannelID,
		ConnectCode: connCode,
	})
	if lock == nil {
		log.Println("Couldn't obtain lock for DGS on game start...")
		return
	}
	dgs.GameData.SetRoomRegionMap("", "", game.EMPTYMAP)

	dgs.clearGameTracking(bot.PrimarySession)

	dgs.Running = true

	if voiceChannelID != "" {
		dgs.VoiceChannel = voiceChannelID
		for _, v := range g.VoiceStates {
			if v.ChannelID == voiceChannelID {
				dgs.checkCacheAndAddUser(g, bot.PrimarySession, v.UserID)
			}
		}
	}

	dgs.CreateMessage(bot.PrimarySession, bot.gameStateResponse(dgs, sett), textChannelID, userID)

	// release the lock
	bot.RedisInterface.SetDiscordGameState(dgs, lock)
}

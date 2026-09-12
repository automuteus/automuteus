package bot

import (
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"strconv"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/lock"
	"github.com/automuteus/automuteus/v8/pkg/task"

	"github.com/bwmarrin/discordgo"
)

// voiceStateChange handles more edge-case behavior for users moving between voice channels, and catches when
// relevant discord api requests are fully applied successfully. Otherwise, we can issue multiple requests for
// the same mute/unmute, erroneously
func (bot *Bot) handleVoiceStateChange(s *discordgo.Session, m *discordgo.VoiceStateUpdate) {
	snowFlakeLock := bot.store.LockSnowflake(m.ChannelID + m.UserID + m.SessionID)
	// couldn't obtain lock; bail bail bail!
	if snowFlakeLock == nil {
		return
	}
	defer snowFlakeLock.Release(ctx)

	sett, settingsErr := bot.settings.LoadGuildSettings(ctx, m.GuildID)
	if settingsErr != nil {
		bot.log.Error("failed to load guild settings", "guild", m.GuildID, "err", settingsErr)
		return
	}
	gsr := GameStateRequest{
		GuildID:      m.GuildID,
		VoiceChannel: m.ChannelID,
	}

	stateLock, dgs := bot.store.GetDiscordGameStateAndLock(gsr)
	if stateLock == nil {
		return
	}
	defer stateLock.Release(ctx)
	gl := bot.gameLog(GameStateRequest{GuildID: m.GuildID, ConnectCode: dgs.ConnectCode, VoiceChannel: m.ChannelID})

	var voiceLock lock.Lock
	if dgs.ConnectCode != "" {
		voiceLock = bot.store.LockVoiceChanges(dgs.ConnectCode, time.Second)
		if voiceLock == nil {
			return
		}
	}

	g, err := bot.guilds.Guild(dgs.GuildID)

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
			gl.Info("voice state changed; applying voice change", "user", m.UserID, "mute", mute, "deaf", deaf)
			uid, _ := strconv.ParseUint(m.UserID, 10, 64)
			req := task.UserModifyRequest{
				Premium: bot.premiumTier(m.GuildID),
				Users: []task.UserModify{
					{
						UserID: uid,
						Mute:   mute,
						Deaf:   deaf,
					},
				},
			}
			err = bot.voice.ModifyUsers(m.GuildID, dgs.ConnectCode, req, voiceLock)
			if err != nil {
				gl.Error("failed to apply voice change", "user", m.UserID, "err", err)
			}
		}
	}
	bot.store.SetDiscordGameState(dgs, stateLock)
}

func (bot *Bot) handleGameStartMessage(guildID, textChannelID, voiceChannelID, userID string, sett *settings.GuildSettings, g *discordgo.Guild, connCode string) {
	lock, dgs := bot.store.GetDiscordGameStateAndLock(GameStateRequest{
		GuildID:     guildID,
		TextChannel: textChannelID,
		ConnectCode: connCode,
	})
	if lock == nil {
		bot.log.Warn("could not lock game state on game start", "guild", guildID, "code", connCode)
		return
	}
	dgs.GameData.Reset()

	dgs.UnlinkAllUsers()
	dgs.VoiceChannel = ""
	dgs.DeleteGameStateMsg(bot.discord, true)

	dgs.Running = true

	if voiceChannelID != "" {
		dgs.VoiceChannel = voiceChannelID
		for _, v := range g.VoiceStates {
			if v.ChannelID == voiceChannelID {
				dgs.checkCacheAndAddUser(g, bot.discord, v.UserID)
			}
		}
	}

	_ = dgs.CreateMessage(bot.discord, bot.gameStateResponse(dgs, sett), textChannelID, userID)

	// release the lock
	bot.store.SetDiscordGameState(dgs, lock)
}

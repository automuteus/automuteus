package bot

import (
	"context"
	"errors"
	"github.com/automuteus/automuteus/v8/internal/server"
	"sync"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/automuteus/automuteus/v8/pkg/notice"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis/v8"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// Platform events (pkg/notice) reach the bot two ways: the active notice is read whenever a game status message is
// rendered, so it appears as a banner on every status message while it is active; and each shard listens for
// published events so it can react immediately. A notice change makes the shard re-read the active notice and
// either refresh status messages or, if it is critical, end every game. A shutdown ends the games whose capture
// connections a Galactus replica is about to sever.

// NoticeSource reads the active platform notice.
type NoticeSource interface {
	Active(ctx context.Context) (*notice.Notice, error)
}

type redisNotices struct {
	client *redis.Client
}

func (r redisNotices) Active(ctx context.Context) (*notice.Notice, error) {
	return notice.Active(ctx, r.client)
}

var _ NoticeSource = redisNotices{}

// activeNotice returns the current platform notice, or nil. Errors are logged and treated as no notice, so a Redis
// hiccup never blocks a status message.
func (bot *Bot) activeNotice() *notice.Notice {
	n, err := bot.notices.Active(ctx)
	if err != nil {
		bot.log.Error("failed to read active notice", "err", err)
		return nil
	}
	return n
}

// applyNotice adds a prominent banner for n to a game status embed and tints the embed by severity.
func applyNotice(embed *discordgo.MessageEmbed, n *notice.Notice, sett *settings.GuildSettings) {
	if embed == nil || n == nil {
		return
	}
	var name string
	if n.Severity == notice.Critical {
		name = sett.LocalizeMessage(&i18n.Message{
			ID:    "notices.banner.critical",
			Other: "🛑 CRITICAL NOTICE",
		})
		embed.Color = discord.RED
	} else {
		name = sett.LocalizeMessage(&i18n.Message{
			ID:    "notices.banner.warning",
			Other: "⚠️ WARNING",
		})
		embed.Color = discord.YELLOW
	}
	banner := &discordgo.MessageEmbedField{
		Name:   name,
		Value:  "**" + n.Message + "**",
		Inline: false,
	}
	embed.Fields = append([]*discordgo.MessageEmbedField{banner}, embed.Fields...)
}

// listenForNotices reacts to published events until the subscription is closed. It is started once per shard.
func (bot *Bot) listenForNotices(sub *redis.PubSub) {
	for msg := range sub.Channel() {
		e, err := notice.DecodeEvent([]byte(msg.Payload))
		if err != nil {
			bot.log.Error("malformed platform event", "err", err)
			continue
		}
		bot.handleEvent(e)
	}
}

// handleEvent applies a platform event to the games this shard runs.
func (bot *Bot) handleEvent(e *notice.Event) {
	games := bot.activeGames()
	switch {
	case e.Shutdown != nil:
		affected := make(map[string]bool, len(e.Shutdown.ConnectCodes))
		for _, code := range e.Shutdown.ConnectCodes {
			affected[code] = true
		}
		var mine []GameStateRequest
		for _, gsr := range games {
			if affected[gsr.ConnectCode] {
				mine = append(mine, gsr)
			}
		}
		bot.log.Info("capture service shutdown announced", "affected", len(mine), "games", len(games))
		forEachGame(mine, func(gsr GameStateRequest) {
			sett := bot.settingsForCleanup(gsr)
			bot.stopGame(gsr, server.EndReasonCaptureShutdown, sett.LocalizeMessage(&i18n.Message{
				ID:    "notices.shutdown.gameEnded",
				Other: "🛑 **The AutoMuteUs capture service is restarting, so this game has been ended and everyone has been unmuted.**\nRun `/new` in a minute to start again.",
			}))
		})

	case e.NoticeChanged:
		n := bot.activeNotice()
		if n != nil && n.Severity == notice.Critical {
			bot.log.Info("critical notice active; ending all games", "message", n.Message, "games", len(games))
			forEachGame(games, func(gsr GameStateRequest) {
				sett := bot.settingsForCleanup(gsr)
				bot.stopGame(gsr, server.EndReasonCriticalNotice, sett.LocalizeMessage(&i18n.Message{
					ID:    "notices.critical.gameEnded",
					Other: "🛑 **This game has been ended by the AutoMuteUs team and everyone has been unmuted.**\n{{.Message}}\nRun `/new` to start again once the notice is over.",
				}, map[string]interface{}{"Message": n.Message}))
			})
			return
		}
		bot.log.Info("platform notice changed; refreshing status messages", "active", n != nil, "games", len(games))
		bot.refreshActiveGames()
	}
}

// settingsForCleanup loads a guild's settings, falling back to defaults: ending a game and unmuting players must
// not depend on the settings store being reachable.
func (bot *Bot) settingsForCleanup(gsr GameStateRequest) *settings.GuildSettings {
	sett, err := bot.settings.LoadGuildSettings(ctx, gsr.GuildID)
	if err != nil {
		bot.gameLog(gsr).Error("failed to load guild settings; using defaults for cleanup", "err", err)
		return settings.MakeGuildSettings()
	}
	return sett
}

// refreshActiveGames re-renders the status message of every game this shard runs, so a banner appears or disappears
// promptly.
func (bot *Bot) refreshActiveGames() {
	forEachGame(bot.activeGames(), func(gsr GameStateRequest) {
		sett, err := bot.settings.LoadGuildSettings(ctx, gsr.GuildID)
		if err != nil {
			return
		}
		if dgs := bot.store.GetReadOnlyDiscordGameState(gsr); dgs != nil && dgs.GameStateMsg.Exists() {
			bot.DispatchRefreshOrEdit(dgs, gsr, sett)
		}
	})
}

// noticeWorkers bounds how many games a shard acts on at once when applying an event. Each game's work is a few
// Discord requests paced by discordgo's per-guild buckets plus, when ending, a wait on its capture subscriber, so
// doing them concurrently makes an event take about as long as the slowest game instead of the sum.
const noticeWorkers = 8

// forEachGame runs fn for every game with at most noticeWorkers in flight, and returns when all are done.
func forEachGame(games []GameStateRequest, fn func(GameStateRequest)) {
	sem := make(chan struct{}, noticeWorkers)
	var wg sync.WaitGroup
	for _, gsr := range games {
		wg.Add(1)
		sem <- struct{}{}
		go func(gsr GameStateRequest) {
			defer wg.Done()
			defer func() { <-sem }()
			fn(gsr)
		}(gsr)
	}
	wg.Wait()
}

// trackGame / untrackGame / activeGames maintain the set of games this shard is subscribed to, keyed by connect code.
func (bot *Bot) trackGame(gsr GameStateRequest) {
	bot.ChannelsMapLock.Lock()
	bot.activeGameRequests[gsr.ConnectCode] = gsr
	bot.metrics.SetActiveGames(len(bot.activeGameRequests))
	bot.ChannelsMapLock.Unlock()
}

func (bot *Bot) untrackGame(connectCode string) {
	bot.ChannelsMapLock.Lock()
	delete(bot.activeGameRequests, connectCode)
	bot.metrics.SetActiveGames(len(bot.activeGameRequests))
	bot.ChannelsMapLock.Unlock()
}

func (bot *Bot) activeGames() []GameStateRequest {
	bot.ChannelsMapLock.RLock()
	defer bot.ChannelsMapLock.RUnlock()
	games := make([]GameStateRequest, 0, len(bot.activeGameRequests))
	for _, gsr := range bot.activeGameRequests {
		games = append(games, gsr)
	}
	return games
}

// finishGame does everything that should happen when a game ends before the capture reported a result: the match
// is recorded as aborted, everyone is unmuted and undeafened, and an optional message is posted in the game's
// channel. It does not delete the game's state or status message; see forceEndGame.
func (bot *Bot) finishGame(dgs *GameState, reason server.EndReason, message string) error {
	gl := bot.gameLog(GameStateRequest{GuildID: dgs.GuildID, ConnectCode: dgs.ConnectCode}).With("reason", reason)
	var result error

	if dgs.MatchID > 0 {
		if err := bot.recorder.AbortGame(dgs.MatchID, time.Now().Unix()); err != nil {
			gl.Error("failed to record match as aborted", "match", dgs.MatchID, "err", err)
			bot.metrics.RecordCleanupFailure(server.CleanupRecordMatch)
			result = errors.Join(result, err)
		} else {
			gl.Info("match aborted", "match", dgs.MatchID)
		}
	}

	if message != "" && dgs.GameStateMsg.MessageChannelID != "" {
		if _, err := bot.discord.ChannelMessageSend(dgs.GameStateMsg.MessageChannelID, message); err != nil {
			gl.Error("failed to post end-of-game message", "err", err)
			bot.metrics.RecordCleanupFailure(server.CleanupNotify)
			result = errors.Join(result, err)
		}
	}

	if err := bot.applyToAll(dgs, bot.premiumTier(dgs.GuildID), false, false); err != nil {
		gl.Error("failed to unmute everyone at end of game", "err", err)
		bot.metrics.RecordCleanupFailure(server.CleanupUnmute)
		result = errors.Join(result, err)
	}
	gl.Info("game ended")
	return result
}

// stopGame asks the capture subscriber to stop between jobs, then perform the final unmute and cleanup.
// A slow subscriber keeps its queued request; deleting its state from here could race with a pending mute.
func (bot *Bot) stopGame(gsr GameStateRequest, reason server.EndReason, message string) error {
	// Slash commands resolve a game by channel, whereas events already carry its connect code.
	dgs := bot.store.GetReadOnlyDiscordGameState(gsr)
	if dgs == nil {
		return nil
	}
	gsr.ConnectCode = dgs.ConnectCode
	bot.ChannelsMapLock.RLock()
	kill, ok := bot.EndGameChannels[gsr.ConnectCode]
	bot.ChannelsMapLock.RUnlock()
	if !ok {
		return bot.completeGame(gsr, reason, message)
	}
	end := EndGameMessage{reason: reason, message: message, done: make(chan error, 1)}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case kill <- end:
		select {
		case err := <-end.done:
			return err
		case <-timer.C:
			bot.gameLog(gsr).Warn("game cleanup is still pending in capture subscriber")
			return context.DeadlineExceeded
		}
	case <-timer.C:
		bot.gameLog(gsr).Warn("capture subscriber did not accept end request")
		return context.DeadlineExceeded
	}
}

// completeGame runs after capture work has stopped. Persisting Running=false also prevents Discord voice
// events generated by the final unmute from enforcing the old game's mute state again.
func (bot *Bot) completeGame(gsr GameStateRequest, reason server.EndReason, message string) error {
	if bot.store.GetReadOnlyDiscordGameState(gsr) == nil {
		return nil
	}
	stateLock, dgs := bot.store.GetDiscordGameStateAndLock(gsr)
	for stateLock == nil {
		stateLock, dgs = bot.store.GetDiscordGameStateAndLock(gsr)
	}
	dgs.Running = false
	bot.store.SetDiscordGameState(dgs, stateLock)
	err := bot.finishGame(dgs, reason, message)
	bot.forceEndGame(gsr)
	bot.metrics.RecordGameEnded(reason)
	return err
}

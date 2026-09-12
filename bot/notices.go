package bot

import (
	"context"
	"sync"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/automuteus/automuteus/v8/pkg/notice"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis/v8"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// Platform notices (pkg/notice) reach the bot two ways: the active notice is read whenever a game status message is
// rendered, so it appears as a banner on every status message while it is active; and each shard listens for
// published notices so it can react immediately, by refreshing status messages or, for critical notices, ending
// every game it runs.

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

// noticeText returns the notice's message in the guild's language when the bot knows the message by ID, and the
// text carried in the notice otherwise (operator-written notices are free text and are shown as written).
func noticeText(sett *settings.GuildSettings, n *notice.Notice) string {
	switch n.MessageID {
	case notice.GalactusShutdownMessageID:
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "notices.galactus.shutdown",
			Other: "The AutoMuteUs capture service is restarting for maintenance.",
		})
	}
	return n.Message
}

// applyNotice adds a prominent banner for n to a game status embed and tints the embed by severity.
func applyNotice(embed *discordgo.MessageEmbed, n *notice.Notice, sett *settings.GuildSettings) {
	if embed == nil || n == nil || n.Cleared {
		return
	}
	var name string
	switch n.Severity {
	case notice.Critical:
		name = sett.LocalizeMessage(&i18n.Message{
			ID:    "notices.banner.critical",
			Other: "🛑 CRITICAL NOTICE",
		})
		embed.Color = discord.RED
	case notice.Warning:
		name = sett.LocalizeMessage(&i18n.Message{
			ID:    "notices.banner.warning",
			Other: "⚠️ WARNING",
		})
		embed.Color = discord.YELLOW
	default:
		name = sett.LocalizeMessage(&i18n.Message{
			ID:    "notices.banner.info",
			Other: "ℹ️ NOTICE",
		})
	}
	banner := &discordgo.MessageEmbedField{
		Name:   name,
		Value:  "**" + noticeText(sett, n) + "**",
		Inline: false,
	}
	embed.Fields = append([]*discordgo.MessageEmbedField{banner}, embed.Fields...)
}

// listenForNotices reacts to published notices until the subscription is closed. It is started once per shard.
func (bot *Bot) listenForNotices(sub *redis.PubSub) {
	for msg := range sub.Channel() {
		n, err := notice.Decode([]byte(msg.Payload))
		if err != nil {
			bot.log.Error("malformed notice", "err", err)
			continue
		}
		bot.handleNotice(n)
	}
}

// handleNotice applies a notice to every game this shard is running: critical notices end them, everything else
// refreshes their status messages so the banner appears (or disappears) promptly.
func (bot *Bot) handleNotice(n *notice.Notice) {
	games := bot.activeGames()
	l := bot.log.With("severity", n.Severity, "source", n.Source, "cleared", n.Cleared, "games", len(games))
	if n.Cleared {
		l.Info("platform notice cleared")
	} else {
		l.Info("platform notice received", "message", n.Message)
	}

	// a notice that expires on its own is never followed by a cleared message, so schedule a refresh for when it
	// lapses; otherwise idle games would keep showing the banner until their next edit
	if !n.Cleared && !n.Targeted() && n.ExpiresAt > 0 {
		if until := time.Until(time.Unix(n.ExpiresAt, 0)); until > 0 {
			go func() {
				bot.sleep(until)
				bot.refreshActiveGames()
			}()
		}
	}

	forEachGame(games, func(gsr GameStateRequest) {
		sett, err := bot.settings.LoadGuildSettings(ctx, gsr.GuildID)
		if err != nil {
			bot.gameLog(gsr).Error("failed to load guild settings for notice", "err", err)
			return
		}
		if n.Severity == notice.Critical && !n.Cleared {
			if !n.Targets(gsr.ConnectCode) {
				return
			}
			text := sett.LocalizeMessage(&i18n.Message{
				ID:    "notices.critical.gameEnded",
				Other: "🛑 **This game has been ended by the AutoMuteUs team and everyone has been unmuted.**\n{{.Message}}\nRun `/new` to start again once the notice is over.",
			}, map[string]interface{}{"Message": noticeText(sett, n)})
			bot.stopGame(gsr, "critical platform notice", text)
			return
		}
		dgs := bot.store.GetReadOnlyDiscordGameState(gsr)
		if dgs == nil || !dgs.GameStateMsg.Exists() {
			return
		}
		bot.DispatchRefreshOrEdit(dgs, gsr, sett)
	})
}

// refreshActiveGames re-renders the status message of every game this shard runs, e.g. after a notice lapses.
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

// noticeWorkers bounds how many games a shard acts on at once when applying a notice. Each game's work is a few
// Discord requests paced by discordgo's per-guild buckets plus, for critical notices, a wait on its capture
// subscriber, so doing them concurrently makes a notice take about as long as the slowest game instead of the sum.
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
	bot.ChannelsMapLock.Unlock()
}

func (bot *Bot) untrackGame(connectCode string) {
	bot.ChannelsMapLock.Lock()
	delete(bot.activeGameRequests, connectCode)
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
func (bot *Bot) finishGame(dgs *GameState, reason, message string) {
	gl := bot.gameLog(GameStateRequest{GuildID: dgs.GuildID, ConnectCode: dgs.ConnectCode}).With("reason", reason)

	if dgs.MatchID > 0 {
		if err := bot.recorder.AbortGame(dgs.MatchID, time.Now().Unix()); err != nil {
			gl.Error("failed to record match as aborted", "match", dgs.MatchID, "err", err)
		} else {
			gl.Info("match aborted", "match", dgs.MatchID)
		}
	}

	if message != "" && dgs.GameStateMsg.MessageChannelID != "" {
		if _, err := bot.discord.ChannelMessageSend(dgs.GameStateMsg.MessageChannelID, message); err != nil {
			gl.Error("failed to post end-of-game message", "err", err)
		}
	}

	if err := bot.applyToAll(dgs, bot.premiumTier(dgs.GuildID), false, false); err != nil {
		gl.Error("failed to unmute everyone at end of game", "err", err)
	}
	gl.Info("game ended")
}

// stopGame ends a game from outside its capture subscriber: it finishes the game, then signals the subscriber, which
// deletes the game's state and status message. If no subscriber is running, the game is deleted directly.
func (bot *Bot) stopGame(gsr GameStateRequest, reason, message string) {
	dgs := bot.store.GetReadOnlyDiscordGameState(gsr)
	if dgs == nil {
		return
	}
	bot.finishGame(dgs, reason, message)

	bot.ChannelsMapLock.Lock()
	kill, ok := bot.EndGameChannels[dgs.ConnectCode]
	delete(bot.EndGameChannels, dgs.ConnectCode)
	bot.ChannelsMapLock.Unlock()

	if !ok {
		bot.forceEndGame(gsr)
		return
	}
	select {
	case kill <- true:
	case <-time.After(5 * time.Second):
		bot.gameLog(gsr).Warn("capture subscriber did not accept end signal; deleting game directly")
		bot.forceEndGame(gsr)
	}
}

var _ NoticeSource = redisNotices{}

var _ NoticeSource = redisNotices{}

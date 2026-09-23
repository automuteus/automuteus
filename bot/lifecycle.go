package bot

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"

	redis_common "github.com/automuteus/automuteus/v8/common"
	"github.com/automuteus/automuteus/v8/internal/server"
	"github.com/automuteus/automuteus/v8/pkg/notice"
	"github.com/automuteus/automuteus/v8/pkg/task"
	"github.com/go-redis/redis/v8"
)

// InflightDrainTimeout bounds how long shutdown waits for handlers that were already running when draining began.
// Keep it well under the orchestrator's termination grace period (30s by default), leaving time to close sessions.
const InflightDrainTimeout = 10 * time.Second

// Drain puts the shard into shutdown mode, so that a redeploy is invisible to players when a twin process (one
// identifying with the same shard IDs) is running:
//
//   - new interactions and voice events are refused before any lock is taken, so the twin wins the lock and answers;
//   - capture subscribers release their consumer leases and stop popping jobs, so a standby takes over at once;
//   - the games this shard was running are announced, so twins subscribe to any they had not seen.
//
// Handlers already running keep going; WaitForInflight lets shutdown give them time to finish, after which
// AnnounceGames should run again for games attached during that window.
func (bot *Bot) Drain() {
	if !bot.draining.CompareAndSwap(false, true) {
		return
	}
	games := bot.activeGames()
	bot.log.Info("draining; refusing new events", "games", len(games), "inflight", bot.inflight.Load())
	bot.AnnounceGames()
	// wake this process's own subscribers so they hand their consumer leases over now rather than on their next poll
	if client := bot.redisClient(); client != nil {
		for _, gsr := range games {
			task.Notify(ctx, client, gsr.ConnectCode)
		}
	}
}

// AnnounceGames tells every other process about the games this shard is subscribed to, so that each has a standby.
func (bot *Bot) AnnounceGames() {
	games := bot.activeGames()
	client := bot.redisClient()
	if client == nil || len(games) == 0 {
		return
	}
	refs := make([]notice.GameRef, 0, len(games))
	for _, gsr := range games {
		refs = append(refs, notice.GameRef{GuildID: gsr.GuildID, ConnectCode: gsr.ConnectCode})
	}
	if err := notice.AnnounceGames(ctx, client, refs); err != nil {
		bot.log.Error("failed to announce games", "err", err)
		return
	}
	bot.log.Info("announced games", "games", len(refs))
}

// announceGame tells every other process about one game, right after it is created, so it has a standby from the
// start rather than from the next restart or drain.
func (bot *Bot) announceGame(guildID, connectCode string) {
	client := bot.redisClient()
	if client == nil {
		return
	}
	if err := notice.AnnounceGames(ctx, client, []notice.GameRef{{GuildID: guildID, ConnectCode: connectCode}}); err != nil {
		bot.gameLog(GameStateRequest{GuildID: guildID, ConnectCode: connectCode}).Error("failed to announce new game", "err", err)
	}
}

// DiscoveryInterval is how often a shard looks for recently active games in its guilds that it is not subscribed
// to. Announcements cover the normal case; discovery recovers games whose announcement this process missed, for
// example because it was still identifying when the game was created.
const DiscoveryInterval = time.Minute

// discoverGames runs discoverGamesOnce until the shard drains.
func (bot *Bot) discoverGames() {
	ticker := time.NewTicker(DiscoveryInterval)
	defer ticker.Stop()
	for range ticker.C {
		if bot.Draining() {
			return
		}
		bot.discoverGamesOnce()
	}
}

// discoverGamesOnce subscribes to every recently active game in a guild this shard serves, and reports how many it
// newly attached to.
func (bot *Bot) discoverGamesOnce() int {
	if bot.Draining() || bot.RedisInterface == nil {
		return 0
	}
	adopted := 0
	for _, gsr := range bot.RedisInterface.LoadRecentGames() {
		if _, err := bot.guilds.Guild(gsr.GuildID); err != nil {
			continue
		}
		if bot.attachToGame(gsr, server.AdoptDiscovery) {
			adopted++
		}
	}
	if adopted > 0 {
		bot.log.Info("discovered games without a subscription here", "adopted", adopted)
	}
	return adopted
}

// Draining reports whether Drain has been called.
func (bot *Bot) Draining() bool {
	return bot.draining.Load()
}

// WaitForInflight blocks until every handler that was running when draining began has finished, or timeout
// elapses. It returns how many were still running.
func (bot *Bot) WaitForInflight(timeout time.Duration) int64 {
	deadline := time.Now().Add(timeout)
	for {
		n := bot.inflight.Load()
		if n == 0 || !time.Now().Before(deadline) {
			return n
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// beginWork counts a handler as in flight until the returned function is called.
func (bot *Bot) beginWork() func() {
	bot.inflight.Add(1)
	return func() { bot.inflight.Add(-1) }
}

func (bot *Bot) redisClient() *redis.Client {
	if bot.RedisInterface == nil {
		return nil
	}
	return bot.RedisInterface.client
}

// adoptGames attaches this shard to announced games in the guilds it serves. Games it is already attached to,
// games in guilds served by other shards, and games no longer in the store are skipped.
func (bot *Bot) adoptGames(games []notice.GameRef) {
	if bot.Draining() {
		return
	}
	adopted := 0
	for _, g := range games {
		if _, err := bot.guilds.Guild(g.GuildID); err != nil {
			continue
		}
		if bot.attachToGame(GameStateRequest{GuildID: g.GuildID, ConnectCode: g.ConnectCode}, server.AdoptAnnounce) {
			adopted++
		}
	}
	if adopted > 0 {
		bot.log.Info("subscribed to announced games", "offered", len(games), "adopted", adopted)
	}
}

// attachToGame subscribes this shard to the capture events of a game that already exists in the store, unless it
// is attached already. It reports whether a new subscription was started, and counts it against source. It never takes the game's state lock: a
// locked fetch creates an empty state when none exists, which would resurrect a game that ended between the check
// and the lock. The subscriber re-checks existence under the consumer lease before consuming anything.
func (bot *Bot) attachToGame(gsr GameStateRequest, source server.AdoptSource) bool {
	bot.ChannelsMapLock.RLock()
	_, attached := bot.EndGameChannels[gsr.ConnectCode]
	bot.ChannelsMapLock.RUnlock()
	if attached {
		return false
	}
	dgs, err := bot.store.ReadDiscordGameState(gsr)
	if err != nil {
		bot.gameLog(gsr).Error("failed to read game for adoption; will retry discovery", "err", err)
		return false
	}
	if dgs == nil || dgs.ConnectCode == "" {
		return false
	}

	killChan := make(chan EndGameMessage, 1)
	bot.ChannelsMapLock.Lock()
	if _, attached := bot.EndGameChannels[gsr.ConnectCode]; attached {
		bot.ChannelsMapLock.Unlock()
		return false
	}
	bot.EndGameChannels[gsr.ConnectCode] = killChan
	bot.ChannelsMapLock.Unlock()

	bot.gameLog(gsr).Info("subscribing to an existing game", "source", string(source))
	bot.metrics.RecordGameAdopted(source)
	go bot.SubscribeToGameByConnectCode(gsr.GuildID, gsr.ConnectCode, killChan)
	return true
}

// reservationGuardTTL is how long a reservation blocks further interactions from the same user while the reserved
// one is still running. It is shortened to the command's normal cooldown on commit and removed on release, so it
// only ever outlives the handler if the process dies mid-interaction.
const reservationGuardTTL = 30 * time.Second

// rateLimitReservation marks a user's rate limit at admission, so that further interactions from the same user are
// refused while this one runs, and lifts it again if the response never reaches Discord (the process exiting, a
// failed response), so that retrying a dropped command is not mistaken for spam. Each reservation carries its own
// token and only ever shortens or removes keys that still hold that token, so a slow interaction's late failure
// cannot lift a reservation a newer interaction has since made.
type rateLimitReservation struct {
	client   *redis.Client
	userID   string
	cmdType  string
	token    string
	ttl      time.Duration
	reserved bool
}

func (r *rateLimitReservation) keys() []string {
	keys := []string{redis_common.UserRateLimitGeneralKey(r.userID)}
	if r.cmdType != "" && r.ttl > 0 {
		keys = append(keys, redis_common.UserRateLimitSpecificKey(r.userID, r.cmdType))
	}
	return keys
}

// reserve blocks the user's further interactions for the guard period, or until commit or release.
func (r *rateLimitReservation) reserve(userID, cmdType string, ttl time.Duration) {
	if r == nil || r.client == nil {
		return
	}
	var id [8]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	r.userID, r.cmdType, r.ttl, r.token, r.reserved = userID, cmdType, ttl, hex.EncodeToString(id[:]), true
	for _, key := range r.keys() {
		if err := r.client.Set(ctx, key, r.token, reservationGuardTTL).Err(); err != nil {
			slog.Default().Warn("failed to reserve rate limit", "user", userID, "err", err)
		}
	}
}

// commit, after the response reached Discord, shortens the reservation to the command's normal cooldown.
func (r *rateLimitReservation) commit() {
	if r == nil || !r.reserved {
		return
	}
	r.reserved = false
	cooldowns := []time.Duration{redis_common.GlobalUserRateLimitDuration, r.ttl}
	for i, key := range r.keys() {
		if err := r.client.Eval(ctx, renewScript, []string{key}, r.token, cooldowns[i].Milliseconds()).Err(); err != nil {
			slog.Default().Warn("failed to commit rate limit", "user", r.userID, "err", err)
		}
	}
}

// release, after a dropped interaction, lifts the reservation so the user can retry at once.
func (r *rateLimitReservation) release() {
	if r == nil || !r.reserved {
		return
	}
	r.reserved = false
	for _, key := range r.keys() {
		if err := r.client.Eval(ctx, releaseScript, []string{key}, r.token).Err(); err != nil {
			slog.Default().Warn("failed to lift rate limit after a dropped interaction", "user", r.userID, "err", err)
		}
	}
}

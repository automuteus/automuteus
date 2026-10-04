package tokenprovider

import (
	"context"
	"sort"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/bwmarrin/discordgo"
)

// UnverifiedGuildLimit is the most servers Discord lets an unverified bot join.
const UnverifiedGuildLimit = 100

// Only IDs and availability are retained; worker sessions do not need guild caches.
type workerMembership struct {
	guilds     map[string]bool
	ready      bool
	connected  bool
	quietUntil time.Time
	// userID, name and guildLimit (0 for none) come from the worker's READY.
	userID     string
	name       string
	guildLimit int
}

func (tp *TokenProvider) openWorkerSession(key string, s *discordgo.Session) error {
	// Keep only the small membership inventory, and process it in gateway order.
	s.StateEnabled = false
	s.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuilds)
	s.SyncEvents = true
	tp.trackWorker(key, s)
	if err := s.Open(); err != nil {
		// Stop any partially opened gateway before removing its inventory. Otherwise
		// a never-ready entry would prevent every subsequent cleanup step.
		_ = s.Close()
		tp.untrackWorker(key)
		return err
	}
	return nil
}

func (tp *TokenProvider) untrackWorker(key string) {
	tp.membershipMu.Lock()
	defer tp.membershipMu.Unlock()
	delete(tp.memberships, key)
}

// Unknown inventories retain the existing routing fallback. Once READY establishes
// membership, do not spend a Discord request on workers known to be absent.
func (tp *TokenProvider) workerKnownAbsent(key, guild string) bool {
	tp.membershipMu.Lock()
	defer tp.membershipMu.Unlock()
	w := tp.memberships[key]
	if w == nil || !w.ready || !w.connected {
		return false
	}
	_, present := w.guilds[guild]
	return !present
}

// WorkersPresent counts the worker bots that are members of guild, out of all workers this process runs. An
// unavailable guild counts as present. known is false when no workers run, some configured worker is not tracked
// (still starting, or failed to open), or any inventory is incomplete or disconnected; the count must not be acted
// on then.
func (tp *TokenProvider) WorkersPresent(guild string) (present, total int, known bool) {
	tp.membershipMu.Lock()
	defer tp.membershipMu.Unlock()
	if len(tp.memberships) == 0 || len(tp.memberships) < tp.configuredWorkers {
		return 0, 0, false
	}
	for _, w := range tp.memberships {
		if !w.ready || !w.connected {
			return 0, 0, false
		}
		if _, ok := w.guilds[guild]; ok {
			present++
		}
	}
	return present, len(tp.memberships), true
}

// WorkerStatus reports whether the worker bot with user ID botID is a member of guild, and whether it is in as many
// servers as Discord allows it. known is false when this process does not run that worker or its inventory is
// incomplete or disconnected.
func (tp *TokenProvider) WorkerStatus(botID, guild string) (member, full, known bool) {
	tp.membershipMu.Lock()
	defer tp.membershipMu.Unlock()
	for _, w := range tp.memberships {
		if w.userID != botID {
			continue
		}
		if !w.ready || !w.connected {
			return false, false, false
		}
		_, member = w.guilds[guild]
		return member, w.guildLimit > 0 && len(w.guilds) >= w.guildLimit, true
	}
	return false, false, false
}

func (tp *TokenProvider) trackWorker(key string, s *discordgo.Session) {
	tp.membershipMu.Lock()
	tp.memberships[key] = &workerMembership{guilds: make(map[string]bool)}
	tp.membershipMu.Unlock()
	s.AddHandler(func(_ *discordgo.Session, e *discordgo.Ready) { tp.workerReady(key, e) })
	s.AddHandler(func(_ *discordgo.Session, e *discordgo.GuildCreate) { tp.workerGuildCreate(key, e) })
	s.AddHandler(func(_ *discordgo.Session, e *discordgo.GuildDelete) { tp.workerGuildDelete(key, e) })
	s.AddHandler(func(_ *discordgo.Session, _ *discordgo.Disconnect) {
		tp.membershipMu.Lock()
		if w := tp.memberships[key]; w != nil {
			w.connected = false
		}
		tp.membershipMu.Unlock()
	})
	s.AddHandler(func(_ *discordgo.Session, _ *discordgo.Resumed) {
		tp.membershipMu.Lock()
		if w := tp.memberships[key]; w != nil {
			w.connected = true
		}
		tp.membershipMu.Unlock()
	})
	s.AddHandler(func(_ *discordgo.Session, e *discordgo.RateLimit) {
		// Keep gateway handling quick. The local pause takes effect immediately;
		// Redis also asks other processes using this token to pause maintenance.
		delay := time.Minute
		if e.TooManyRequests != nil && e.RetryAfter > delay {
			delay = e.RetryAfter
		}
		tp.pauseWorker(key, delay)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := tp.extendWorkerPause(ctx, key, delay); err != nil {
				tp.logger("").Warn("failed to share worker cleanup backoff", "err", err)
			}
		}()
	})
}

func (tp *TokenProvider) workerReady(key string, e *discordgo.Ready) {
	tp.membershipMu.Lock()
	defer tp.membershipMu.Unlock()
	w := tp.memberships[key]
	if w == nil {
		return
	}
	w.guilds = make(map[string]bool, len(e.Guilds))
	for _, g := range e.Guilds {
		w.guilds[g.ID] = !g.Unavailable
	}
	w.ready, w.connected = true, true
	if e.User != nil {
		w.userID, w.name = e.User.ID, e.User.Username
		// Being in more servers than the limit also proves verification, in case READY omits the flag.
		verified := e.User.PublicFlags&discordgo.UserFlagVerifiedBot != 0 ||
			discordgo.UserFlags(e.User.Flags)&discordgo.UserFlagVerifiedBot != 0 ||
			len(w.guilds) > UnverifiedGuildLimit
		w.guildLimit = 0
		if !verified {
			w.guildLimit = UnverifiedGuildLimit
		}
	}
	tp.logger("").Info("worker ready", "worker", w.name, "guilds", len(w.guilds), "guild_limit", w.guildLimit)
	tp.publishWorkerGuilds(w)
}

// publishWorkerGuilds reports w's server count, so a worker nearing Discord's limit is visible before invites fail.
// Callers hold membershipMu.
func (tp *TokenProvider) publishWorkerGuilds(w *workerMembership) {
	if tp.metrics == nil || w.name == "" {
		return
	}
	tp.metrics.SetWorkerGuilds(w.name, len(w.guilds), w.guildLimit)
}

func (tp *TokenProvider) workerGuildCreate(key string, e *discordgo.GuildCreate) {
	tp.membershipMu.Lock()
	defer tp.membershipMu.Unlock()
	if w := tp.memberships[key]; w != nil {
		w.guilds[e.ID] = !e.Unavailable
		tp.publishWorkerGuilds(w)
	}
}

func (tp *TokenProvider) workerGuildDelete(key string, e *discordgo.GuildDelete) {
	tp.membershipMu.Lock()
	defer tp.membershipMu.Unlock()
	w := tp.memberships[key]
	if w == nil {
		return
	}
	if e.Unavailable {
		w.guilds[e.ID] = false
	} else {
		delete(w.guilds, e.ID)
	}
	tp.publishWorkerGuilds(w)
}

// An incomplete/disconnected inventory must never determine which workers to retain.
func (tp *TokenProvider) membershipSnapshot() (map[string][]string, map[string]bool, bool) {
	tp.membershipMu.Lock()
	defer tp.membershipMu.Unlock()
	// A process with no successfully tracked workers has no fleet inventory.
	if len(tp.memberships) == 0 {
		return nil, nil, false
	}
	guilds, unavailable := make(map[string][]string), make(map[string]bool)
	for key, w := range tp.memberships {
		if !w.ready || !w.connected {
			return nil, nil, false
		}
		for guild, available := range w.guilds {
			guilds[guild] = append(guilds[guild], key)
			if !available {
				unavailable[guild] = true
			}
		}
	}
	for _, workers := range guilds {
		sort.Strings(workers)
	}
	return guilds, unavailable, true
}

func (tp *TokenProvider) pauseWorker(key string, delay time.Duration) {
	tp.membershipMu.Lock()
	defer tp.membershipMu.Unlock()
	if w := tp.memberships[key]; w != nil {
		if until := time.Now().Add(delay); until.After(w.quietUntil) {
			w.quietUntil = until
		}
	}
}

func (tp *TokenProvider) workerQuiet(key string) bool {
	tp.membershipMu.Lock()
	defer tp.membershipMu.Unlock()
	w := tp.memberships[key]
	return w != nil && w.ready && w.connected && !time.Now().Before(w.quietUntil)
}

func (tp *TokenProvider) extendWorkerPause(ctx context.Context, key string, delay time.Duration) error {
	// A short rate-limit event must not shorten an existing, longer pause.
	return tp.client.Eval(ctx, `if redis.call('pttl', KEYS[1]) < tonumber(ARGV[1]) then
		return redis.call('psetex', KEYS[1], ARGV[1], '1') end return 0`,
		[]string{rediskey.WorkerCleanupPause(key)}, delay.Milliseconds()).Err()
}

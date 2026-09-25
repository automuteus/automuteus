package tokenprovider

import (
	"context"
	"sort"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/bwmarrin/discordgo"
)

// Only IDs and availability are retained; worker sessions do not need guild caches.
type workerMembership struct {
	guilds     map[string]bool
	ready      bool
	connected  bool
	quietUntil time.Time
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
}

func (tp *TokenProvider) workerGuildCreate(key string, e *discordgo.GuildCreate) {
	tp.membershipMu.Lock()
	defer tp.membershipMu.Unlock()
	if w := tp.memberships[key]; w != nil {
		w.guilds[e.ID] = !e.Unavailable
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

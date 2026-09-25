package tokenprovider

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/automuteus/automuteus/v8/internal/server"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/task"
	"github.com/bsm/redislock"
	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
)

type cleanupTransport func(*http.Request) (*http.Response, error)

func (f cleanupTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type cleanupHarness struct {
	c         *workerCleanup
	r         *miniredis.Miniredis
	mu        sync.Mutex
	requests  []string
	status    int
	body      string
	tier      premium.Tier
	days      int
	lookupErr error
	lookups   []string
}

func newCleanupHarness(t *testing.T, r *miniredis.Miniredis) *cleanupHarness {
	t.Helper()
	if r == nil {
		r = miniredis.RunT(t)
	}
	client := redis.NewClient(&redis.Options{Addr: r.Addr(), MaxRetries: -1})
	t.Cleanup(func() { client.Close() })
	tp := NewTokenProvider(client, nil, time.Second, 100)
	tp.metrics = server.NewMetrics(prometheus.NewRegistry())
	h := &cleanupHarness{r: r, status: http.StatusNoContent, days: premium.NoExpiryCode}
	h.c = &workerCleanup{tp: tp, cfg: CleanupConfig{CheckInterval: time.Second, LeaveInterval: 45 * time.Second}}
	h.c.premium = func(_ context.Context, guild string) (premium.Tier, int, error) {
		h.lookups = append(h.lookups, guild)
		return h.tier, h.days, h.lookupErr
	}
	return h
}

func (h *cleanupHarness) worker(t *testing.T, key string, guilds ...string) {
	t.Helper()
	s, err := discordgo.New("Bot test")
	if err != nil {
		t.Fatal(err)
	}
	s.Client = &http.Client{Transport: cleanupTransport(func(r *http.Request) (*http.Response, error) {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.requests = append(h.requests, key+" "+r.Method+" "+r.URL.Path)
		return &http.Response{StatusCode: h.status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(h.body)), Request: r}, nil
	})}
	h.c.tp.activeSessions[key] = s
	h.c.tp.memberships[key] = &workerMembership{guilds: make(map[string]bool)}
	ready := &discordgo.Ready{}
	for _, guild := range guilds {
		ready.Guilds = append(ready.Guilds, &discordgo.Guild{ID: guild})
	}
	h.c.tp.workerReady(key, ready)
}

func (h *cleanupHarness) step(t *testing.T) {
	t.Helper()
	if err := h.c.step(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestWorkerCleanupEvictsWithoutVoiceTraffic(t *testing.T) {
	for _, tc := range []struct {
		name       string
		tier       premium.Tier
		days, keep int
	}{
		{"free", premium.FreeTier, premium.NoExpiryCode, 0},
		{"bronze", premium.BronzeTier, premium.NoExpiryCode, 0},
		{"trial", premium.TrialTier, premium.NoExpiryCode, 0},
		{"silver", premium.SilverTier, premium.NoExpiryCode, 1},
		{"gold", premium.GoldTier, premium.NoExpiryCode, 3},
		{"expired_gold", premium.GoldTier, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newCleanupHarness(t, nil)
			h.tier, h.days = tc.tier, tc.days
			for _, key := range []string{"a", "b", "c", "d"} {
				h.worker(t, key, "123")
			}
			for i := 0; i < 5; i++ {
				h.step(t)
				h.r.FastForward(time.Minute)
			}
			guilds, _, _ := h.c.tp.membershipSnapshot()
			if len(guilds["123"]) != tc.keep {
				t.Fatalf("retained %v, want %d workers", guilds["123"], tc.keep)
			}
			if len(h.requests) != 4-tc.keep {
				t.Fatalf("requests = %v", h.requests)
			}
			for _, req := range h.requests {
				if !strings.Contains(req, " DELETE /api/v9/users/@me/guilds/123") {
					t.Fatalf("unexpected Discord request: %s", req)
				}
			}
			if tc.keep > 0 && guilds["123"][0] != "a" {
				t.Fatal("retention was not deterministic")
			}
		})
	}
}

func TestWorkerCleanupRechecksQueuedEntitlement(t *testing.T) {
	h := newCleanupHarness(t, nil)
	h.worker(t, "a", "123")
	h.r.Set(rediskey.WorkerCleanupLeaveBudget, "busy")
	h.step(t)
	if n, _ := h.c.tp.client.ZCard(context.Background(), rediskey.WorkerCleanupPending).Result(); n != 1 {
		t.Fatal("expected queued departure")
	}
	h.tier = premium.SilverTier // renewal after enqueue, before departure
	h.r.Del(rediskey.WorkerCleanupLeaveBudget)
	if err := h.c.depart(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(h.requests) != 0 {
		t.Fatal("renewed server was evicted")
	}
	if n, _ := h.c.tp.client.ZCard(context.Background(), rediskey.WorkerCleanupPending).Result(); n != 0 {
		t.Fatal("renewed server remained queued")
	}
}

func TestWorkerCleanupDefersOnUncertaintyAndActivity(t *testing.T) {
	for _, cause := range []string{"database", "redis", "unavailable", "disconnected", "not_ready", "active_game", "recent_voice", "shared_backoff", "unknown_tier"} {
		t.Run(cause, func(t *testing.T) {
			h := newCleanupHarness(t, nil)
			h.worker(t, "a", "123")
			switch cause {
			case "database":
				h.lookupErr = errors.New("database unavailable")
			case "redis":
				h.r.SetError("unavailable")
			case "unavailable":
				h.c.tp.workerGuildDelete("a", &discordgo.GuildDelete{Guild: &discordgo.Guild{ID: "123", Unavailable: true}})
			case "disconnected":
				h.c.tp.memberships["a"].connected = false
			case "not_ready":
				h.c.tp.memberships["a"].ready = false
			case "active_game":
				h.r.ZAdd(rediskey.ActiveGamesForGuild("123"), float64(time.Now().Add(time.Minute).Unix()), "GAME")
			case "recent_voice":
				h.c.tp.pauseWorker("a", time.Minute)
			case "shared_backoff":
				h.r.Set(rediskey.WorkerCleanupPause("a"), "1")
			case "unknown_tier":
				h.tier = premium.Tier(99)
			}
			err := h.c.step(context.Background())
			if cause == "database" || cause == "redis" || cause == "unknown_tier" {
				if err == nil {
					t.Fatal("expected lookup error")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if len(h.requests) != 0 {
				t.Fatalf("sent requests despite %s: %v", cause, h.requests)
			}
		})
	}
}

func TestWorkerCleanupActivityExpiry(t *testing.T) {
	h := newCleanupHarness(t, nil)
	h.worker(t, "a", "123")
	h.r.ZAdd(rediskey.ActiveGamesForGuild("123"), float64(time.Now().Unix()), "GAME")
	h.step(t)
	if len(h.requests) != 0 {
		t.Fatal("evicted during a game")
	}
	h.r.ZAdd(rediskey.ActiveGamesForGuild("123"), float64(time.Now().Unix()-rediskey.ActiveGameTimeoutSeconds-1), "GAME")
	h.step(t)
	if len(h.requests) != 1 {
		t.Fatal("stale game prevented departure")
	}
}

func TestWorkerCleanupFleetBudgetAndLease(t *testing.T) {
	one := newCleanupHarness(t, nil)
	two := newCleanupHarness(t, one.r)
	one.worker(t, "a", "123")
	two.worker(t, "a", "123")
	lease, err := redislock.New(one.c.tp.client).Obtain(context.Background(), rediskey.WorkerCleanupLease, 30*time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	two.step(t)
	if len(two.lookups) > 0 {
		t.Fatal("second process checked a guild while lease held")
	}
	if err := lease.Release(context.Background()); err != nil {
		t.Fatal(err)
	}
	one.step(t)
	two.step(t)
	if len(one.requests)+len(two.requests) != 1 {
		t.Fatal("processes exceeded shared departure budget")
	}
	if ttl := one.r.TTL(rediskey.WorkerCleanupLeaveBudget); ttl < 45*time.Second || ttl > time.Minute {
		t.Fatalf("departure budget TTL = %v", ttl)
	}
	if len(two.lookups) != 0 {
		t.Fatal("processes exceeded shared scan budget")
	}
}

func TestWorkerCleanupFailuresSpendBudgetAndRetryLater(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusTooManyRequests} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			h := newCleanupHarness(t, nil)
			h.worker(t, "a", "123")
			h.status, h.body = status, `{"message":"try later","retry_after":120}`
			if err := h.c.step(context.Background()); err == nil {
				t.Fatal("expected Discord failure")
			}
			h.step(t)
			if len(h.requests) != 1 {
				t.Fatalf("failure retried without pacing: %v", h.requests)
			}
			if n, _ := h.c.tp.client.ZCard(context.Background(), rediskey.WorkerCleanupPending).Result(); n != 1 {
				t.Fatal("failed departure lost from queue")
			}
			if status == http.StatusTooManyRequests && h.r.TTL(rediskey.WorkerCleanupPause("a")) < 119*time.Second {
				t.Fatal("rate-limit backoff was not shared")
			}
		})
	}
}

func TestWorkerCleanupFairnessAcrossRestart(t *testing.T) {
	one := newCleanupHarness(t, nil)
	one.worker(t, "a", "123", "456")
	one.r.ZAdd(rediskey.WorkerCleanupSchedule, 1, "123")
	one.r.ZAdd(rediskey.WorkerCleanupSchedule, 2, "456")
	one.lookupErr = errors.New("query failed")
	if err := one.c.step(context.Background()); err == nil {
		t.Fatal("expected failure")
	}
	one.r.FastForward(time.Second)
	two := newCleanupHarness(t, one.r)
	two.worker(t, "a", "123", "456")
	two.tier = premium.SilverTier
	two.step(t)
	if len(two.lookups) != 1 || two.lookups[0] != "456" {
		t.Fatalf("sweep restarted at failed guild: %v", two.lookups)
	}
}

func TestWorkerMembershipTracksAvailabilityAndReadyReplacement(t *testing.T) {
	h := newCleanupHarness(t, nil)
	h.worker(t, "a", "123")
	h.c.tp.workerGuildDelete("a", &discordgo.GuildDelete{Guild: &discordgo.Guild{ID: "123", Unavailable: true}})
	guilds, unavailable, ready := h.c.tp.membershipSnapshot()
	if !ready || len(guilds["123"]) != 1 || !unavailable["123"] {
		t.Fatal("temporary unavailability lost membership")
	}
	h.c.tp.workerGuildCreate("a", &discordgo.GuildCreate{Guild: &discordgo.Guild{ID: "123"}})
	h.c.tp.workerReady("a", &discordgo.Ready{Guilds: []*discordgo.Guild{{ID: "456"}}})
	guilds, _, _ = h.c.tp.membershipSnapshot()
	if len(guilds["123"]) != 0 || len(guilds["456"]) != 1 {
		t.Fatal("READY retained stale memberships")
	}
}

func TestModifyUsersDoesNotPerformMembershipMaintenance(t *testing.T) {
	h := newCleanupHarness(t, nil)
	h.worker(t, "a", "123")
	h.c.tp.applyPrimary = func(_, _ string, _, _ bool) error { return nil }
	if err := h.c.tp.ModifyUsers("123", "CODE", task.UserModifyRequest{Premium: premium.FreeTier, Users: []task.UserModify{{UserID: 456}}}, nil); err != nil {
		t.Fatal(err)
	}
	// Any obsolete background membership check would emit a Discord request.
	time.Sleep(10 * time.Millisecond)
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.requests) != 0 {
		t.Fatalf("voice batch triggered maintenance: %v", h.requests)
	}
}

func TestWorkerCleanupBusyGuildDoesNotBlockQueue(t *testing.T) {
	h := newCleanupHarness(t, nil)
	h.worker(t, "a", "123", "456")
	h.r.ZAdd(rediskey.WorkerCleanupPending, 1, "123")
	h.r.ZAdd(rediskey.WorkerCleanupPending, 2, "456")
	h.r.ZAdd(rediskey.ActiveGamesForGuild("123"), float64(time.Now().Unix()), "GAME")
	if err := h.c.depart(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := h.c.depart(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(h.requests) != 1 || !strings.HasSuffix(h.requests[0], "/456") {
		t.Fatalf("busy guild blocked queue: %v", h.requests)
	}
}

func TestWorkerCleanupQueuedLookupFailureDoesNotEvict(t *testing.T) {
	h := newCleanupHarness(t, nil)
	h.worker(t, "a", "123")
	h.r.ZAdd(rediskey.WorkerCleanupPending, 1, "123")
	h.lookupErr = errors.New("database unavailable after enqueue")
	if err := h.c.depart(context.Background()); err == nil {
		t.Fatal("expected lookup failure")
	}
	if len(h.requests) != 0 {
		t.Fatal("evicted on failed entitlement recheck")
	}
}

func TestWorkerCleanupDoesNotHoldSessionLockDuringDeparture(t *testing.T) {
	h := newCleanupHarness(t, nil)
	h.worker(t, "a", "123")
	h.c.tp.activeSessions["a"].Client.Transport = cleanupTransport(func(r *http.Request) (*http.Response, error) {
		if !h.c.tp.sessionLock.TryLock() {
			t.Fatal("session lock held during Discord request")
		}
		h.c.tp.sessionLock.Unlock()
		return &http.Response{StatusCode: http.StatusNoContent, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})
	h.step(t)
}

func TestCleanupConfigFromEnv(t *testing.T) {
	t.Setenv("WORKER_CLEANUP_CHECK_INTERVAL", "")
	t.Setenv("WORKER_CLEANUP_LEAVE_INTERVAL", "")
	cfg, err := CleanupConfigFromEnv()
	if err != nil || cfg.CheckInterval != 5*time.Second || cfg.LeaveInterval != 45*time.Second {
		t.Fatalf("defaults = %+v, %v", cfg, err)
	}
	t.Setenv("WORKER_CLEANUP_CHECK_INTERVAL", "30s")
	t.Setenv("WORKER_CLEANUP_LEAVE_INTERVAL", "2m")
	cfg, err = CleanupConfigFromEnv()
	if err != nil || cfg.CheckInterval != 30*time.Second || cfg.LeaveInterval != 2*time.Minute {
		t.Fatalf("config = %+v, %v", cfg, err)
	}
	for _, value := range []string{"0", "-1s", "1s", "invalid"} {
		t.Setenv("WORKER_CLEANUP_LEAVE_INTERVAL", value)
		if _, err := CleanupConfigFromEnv(); err == nil {
			t.Fatalf("accepted unsafe leave interval %q", value)
		}
	}
}

func TestFailedWorkerOpenDoesNotBlockCleanup(t *testing.T) {
	h := newCleanupHarness(t, nil)
	h.worker(t, "healthy", "123")
	s, err := discordgo.New("Bot failed")
	if err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("gateway lookup failed")
	s.Client = &http.Client{Transport: cleanupTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || !strings.HasSuffix(r.URL.Path, "/gateway") {
			t.Errorf("unexpected gateway lookup: %s %s", r.Method, r.URL.Path)
		}
		return nil, wantErr
	})}
	if err := h.c.tp.openWorkerSession("failed", s); !errors.Is(err, wantErr) {
		t.Fatalf("open error = %v", err)
	}
	if _, tracked := h.c.tp.memberships["failed"]; tracked {
		t.Fatal("failed worker remains in inventory")
	}
	guilds, _, ready := h.c.tp.membershipSnapshot()
	if !ready || len(guilds["123"]) != 1 {
		t.Fatal("failed worker blocks healthy inventory")
	}
	h.step(t)
	if len(h.requests) != 1 {
		t.Fatal("failed worker prevented healthy worker cleanup")
	}
	// Late callbacks from the discarded session must not recreate an inventory or panic.
	h.c.tp.workerReady("failed", &discordgo.Ready{})
	h.c.tp.workerGuildCreate("failed", &discordgo.GuildCreate{Guild: &discordgo.Guild{ID: "123"}})
	h.c.tp.workerGuildDelete("failed", &discordgo.GuildDelete{Guild: &discordgo.Guild{ID: "123"}})
	if _, tracked := h.c.tp.memberships["failed"]; tracked {
		t.Fatal("late event recreated failed worker")
	}
}

func TestWorkerRoutingSkipsDepartedGuilds(t *testing.T) {
	h := newCleanupHarness(t, nil)
	h.worker(t, "a", "123", "456")
	sess := h.c.tp.activeSessions["a"]
	if !h.c.tp.tokenUsable("123", "a", sess) {
		t.Fatal("present worker not usable")
	}
	h.c.tp.workerGuildDelete("a", &discordgo.GuildDelete{Guild: &discordgo.Guild{ID: "123"}})
	if h.c.tp.tokenUsable("123", "a", sess) {
		t.Fatal("departed worker still usable")
	}
	if selected, _ := h.c.tp.getSession("123", nil); selected != nil {
		t.Fatal("routing selected departed worker")
	}
	if !h.c.tp.tokenUsable("456", "a", sess) {
		t.Fatal("departure affected another guild")
	}
	h.c.tp.workerGuildCreate("a", &discordgo.GuildCreate{Guild: &discordgo.Guild{ID: "123"}})
	if !h.c.tp.tokenUsable("123", "a", sess) {
		t.Fatal("reinvited worker not usable")
	}
	if len(h.requests) != 0 {
		t.Fatal("membership routing made Discord requests")
	}
}

func TestWorkerCleanupWithoutInventoryDoesNotEraseFleetSchedule(t *testing.T) {
	h := newCleanupHarness(t, nil)
	h.r.ZAdd(rediskey.WorkerCleanupSchedule, 1, "123")
	h.r.ZAdd(rediskey.WorkerCleanupPending, 1, "123")
	h.step(t)
	for _, key := range []string{rediskey.WorkerCleanupSchedule, rediskey.WorkerCleanupPending} {
		if n, err := h.c.tp.client.ZCard(context.Background(), key).Result(); err != nil || n != 1 {
			t.Fatalf("process without inventory modified %s: %d %v", key, n, err)
		}
	}
}

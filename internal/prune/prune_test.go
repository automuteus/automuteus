package prune

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis/v8"
)

const (
	emojiGuild = "100000000000000001"
	deadGuild  = "100000000000000002"
)

type fakeDiscord struct {
	guilds   map[string]Guild
	left     []string
	leaveErr error
}

func (f *fakeDiscord) GuildIDs() []string {
	ids := make([]string, 0, len(f.guilds))
	for id := range f.guilds {
		ids = append(ids, id)
	}
	return ids
}

func (f *fakeDiscord) Guild(id string) (Guild, bool) {
	g, ok := f.guilds[id]
	return g, ok
}

func (f *fakeDiscord) Leave(_ context.Context, id string) error {
	f.left = append(f.left, id)
	if f.leaveErr != nil {
		return f.leaveErr
	}
	delete(f.guilds, id)
	return nil
}

type fakeRecords struct {
	premium, played bool
	err             error
	since           time.Time
}

func (f *fakeRecords) HasPremium(context.Context, string) (bool, error) { return f.premium, f.err }

func (f *fakeRecords) PlayedSince(_ context.Context, _ string, since time.Time) (bool, error) {
	f.since = since
	return f.played, f.err
}

type harness struct {
	s       *Sweeper
	r       *miniredis.Miniredis
	discord *fakeDiscord
	records *fakeRecords
	now     time.Time
}

// newHarness is a process serving one server that nothing protects: joined two years ago, never played.
func newHarness(t *testing.T, r *miniredis.Miniredis) *harness {
	t.Helper()
	if r == nil {
		r = miniredis.RunT(t)
	}
	client := redis.NewClient(&redis.Options{Addr: r.Addr(), MaxRetries: -1})
	t.Cleanup(func() { client.Close() })
	h := &harness{r: r, records: &fakeRecords{}, now: time.Now()}
	h.discord = &fakeDiscord{guilds: map[string]Guild{deadGuild: {JoinedAt: h.now.Add(-2 * 365 * 24 * time.Hour)}}}
	h.s = &Sweeper{
		cfg: Config{InactiveAfter: 540 * 24 * time.Hour, CheckInterval: time.Second, LeaveInterval: 45 * time.Second,
			Keep: map[string]bool{emojiGuild: true}},
		redis: client, discord: h.discord, records: h.records,
		log: slog.New(slog.NewTextHandler(io.Discard, nil)), now: func() time.Time { return h.now },
	}
	return h
}

func (h *harness) step(t *testing.T) {
	t.Helper()
	if err := h.s.step(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestSweeperLeavesInactiveServer(t *testing.T) {
	h := newHarness(t, nil)
	h.step(t)
	if len(h.discord.left) != 1 || h.discord.left[0] != deadGuild {
		t.Fatalf("left %v, want %s", h.discord.left, deadGuild)
	}
	if want := h.now.Add(-540 * 24 * time.Hour); !h.records.since.Equal(want) {
		t.Errorf("asked for matches since %v, want %v", h.records.since, want)
	}
	if !h.r.Exists(rediskey.GuildPruned(deadGuild)) {
		t.Error("no marker to tell this departure from a removal")
	}
	if ttl := h.r.TTL(rediskey.GuildPruneLeaveBudget); ttl < 45*time.Second || ttl > time.Minute {
		t.Errorf("leave budget TTL = %v, want 45s to 60s", ttl)
	}
	if n, _ := h.r.Get(rediskey.GuildPruneLeftCount); n != "1" {
		t.Errorf("left count = %q, want 1", n)
	}
}

func TestSweeperKeepsProtectedServers(t *testing.T) {
	for _, cause := range []string{"kept", "joined_recently", "join_date_unknown", "unavailable", "recent_use", "active_game", "premium", "played"} {
		t.Run(cause, func(t *testing.T) {
			h := newHarness(t, nil)
			guild := h.discord.guilds[deadGuild]
			switch cause {
			case "kept":
				h.discord.guilds = map[string]Guild{emojiGuild: guild}
			case "joined_recently":
				guild.JoinedAt = h.now.Add(-Grace + time.Hour)
			case "join_date_unknown":
				guild.JoinedAt = time.Time{}
			case "unavailable":
				guild.Unavailable = true
			case "recent_use":
				h.r.Set(rediskey.GuildRecentUse(deadGuild), "1")
			case "active_game":
				h.r.ZAdd(rediskey.ActiveGamesForGuild(deadGuild), float64(h.now.Unix()), "GAME")
			case "premium":
				h.records.premium = true
			case "played":
				h.records.played = true
			}
			if cause != "kept" {
				h.discord.guilds[deadGuild] = guild
			}
			h.step(t)
			if len(h.discord.left) != 0 {
				t.Fatalf("left %v despite %s", h.discord.left, cause)
			}
			if h.r.Exists(rediskey.GuildPruneLeaveBudget) {
				t.Error("spent the leave budget without leaving")
			}
		})
	}
}

func TestSweeperStaleGameDoesNotProtect(t *testing.T) {
	h := newHarness(t, nil)
	h.r.ZAdd(rediskey.ActiveGamesForGuild(deadGuild), float64(h.now.Unix()-rediskey.ActiveGameTimeoutSeconds-1), "GAME")
	h.step(t)
	if len(h.discord.left) != 1 {
		t.Fatal("a game that timed out long ago kept the server")
	}
}

func TestSweeperDefersOnUncertainty(t *testing.T) {
	for _, cause := range []string{"database", "redis"} {
		t.Run(cause, func(t *testing.T) {
			h := newHarness(t, nil)
			if cause == "database" {
				h.records.err = errors.New("database unavailable")
			} else {
				h.r.SetError("unavailable")
			}
			if err := h.s.step(context.Background()); err == nil {
				t.Fatal("expected the failed lookup to be reported")
			}
			if len(h.discord.left) != 0 {
				t.Fatalf("left %v without knowing the server was inactive", h.discord.left)
			}
		})
	}
}

func TestSweeperDryRunOnlyReports(t *testing.T) {
	h := newHarness(t, nil)
	h.s.cfg.DryRun = true
	h.step(t)
	h.step(t) // a second pass finds the same server
	if len(h.discord.left) != 0 {
		t.Fatalf("dry run left %v", h.discord.left)
	}
	if members, _ := h.r.Members(rediskey.GuildPruneDryRun); len(members) != 1 || members[0] != deadGuild {
		t.Errorf("dry run recorded %v, want %s", members, deadGuild)
	}
	if h.r.Exists(rediskey.GuildPruneLeaveBudget) || h.r.Exists(rediskey.GuildPruned(deadGuild)) {
		t.Error("dry run wrote the keys of a real departure")
	}
}

// Two processes serve the same server, as twins do.
func TestSweeperFleetSharesOneBudget(t *testing.T) {
	one := newHarness(t, nil)
	two := newHarness(t, one.r)
	other := "100000000000000003"
	for _, h := range []*harness{one, two} {
		h.discord.guilds[other] = h.discord.guilds[deadGuild]
	}
	one.step(t)
	two.step(t)
	if n := len(one.discord.left) + len(two.discord.left); n != 1 {
		t.Fatalf("%d departures in one interval, want 1", n)
	}
	if two.records.since != (time.Time{}) {
		t.Error("checked a server while nothing could be left")
	}

	// The gateway tells the twin that the server is gone.
	delete(two.discord.guilds, one.discord.left[0])
	one.r.FastForward(2 * time.Minute)
	for i := 0; i < 3; i++ {
		two.step(t)
	}
	if len(two.discord.left) != 1 || two.discord.left[0] == one.discord.left[0] {
		t.Fatalf("twin left %v after the first process left %v", two.discord.left, one.discord.left)
	}
}

func TestSweeperTreatsServerAlreadyLeftAsDone(t *testing.T) {
	h := newHarness(t, nil)
	h.discord.leaveErr = &discordgo.RESTError{Message: &discordgo.APIErrorMessage{Code: discordgo.ErrCodeUnknownGuild}}
	h.step(t)
	if !h.r.Exists(rediskey.GuildPruned(deadGuild)) {
		t.Error("removed the marker the other process's departure is read against")
	}
	if h.r.Exists(rediskey.GuildPruneLeftCount) {
		t.Error("counted a departure another process made")
	}
}

func TestSweeperFailureSpendsBudgetAndMovesOn(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		minWait time.Duration
	}{
		{"server error", errors.New("HTTP 500"), 45 * time.Second},
		{"rate limited", &discordgo.RateLimitError{RateLimit: &discordgo.RateLimit{
			TooManyRequests: &discordgo.TooManyRequests{RetryAfter: 10 * time.Minute}}}, 10 * time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, nil)
			h.discord.leaveErr = tc.err
			if err := h.s.step(context.Background()); err == nil {
				t.Fatal("expected the failed departure to be reported")
			}
			h.step(t)
			if len(h.discord.left) != 1 {
				t.Fatalf("retried without pacing: %v", h.discord.left)
			}
			if h.r.Exists(rediskey.GuildPruned(deadGuild)) {
				t.Error("marker outlived a departure that did not happen")
			}
			if ttl := h.r.TTL(rediskey.GuildPruneLeaveBudget); ttl < tc.minWait {
				t.Errorf("leave budget TTL = %v, want at least %v", ttl, tc.minWait)
			}
		})
	}
}

func TestConfigFromEnv(t *testing.T) {
	for _, tc := range []struct {
		name    string
		env     map[string]string
		emoji   string
		wantErr bool
		check   func(*testing.T, Config)
	}{
		{name: "off by default", emoji: emojiGuild, check: func(t *testing.T, cfg Config) {
			if cfg.Enabled() {
				t.Error("pruning enabled with no PRUNE_INACTIVE_DAYS")
			}
		}},
		{name: "dry run unless told otherwise", env: map[string]string{"PRUNE_INACTIVE_DAYS": "540"}, emoji: emojiGuild,
			check: func(t *testing.T, cfg Config) {
				if !cfg.Enabled() || !cfg.DryRun || cfg.InactiveAfter != 540*24*time.Hour {
					t.Errorf("config = %+v", cfg)
				}
				if cfg.CheckInterval != 5*time.Second || cfg.LeaveInterval != 45*time.Second {
					t.Errorf("default pacing = %v, %v", cfg.CheckInterval, cfg.LeaveInterval)
				}
				if !cfg.Keep[emojiGuild] {
					t.Error("emoji server is not kept")
				}
			}},
		{name: "leaving for real", env: map[string]string{"PRUNE_INACTIVE_DAYS": "540", "PRUNE_DRY_RUN": "false",
			"PRUNE_KEEP_GUILD_IDS": deadGuild + ", 100000000000000003", "PRUNE_LEAVE_INTERVAL": "10s"}, emoji: emojiGuild,
			check: func(t *testing.T, cfg Config) {
				if cfg.DryRun || cfg.LeaveInterval != 10*time.Second || len(cfg.Keep) != 3 || !cfg.Keep[deadGuild] {
					t.Errorf("config = %+v", cfg)
				}
			}},
		{name: "shorter than the grace period", env: map[string]string{"PRUNE_INACTIVE_DAYS": "30"}, emoji: emojiGuild, wantErr: true},
		{name: "no emoji server", env: map[string]string{"PRUNE_INACTIVE_DAYS": "540"}, wantErr: true},
		{name: "bad kept server", env: map[string]string{"PRUNE_INACTIVE_DAYS": "540", "PRUNE_KEEP_GUILD_IDS": "abc"}, emoji: emojiGuild, wantErr: true},
		{name: "bad dry run", env: map[string]string{"PRUNE_INACTIVE_DAYS": "540", "PRUNE_DRY_RUN": "maybe"}, emoji: emojiGuild, wantErr: true},
		{name: "leaving too fast", env: map[string]string{"PRUNE_INACTIVE_DAYS": "540", "PRUNE_LEAVE_INTERVAL": "1s"}, emoji: emojiGuild, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, name := range []string{"PRUNE_INACTIVE_DAYS", "PRUNE_DRY_RUN", "PRUNE_KEEP_GUILD_IDS", "PRUNE_CHECK_INTERVAL", "PRUNE_LEAVE_INTERVAL"} {
				t.Setenv(name, tc.env[name])
			}
			cfg, err := ConfigFromEnv(tc.emoji)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, wantErr = %v", err, tc.wantErr)
			}
			if tc.check != nil {
				tc.check(t, cfg)
			}
		})
	}
}

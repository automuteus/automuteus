// Package prune leaves servers that stopped using the bot. Every server the bot is in costs each process serving
// it memory, whether or not anyone there still plays, and most servers that ever added the bot no longer do.
//
// Leaving loses nothing a returning server would miss: match history and premium stay in Postgres, and the bot
// keeps the server's settings when it is the one that left (see bot.leaveGuild). The server only has to invite the
// bot again.
package prune

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis/v8"
)

// Grace is how long a server is safe after the bot joins it, or after someone there starts a game with /new,
// whatever its match history says. It is what lets a server that comes back after being left settle in again.
const Grace = 90 * 24 * time.Hour

// prunedMarkerTTL outlasts the gateway's delivery of the bot's own departure to every process serving the server.
const prunedMarkerTTL = 10 * time.Minute

// dryRunTTL is how long a dry run's findings stay readable after the last one was added.
const dryRunTTL = 30 * 24 * time.Hour

type Config struct {
	// InactiveAfter is how long a server may go without a match before the bot leaves it. Zero disables pruning.
	InactiveAfter time.Duration
	// CheckInterval paces each process's own walk through its servers.
	CheckInterval time.Duration
	// LeaveInterval is the least time between two departures anywhere in the fleet.
	LeaveInterval time.Duration
	// DryRun records what would be left in rediskey.GuildPruneDryRun instead of leaving it.
	DryRun bool
	// Keep lists servers that are never left.
	Keep map[string]bool
}

func (cfg Config) Enabled() bool { return cfg.InactiveAfter > 0 }

// ConfigFromEnv reads the PRUNE_* variables. Pruning stays off unless PRUNE_INACTIVE_DAYS is set, and only reports
// what it would leave unless PRUNE_DRY_RUN is also set to false. emojiGuildID is always kept: the status emojis live
// there, and nobody plays in it.
func ConfigFromEnv(emojiGuildID string) (Config, error) {
	cfg := Config{CheckInterval: 5 * time.Second, LeaveInterval: 45 * time.Second, DryRun: true, Keep: map[string]bool{}}
	days := os.Getenv("PRUNE_INACTIVE_DAYS")
	if days == "" {
		return Config{}, nil
	}
	n, err := strconv.Atoi(days)
	if err != nil {
		return cfg, fmt.Errorf("PRUNE_INACTIVE_DAYS: %w", err)
	}
	cfg.InactiveAfter = time.Duration(n) * 24 * time.Hour
	if cfg.InactiveAfter < Grace {
		return cfg, fmt.Errorf("PRUNE_INACTIVE_DAYS must be at least %d", int(Grace/(24*time.Hour)))
	}
	if emojiGuildID == "" {
		// Without it the status emojis live in whichever server the bot saw first, which pruning could then leave.
		return cfg, errors.New("PRUNE_INACTIVE_DAYS requires EMOJI_GUILD_ID")
	}
	cfg.Keep[emojiGuildID] = true
	for _, id := range strings.Split(os.Getenv("PRUNE_KEEP_GUILD_IDS"), ",") {
		if id = strings.TrimSpace(id); id == "" {
			continue
		}
		if err := discord.ValidateSnowflake(id); err != nil {
			return cfg, fmt.Errorf("PRUNE_KEEP_GUILD_IDS: %q: %w", id, err)
		}
		cfg.Keep[id] = true
	}
	if v := os.Getenv("PRUNE_DRY_RUN"); v != "" {
		if cfg.DryRun, err = strconv.ParseBool(v); err != nil {
			return cfg, fmt.Errorf("PRUNE_DRY_RUN: %w", err)
		}
	}
	for name, dest := range map[string]*time.Duration{
		"PRUNE_CHECK_INTERVAL": &cfg.CheckInterval,
		"PRUNE_LEAVE_INTERVAL": &cfg.LeaveInterval,
	} {
		if v := os.Getenv(name); v != "" {
			if *dest, err = time.ParseDuration(v); err != nil {
				return cfg, fmt.Errorf("%s: %w", name, err)
			}
		}
	}
	if cfg.CheckInterval < time.Second || cfg.LeaveInterval < 5*time.Second {
		return cfg, errors.New("pruning requires PRUNE_CHECK_INTERVAL >= 1s and PRUNE_LEAVE_INTERVAL >= 5s")
	}
	return cfg, nil
}

// Guild is what the gateway last said about a server this process serves.
type Guild struct {
	// JoinedAt is when the bot joined. It is zero until the gateway has sent the server in full.
	JoinedAt    time.Time
	Unavailable bool
}

// Discord is this process's view of the servers it serves, and the means to leave one.
type Discord interface {
	GuildIDs() []string
	// Guild reports false once the bot is no longer in the server.
	Guild(id string) (Guild, bool)
	// Leave must not retry: a failed attempt has already spent the fleet's budget for this interval.
	Leave(ctx context.Context, id string) error
}

// Records answers from the database. Either may fail, and a failure never counts as inactivity.
type Records interface {
	HasPremium(ctx context.Context, guildID string) (bool, error)
	PlayedSince(ctx context.Context, guildID string, since time.Time) (bool, error)
}

// Sweeper walks one process's servers, one per tick, and leaves those nobody uses any more.
//
// Every process runs one. They coordinate only through the leave budget in Redis, which is enough: a process can
// leave only servers it serves, and when two processes serve the same server the second to try finds it gone.
type Sweeper struct {
	cfg     Config
	redis   *redis.Client
	discord Discord
	records Records
	log     *slog.Logger
	now     func() time.Time

	// queue is what remains of the current pass. The server at its head is examined next.
	queue  []string
	cancel context.CancelFunc
	done   chan struct{}
}

// Start begins sweeping in the background. cfg must be Enabled.
func Start(cfg Config, client *redis.Client, d Discord, records Records, logger *slog.Logger) *Sweeper {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Sweeper{cfg: cfg, redis: client, discord: d, records: records, log: logger, now: time.Now,
		cancel: cancel, done: make(chan struct{})}
	logger.Info("pruning inactive servers", "inactive_after", cfg.InactiveAfter, "dry_run", cfg.DryRun,
		"check_interval", cfg.CheckInterval, "leave_interval", cfg.LeaveInterval, "kept", len(cfg.Keep))
	go func() {
		defer close(s.done)
		// A ticker, so a restart neither bursts nor catches up on what it missed.
		ticker := time.NewTicker(cfg.CheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.step(ctx); err != nil && ctx.Err() == nil {
					logger.Warn("inactive server check failed; will retry on a later pass", "err", err)
				}
			}
		}
	}()
	return s
}

func (s *Sweeper) Stop() { s.cancel(); <-s.done }

// step examines the server at the head of the queue and, if nobody uses it, leaves it.
func (s *Sweeper) step(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	if !s.cfg.DryRun {
		// Nothing can be left until the budget frees, so finding candidates sooner would only cost queries.
		busy, err := s.redis.Exists(ctx, rediskey.GuildPruneLeaveBudget).Result()
		if err != nil || busy != 0 {
			return err
		}
	}
	if len(s.queue) == 0 {
		s.queue = s.discord.GuildIDs()
		// Two processes serving the same shards would otherwise examine the same server on every tick.
		rand.Shuffle(len(s.queue), func(i, j int) { s.queue[i], s.queue[j] = s.queue[j], s.queue[i] })
		if len(s.queue) == 0 {
			return nil
		}
	}
	guild := s.queue[0]
	inactive, err := s.inactive(ctx, guild)
	if err != nil || !inactive {
		// A failed check moves on too, so one server cannot stall the pass.
		s.queue = s.queue[1:]
		return err
	}
	if s.cfg.DryRun {
		s.queue = s.queue[1:]
		return s.report(ctx, guild)
	}
	// Reserve the fleet's budget before the request. A failed attempt spends it too.
	spacing := s.cfg.LeaveInterval + time.Duration(rand.Int64N(int64(s.cfg.LeaveInterval/3)))
	reserved, err := s.redis.SetNX(ctx, rediskey.GuildPruneLeaveBudget, "1", spacing).Result()
	if err != nil || !reserved {
		// Another process took this interval. The server stays at the head and is checked afresh in the next one.
		return err
	}
	s.queue = s.queue[1:]
	return s.leave(ctx, guild, spacing)
}

// inactive reports whether nothing protects the server: it is settled in, unused, and unpaid. Anything unknown
// protects it.
func (s *Sweeper) inactive(ctx context.Context, guild string) (bool, error) {
	if s.cfg.Keep[guild] {
		return false, nil
	}
	g, member := s.discord.Guild(guild)
	now := s.now()
	if !member || g.Unavailable || g.JoinedAt.IsZero() || now.Sub(g.JoinedAt) < Grace {
		return false, nil
	}
	used, err := s.redis.Exists(ctx, rediskey.GuildRecentUse(guild)).Result()
	if err != nil || used != 0 {
		return false, err
	}
	running, err := s.redis.ZCount(ctx, rediskey.ActiveGamesForGuild(guild),
		strconv.FormatInt(now.Unix()-rediskey.ActiveGameTimeoutSeconds, 10), "+inf").Result()
	if err != nil || running != 0 {
		return false, err
	}
	premium, err := s.records.HasPremium(ctx, guild)
	if err != nil || premium {
		return false, err
	}
	played, err := s.records.PlayedSince(ctx, guild, now.Add(-s.cfg.InactiveAfter))
	if err != nil || played {
		return false, err
	}
	return true, nil
}

// report records a server a dry run would have left. Each server is logged once, however many passes and processes
// find it.
func (s *Sweeper) report(ctx context.Context, guild string) error {
	added, err := s.redis.SAdd(ctx, rediskey.GuildPruneDryRun, guild).Result()
	if err != nil {
		return err
	}
	if added != 0 {
		g, _ := s.discord.Guild(guild)
		s.log.Info("would leave inactive server", "guild", guild, "joined", g.JoinedAt)
	}
	return s.redis.Expire(ctx, rediskey.GuildPruneDryRun, dryRunTTL).Err()
}

func (s *Sweeper) leave(ctx context.Context, guild string, spacing time.Duration) error {
	g, _ := s.discord.Guild(guild)
	if err := s.redis.Set(ctx, rediskey.GuildPruned(guild), "1", prunedMarkerTTL).Err(); err != nil {
		return err
	}
	err := s.discord.Leave(ctx, guild)
	var rest *discordgo.RESTError
	if errors.As(err, &rest) && rest.Message != nil && rest.Message.Code == discordgo.ErrCodeUnknownGuild {
		// Another process serving this server left it first. Its marker is the one the departure is read against.
		return nil
	}
	if err != nil {
		// The bot is still in the server, so being removed from it later must not look like a prune.
		s.redis.Del(ctx, rediskey.GuildPruned(guild))
		var limited *discordgo.RateLimitError
		if errors.As(err, &limited) {
			if wait := max(time.Minute, limited.RetryAfter); wait > spacing {
				s.redis.Set(ctx, rediskey.GuildPruneLeaveBudget, "1", wait)
			}
		}
		return fmt.Errorf("leave inactive server %s: %w", guild, err)
	}
	s.log.Info("left inactive server", "guild", guild, "joined", g.JoinedAt)
	return s.redis.Incr(ctx, rediskey.GuildPruneLeftCount).Err()
}

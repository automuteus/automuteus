package tokenprovider

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"strconv"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/bsm/redislock"
	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis/v8"
)

type CleanupConfig struct {
	CheckInterval time.Duration
	LeaveInterval time.Duration
}

func CleanupConfigFromEnv() (CleanupConfig, error) {
	cfg := CleanupConfig{CheckInterval: 5 * time.Second, LeaveInterval: 45 * time.Second}
	for name, dest := range map[string]*time.Duration{
		"WORKER_CLEANUP_CHECK_INTERVAL": &cfg.CheckInterval,
		"WORKER_CLEANUP_LEAVE_INTERVAL": &cfg.LeaveInterval,
	} {
		if value := os.Getenv(name); value != "" {
			d, err := time.ParseDuration(value)
			if err != nil {
				return cfg, fmt.Errorf("%s: %w", name, err)
			}
			*dest = d
		}
	}
	return cfg, cfg.validate()
}

func (cfg CleanupConfig) validate() error {
	if cfg.CheckInterval < time.Second || cfg.LeaveInterval < 30*time.Second {
		return errors.New("worker cleanup requires check interval >= 1s and leave interval >= 30s")
	}
	return nil
}

// PremiumLookup must report database failures, rather than interpreting them as FreeTier.
type PremiumLookup func(context.Context, string) (premium.Tier, int, error)

type workerCleanup struct {
	tp      *TokenProvider
	cfg     CleanupConfig
	premium PremiumLookup
	cancel  context.CancelFunc
	done    chan struct{}
}

// StartCleanup is called once after all configured worker sessions have been opened.
func (tp *TokenProvider) StartCleanup(cfg CleanupConfig, lookup PremiumLookup) error {
	if err := cfg.validate(); err != nil {
		return err
	}
	if tp.cleanup != nil {
		return errors.New("worker cleanup already started")
	}
	if lookup == nil || tp.client == nil {
		return errors.New("worker cleanup needs premium lookup and Redis")
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := &workerCleanup{tp: tp, cfg: cfg, premium: lookup, cancel: cancel, done: make(chan struct{})}
	tp.cleanup = c
	go func() {
		defer close(c.done)
		// No startup burst and no catch-up work after downtime.
		ticker := time.NewTicker(cfg.CheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := c.step(ctx); err != nil && ctx.Err() == nil {
					tp.metrics.RecordWorkerCleanup("failed")
					tp.logger("").Warn("worker cleanup failed; will retry on a later sweep", "err", err)
				}
			}
		}
	}()
	return nil
}

func (c *workerCleanup) stop() { c.cancel(); <-c.done }

func (c *workerCleanup) step(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	lease, err := redislock.New(c.tp.client).Obtain(ctx, rediskey.WorkerCleanupLease, 30*time.Second, nil)
	if errors.Is(err, redislock.ErrNotObtained) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() {
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), time.Second)
		defer releaseCancel()
		_ = lease.Release(releaseCtx)
	}()
	guilds, unavailable, ready := c.tp.membershipSnapshot()
	if !ready {
		c.tp.metrics.RecordWorkerCleanup("deferred")
		return nil
	}
	if err := c.syncInventory(ctx, guilds); err != nil {
		return err
	}
	allowed, err := c.tp.client.SetNX(ctx, rediskey.WorkerCleanupScanBudget, "1", c.cfg.CheckInterval).Result()
	if err != nil {
		return err
	}
	if allowed {
		if err := c.scan(ctx, guilds, unavailable); err != nil {
			return err
		}
	}
	if err := c.depart(ctx); err != nil {
		return err
	}
	return c.observe(ctx)
}

func (c *workerCleanup) syncInventory(ctx context.Context, guilds map[string][]string) error {
	allowed, err := c.tp.client.SetNX(ctx, rediskey.WorkerCleanupInventoryBudget, "1", time.Minute).Result()
	if err != nil || !allowed {
		return err
	}
	// Batch Redis writes; this does not issue any Discord requests.
	batch := make([]*redis.Z, 0, 500)
	for guild := range guilds {
		batch = append(batch, &redis.Z{Score: float64(time.Now().UnixMilli()), Member: guild})
		if len(batch) == cap(batch) {
			if err := c.tp.client.ZAddNX(ctx, rediskey.WorkerCleanupSchedule, batch...).Err(); err != nil {
				return err
			}
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		return c.tp.client.ZAddNX(ctx, rediskey.WorkerCleanupSchedule, batch...).Err()
	}
	return nil
}

func (c *workerCleanup) limit(ctx context.Context, guild string) (int, error) {
	tier, days, err := c.premium(ctx, guild)
	if err != nil {
		return 0, err
	}
	limit, ok := PremiumBotConstraints[tier]
	if !ok {
		return 0, fmt.Errorf("unknown premium tier %d for guild %s", tier, guild)
	}
	if premium.IsExpired(tier, days) {
		return 0, nil
	}
	return limit, nil
}

func (c *workerCleanup) scan(ctx context.Context, guilds map[string][]string, unavailable map[string]bool) error {
	oldest, err := c.tp.client.ZRange(ctx, rediskey.WorkerCleanupSchedule, 0, 0).Result()
	if err != nil || len(oldest) == 0 {
		return err
	}
	guild := oldest[0]
	if len(guilds[guild]) == 0 {
		if err := c.tp.client.ZRem(ctx, rediskey.WorkerCleanupSchedule, guild).Err(); err != nil {
			return err
		}
		return c.tp.client.ZRem(ctx, rediskey.WorkerCleanupPending, guild).Err()
	}
	// Rotate even failed/unavailable checks so one server cannot stall the sweep.
	if err := c.rotate(ctx, rediskey.WorkerCleanupSchedule, guild); err != nil {
		return err
	}
	if unavailable[guild] {
		c.tp.metrics.RecordWorkerCleanup("deferred")
		return nil
	}
	limit, err := c.limit(ctx, guild)
	if err != nil {
		return err
	}
	c.tp.metrics.RecordWorkerCleanup("checked")
	if len(guilds[guild]) > limit {
		return c.tp.client.ZAddNX(ctx, rediskey.WorkerCleanupPending, &redis.Z{Score: float64(time.Now().UnixMilli()), Member: guild}).Err()
	}
	return c.tp.client.ZRem(ctx, rediskey.WorkerCleanupPending, guild).Err()
}

func (c *workerCleanup) rotate(ctx context.Context, key, guild string) error {
	return c.tp.client.ZAdd(ctx, key, &redis.Z{Score: float64(time.Now().UnixMilli()), Member: guild}).Err()
}

func (c *workerCleanup) depart(ctx context.Context) error {
	busy, err := c.tp.client.Exists(ctx, rediskey.WorkerCleanupLeaveBudget).Result()
	if err != nil || busy != 0 {
		return err
	}
	queue, err := c.tp.client.ZRange(ctx, rediskey.WorkerCleanupPending, 0, 0).Result()
	if err != nil || len(queue) == 0 {
		return err
	}
	guild := queue[0]
	if err := c.rotate(ctx, rediskey.WorkerCleanupPending, guild); err != nil {
		return err
	}
	guilds, unavailable, ready := c.tp.membershipSnapshot()
	if !ready || unavailable[guild] {
		c.tp.metrics.RecordWorkerCleanup("deferred")
		return nil
	}
	workers := guilds[guild]
	if len(workers) == 0 {
		return c.tp.client.ZRem(ctx, rediskey.WorkerCleanupPending, guild).Err()
	}
	// Refresh entitlement at execution time: queued work must not outlive a renewal.
	limit, err := c.limit(ctx, guild)
	if err != nil {
		return err
	}
	if len(workers) <= limit {
		return c.tp.client.ZRem(ctx, rediskey.WorkerCleanupPending, guild).Err()
	}
	active, err := c.tp.client.ZCount(ctx, rediskey.ActiveGamesForGuild(guild),
		strconv.FormatInt(time.Now().Unix()-rediskey.ActiveGameTimeoutSeconds, 10), "+inf").Result()
	if err != nil {
		return err
	}
	if active > 0 {
		c.tp.metrics.RecordWorkerCleanup("deferred")
		return nil
	}
	// Retain the first N workers in stable token-hash order, regardless of recent usage.
	key := workers[len(workers)-1]
	if !c.tp.workerQuiet(key) {
		c.tp.metrics.RecordWorkerCleanup("deferred")
		return nil
	}
	paused, err := c.tp.client.Exists(ctx, rediskey.WorkerCleanupPause(key)).Result()
	if err != nil || paused > 0 {
		return err
	}
	c.tp.sessionLock.RLock()
	sess := c.tp.activeSessions[key]
	c.tp.sessionLock.RUnlock()
	if sess == nil {
		return nil
	}
	bucket := sess.Ratelimiter.GetBucket(discordgo.EndpointUserGuild("", guild))
	bucket.Lock()
	wait := sess.Ratelimiter.GetWaitTime(bucket, 1)
	bucket.Unlock()
	if wait > 0 {
		c.tp.metrics.RecordWorkerCleanup("deferred")
		return nil
	}
	// Reserve the fleet-wide budget BEFORE the HTTP call. Failures also spend it.
	spacing := c.cfg.LeaveInterval + time.Duration(rand.Int64N(int64(c.cfg.LeaveInterval/3)))
	allowed, err := c.tp.client.SetNX(ctx, rediskey.WorkerCleanupLeaveBudget, "1", spacing).Result()
	if err != nil || !allowed {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	err = sess.GuildLeave(guild, discordgo.WithContext(ctx), discordgo.WithRetryOnRatelimit(false), discordgo.WithRestRetries(0))
	var restError *discordgo.RESTError
	if errors.As(err, &restError) && restError.Message != nil && restError.Message.Code == discordgo.ErrCodeUnknownGuild {
		// Another process may have left while this gateway was catching up.
		err = nil
	}
	if err != nil {
		var rateLimit *discordgo.RateLimitError
		if errors.As(err, &rateLimit) {
			delay := max(time.Minute, rateLimit.RetryAfter)
			c.tp.pauseWorker(key, delay)
			if pauseErr := c.tp.extendWorkerPause(ctx, key, delay); pauseErr != nil {
				return pauseErr
			}
			c.tp.metrics.RecordWorkerCleanup("rate_limited")
		}
		return err
	}
	c.tp.workerGuildDelete(key, &discordgo.GuildDelete{Guild: &discordgo.Guild{ID: guild}})
	c.tp.metrics.RecordWorkerCleanup("left")
	c.tp.logger(guild).Info("worker left server above premium allowance", "token", key, "limit", limit)
	if len(workers)-1 <= limit {
		return c.tp.client.ZRem(ctx, rediskey.WorkerCleanupPending, guild).Err()
	}
	return nil
}

func (c *workerCleanup) observe(ctx context.Context) error {
	pending, err := c.tp.client.ZCard(ctx, rediskey.WorkerCleanupPending).Result()
	if err != nil {
		return err
	}
	oldest, err := c.tp.client.ZRangeWithScores(ctx, rediskey.WorkerCleanupSchedule, 0, 0).Result()
	if err != nil {
		return err
	}
	age := 0.0
	if len(oldest) > 0 {
		age = max(0, float64(time.Now().UnixMilli())/1000-oldest[0].Score/1000)
	}
	c.tp.metrics.SetWorkerCleanupStatus(float64(pending), age)
	return nil
}

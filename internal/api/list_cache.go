package api

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// DefaultListCacheTTL bounds how long a guild's role or channel list is reused before Discord is asked again. It is
// short because a picker should show a channel created moments ago, and long enough that a browser held on
// refresh, or many people opening the same guild's settings, cost Discord a handful of calls a minute.
const DefaultListCacheTTL = 30 * time.Second

// listCache remembers one kind of per-guild list from Discord for a short while and collapses concurrent fetches
// of the same guild into one. Errors are never cached; the verifier's rate-limit cooldown already makes a
// throttled route fail fast. Cached values are shared, so callers must not modify what they get back.
type listCache[T any] struct {
	ttl     time.Duration
	now     func() time.Time
	fetch   func(ctx context.Context, guildID string) (T, error)
	mu      sync.Mutex
	entries map[string]cachedList[T]
	flight  singleflight.Group
	// generation counts forget calls, so a fetch that started before one does not store what it read.
	generation uint64
}

type cachedList[T any] struct {
	value   T
	expires time.Time
}

func newListCache[T any](ttl time.Duration, now func() time.Time, fetch func(ctx context.Context, guildID string) (T, error)) *listCache[T] {
	if now == nil {
		now = time.Now
	}
	return &listCache[T]{ttl: ttl, now: now, fetch: fetch, entries: map[string]cachedList[T]{}}
}

func (c *listCache[T]) lookup(guildID string) (T, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[guildID]
	if !ok {
		var zero T
		return zero, false
	}
	if !c.now().Before(entry.expires) {
		delete(c.entries, guildID)
		var zero T
		return zero, false
	}
	return entry.value, true
}

func (c *listCache[T]) store(guildID string, value T, generation uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if generation != c.generation {
		return
	}
	c.entries[guildID] = cachedList[T]{value: value, expires: c.now().Add(c.ttl)}
	// Opportunistic sweep so an idle cache does not hold every guild ever seen.
	if len(c.entries) > 4096 {
		now := c.now()
		for k, e := range c.entries {
			if !now.Before(e.expires) {
				delete(c.entries, k)
			}
		}
	}
}

// get answers from the cache when it can, otherwise fetches once for all concurrent callers of the same guild.
// A non-positive TTL disables caching and collapsing alike.
func (c *listCache[T]) get(ctx context.Context, guildID string) (T, error) {
	if c == nil || c.ttl <= 0 {
		return c.fetch(ctx, guildID)
	}
	if value, ok := c.lookup(guildID); ok {
		return value, nil
	}
	result, err, _ := c.flight.Do(guildID, func() (interface{}, error) {
		if value, ok := c.lookup(guildID); ok {
			return value, nil
		}
		c.mu.Lock()
		generation := c.generation
		c.mu.Unlock()
		// The leader's deadline governs the shared call; a cancelled leader just makes everyone retry next time.
		value, err := c.fetch(ctx, guildID)
		if err != nil {
			return nil, err
		}
		c.store(guildID, value, generation)
		return value, nil
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return result.(T), nil
}

// forget drops every cached entry whose key matches, after the data behind them changed. A fetch already running
// still answers its callers, but what it read is not kept.
func (c *listCache[T]) forget(match func(key string) bool) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generation++
	for k := range c.entries {
		if match(k) {
			delete(c.entries, k)
		}
	}
}

// cachedRoleLister serves GET /guild/roles from a listCache. PATCH keeps the uncached lister so a role created a
// moment ago is accepted; that path already pays a live Discord verification per request.
type cachedRoleLister struct{ cache *listCache[[]GuildRole] }

func (c cachedRoleLister) ListRoles(ctx context.Context, guildID string) ([]GuildRole, error) {
	return c.cache.get(ctx, guildID)
}

// cachedChannelLister serves GET /guild/channels from a listCache.
type cachedChannelLister struct{ cache *listCache[[]GuildChannel] }

func (c cachedChannelLister) ListChannels(ctx context.Context, guildID string) ([]GuildChannel, error) {
	return c.cache.get(ctx, guildID)
}

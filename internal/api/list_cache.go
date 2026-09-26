package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// defaultBuildTimeout bounds a fetch that no caller is waiting for any more. It matches the request deadline, so
// a Discord list fetch is never kept alive longer than the request that wanted it would have been.
const defaultBuildTimeout = 10 * time.Second

// errStillBuilding is returned to a caller whose context ended while the entry was being fetched. The fetch keeps
// running on its own deadline and stores its result, so a retry a moment later is answered from the cache.
var errStillBuilding = errors.New("still building")

// DefaultListCacheTTL bounds how long a guild's role or channel list is reused before Discord is asked again. It is
// short because a picker should show a channel created moments ago, and long enough that a browser held on
// refresh, or many people opening the same guild's settings, cost Discord a handful of calls a minute.
const DefaultListCacheTTL = 30 * time.Second

// listCache remembers one kind of per-guild list from Discord for a short while and collapses concurrent fetches
// of the same guild into one. Errors are never cached; the verifier's rate-limit cooldown already makes a
// throttled route fail fast. Cached values are shared, so callers must not modify what they get back.
type listCache[T any] struct {
	ttl   time.Duration
	now   func() time.Time
	fetch func(ctx context.Context, guildID string) (T, error)
	// buildTimeout is the deadline of each fetch, which runs detached from the callers waiting on it.
	buildTimeout time.Duration
	mu           sync.Mutex
	entries      map[string]cachedList[T]
	flight       singleflight.Group
	// building is every fetch in flight, by key, so a forget that matches the key can mark what it will read
	// stale without touching the builds of other guilds. singleflight keeps it to one build per key.
	building map[string]*build
}

// build is one fetch in flight. stale is set by a forget that matched its key, so its result is answered to the
// callers waiting for it but not kept.
type build struct{ stale bool }

type cachedList[T any] struct {
	value   T
	expires time.Time
}

func newListCache[T any](ttl time.Duration, now func() time.Time, fetch func(ctx context.Context, guildID string) (T, error)) *listCache[T] {
	if now == nil {
		now = time.Now
	}
	return &listCache[T]{ttl: ttl, now: now, fetch: fetch, buildTimeout: defaultBuildTimeout, entries: map[string]cachedList[T]{}, building: map[string]*build{}}
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

// begin registers a fetch of the key, so forget can find it; finish deregisters it and keeps its value unless a
// forget matched the key in between.
func (c *listCache[T]) begin(guildID string) *build {
	c.mu.Lock()
	defer c.mu.Unlock()
	b := &build{}
	c.building[guildID] = b
	return b
}

func (c *listCache[T]) finish(guildID string, b *build, value T, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.building, guildID)
	if err != nil || b.stale {
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
// The fetch runs on its own deadline (buildTimeout), not the callers': a caller whose context ends first gets
// errStillBuilding, and the fetch carries on and stores its result for the next request. That is what lets a
// guild whose rollup takes longer than a request build at all. A non-positive TTL disables caching, collapsing,
// and detaching alike.
func (c *listCache[T]) get(ctx context.Context, guildID string) (T, error) {
	var zero T
	if c == nil || c.ttl <= 0 {
		return c.fetch(ctx, guildID)
	}
	if value, ok := c.lookup(guildID); ok {
		return value, nil
	}
	results := c.flight.DoChan(guildID, func() (result interface{}, err error) {
		// A panic in a fetch must become an error here: singleflight re-raises one from a DoChan fn on a goroutine
		// of its own, past any handler's recovery, and would take the whole process down for one bad guild.
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Guild %s fetch panicked: %v\n%s", guildID, r, debug.Stack())
				result, err = nil, fmt.Errorf("fetch panicked: %v", r)
			}
		}()
		if value, ok := c.lookup(guildID); ok {
			return value, nil
		}
		b := c.begin(guildID)
		buildCtx, cancel := context.WithTimeout(context.Background(), c.buildTimeout)
		defer cancel()
		value, err := c.fetch(buildCtx, guildID)
		c.finish(guildID, b, value, err)
		if err != nil {
			return nil, err
		}
		return value, nil
	})
	select {
	case result := <-results:
		if result.Err != nil {
			return zero, result.Err
		}
		return result.Val.(T), nil
	case <-ctx.Done():
		return zero, errStillBuilding
	}
}

// setBuildTimeout changes the deadline given to each fetch.
func (c *listCache[T]) setBuildTimeout(d time.Duration) {
	if c != nil {
		c.buildTimeout = d
	}
}

// forget drops every cached entry whose key matches, after the data behind them changed. A fetch already running
// for a matching key still answers its callers, but what it read is not kept. Fetches for other keys are left
// alone: a large guild's rollup, which outlives the requests that want it, must not be thrown away every time
// some other guild finishes a game.
func (c *listCache[T]) forget(match func(key string) bool) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, b := range c.building {
		if match(k) {
			b.stale = true
		}
	}
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

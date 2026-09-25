package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// DefaultAccessCacheTTL bounds how long a verified read authorization is reused before Discord is asked again.
const DefaultAccessCacheTTL = time.Minute

type cachedAccess struct {
	access  VerifiedGuildAccess
	expires time.Time
}

// accessCache remembers recent VerifyGuild answers per (token, guild) and collapses concurrent lookups for the
// same pair into one Discord round trip. It exists because a single settings page load asks about one guild
// several times at once, and Discord's per-user bucket for the guild list is small.
//
// Only read actions consult the cache; writes always verify live. Tokens are stored hashed. Errors are never
// cached, so a transient failure is retried by the next request.
type accessCache struct {
	ttl     time.Duration
	now     func() time.Time
	mu      sync.Mutex
	entries map[string]cachedAccess
	flight  singleflight.Group
}

func newAccessCache(ttl time.Duration, now func() time.Time) *accessCache {
	if now == nil {
		now = time.Now
	}
	return &accessCache{ttl: ttl, now: now, entries: map[string]cachedAccess{}}
}

func accessKey(token, guildID string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:]) + ":" + guildID
}

func (c *accessCache) lookup(key string) (VerifiedGuildAccess, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		return VerifiedGuildAccess{}, false
	}
	if !c.now().Before(entry.expires) {
		delete(c.entries, key)
		return VerifiedGuildAccess{}, false
	}
	return entry.access, true
}

func (c *accessCache) forget(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

func (c *accessCache) store(key string, access VerifiedGuildAccess) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cachedAccess{access: access, expires: c.now().Add(c.ttl)}
	// Opportunistic sweep so an idle cache does not hold every token ever seen.
	if len(c.entries) > 4096 {
		now := c.now()
		for k, e := range c.entries {
			if !now.Before(e.expires) {
				delete(c.entries, k)
			}
		}
	}
}

// verify answers for one request. With fresh set the cache is bypassed and the live answer replaces whatever was
// stored, so a write that follows a revocation is refused and later reads see the refusal too. A live check that
// finds the token invalid or unscoped also evicts the entry, whether the check was fresh or a cache miss.
func (c *accessCache) verify(ctx context.Context, verifier GuildVerifier, token, guildID string, fresh bool) (VerifiedGuildAccess, error) {
	if c == nil || c.ttl <= 0 {
		return verifier.VerifyGuild(ctx, token, guildID)
	}
	key := accessKey(token, guildID)
	if !fresh {
		if access, ok := c.lookup(key); ok {
			return access, nil
		}
	}
	result, err, _ := c.flight.Do(key, func() (interface{}, error) {
		if !fresh {
			if access, ok := c.lookup(key); ok {
				return access, nil
			}
		}
		// The leader's deadline governs the shared call; a follower cannot extend it, and a cancelled leader
		// simply makes everyone retry on their next request.
		access, err := verifier.VerifyGuild(ctx, token, guildID)
		if err != nil {
			// A dead or unscoped token is definitive: whatever was cached for it must not serve another read.
			// Outages are not, so the entry survives a transient failure.
			if errors.Is(err, errInvalidToken) || errors.Is(err, errDiscordForbidden) {
				c.forget(key)
			}
			return nil, err
		}
		c.store(key, access)
		return access, nil
	})
	if err != nil {
		return VerifiedGuildAccess{}, err
	}
	return result.(VerifiedGuildAccess), nil
}

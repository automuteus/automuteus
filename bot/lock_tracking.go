package bot

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/lock"
	"github.com/bsm/redislock"
)

// trackedLock wraps a redis lock to log when it is released after its TTL already expired. An expired lock means
// another holder may have read state mid-update and overwritten it, so these logs show which locks have TTLs
// that are too short for the work done under them.
type trackedLock struct {
	l        *redislock.Lock
	key      string
	ttl      time.Duration
	obtained time.Time
	// some callers release twice (a deferred Release plus SetDiscordGameState); the second release would otherwise
	// report ErrLockNotHeld and look like an expiry
	released atomic.Bool
}

func trackLock(l *redislock.Lock, key string, ttl time.Duration) lock.Lock {
	return &trackedLock{l: l, key: key, ttl: ttl, obtained: time.Now()}
}

func (t *trackedLock) Release(ctx context.Context) error {
	if t.released.Swap(true) {
		return nil
	}
	err := t.l.Release(ctx)
	if errors.Is(err, redislock.ErrLockNotHeld) {
		slog.Warn("lock expired before release",
			"key", t.key,
			"ttl_ms", t.ttl.Milliseconds(),
			"held_ms", time.Since(t.obtained).Milliseconds())
	}
	return err
}

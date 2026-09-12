// Package lock defines the minimal lock contract the bot relies on, so that callers can be exercised without a live
// Redis-backed lock implementation.
package lock

import "context"

// Lock is the subset of *redislock.Lock that the bot uses. A nil Lock means the lock could not be obtained.
type Lock interface {
	Release(ctx context.Context) error
}

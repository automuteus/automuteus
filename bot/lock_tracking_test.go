package bot

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bsm/redislock"
	"github.com/go-redis/redis/v8"
)

func captureSlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestTrackedLock_LogsExpiryOnlyOnce(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	buf := captureSlog(t)

	l, err := redislock.New(client).Obtain(ctx, "k:lock", 250*time.Millisecond, nil)
	if err != nil {
		t.Fatal(err)
	}
	tracked := trackLock(l, "k:lock", 250*time.Millisecond)
	mr.FastForward(time.Second)

	if err := tracked.Release(ctx); err == nil {
		t.Fatal("expected ErrLockNotHeld after expiry")
	}
	// a second release must not log again or report an error
	if err := tracked.Release(ctx); err != nil {
		t.Fatalf("second release: %v", err)
	}
	if n := strings.Count(buf.String(), "lock expired before release"); n != 1 {
		t.Fatalf("expected 1 expiry log, got %d:\n%s", n, buf.String())
	}
}

func TestTrackedLock_DoubleReleaseBeforeExpiryIsSilent(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	buf := captureSlog(t)

	l, err := redislock.New(client).Obtain(ctx, "k:lock", 250*time.Millisecond, nil)
	if err != nil {
		t.Fatal(err)
	}
	tracked := trackLock(l, "k:lock", 250*time.Millisecond)
	if err := tracked.Release(ctx); err != nil {
		t.Fatal(err)
	}
	if err := tracked.Release(ctx); err != nil {
		t.Fatalf("second release: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no logs, got:\n%s", buf.String())
	}
}

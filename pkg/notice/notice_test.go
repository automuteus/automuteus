package notice

import (
	"context"
	"encoding/json"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
)

func TestRaiseActiveClear(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	sub := Subscribe(ctx, client)
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	if err := Raise(ctx, client, Notice{Severity: "bogus", Message: "x"}, 0); err != ErrInvalid {
		t.Fatalf("invalid severity accepted: %v", err)
	}
	if err := Raise(ctx, client, Notice{Severity: Warning}, 0); err != ErrInvalid {
		t.Fatalf("empty message accepted: %v", err)
	}
	if n, err := Active(ctx, client); err != nil || n != nil {
		t.Fatalf("expected no active notice, got %+v, %v", n, err)
	}

	if err := Raise(ctx, client, Notice{Severity: Warning, Message: "db maintenance", Source: "test"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	got, err := Active(ctx, client)
	if err != nil || got == nil || got.Severity != Warning || got.Message != "db maintenance" || got.ExpiresAt == 0 {
		t.Fatalf("active = %+v, %v", got, err)
	}
	if ttl := mr.TTL(rediskey.ActiveNotice); ttl <= 0 || ttl > time.Minute {
		t.Fatalf("active notice ttl = %v, want about a minute", ttl)
	}
	msg := receive(t, sub)
	if msg.Severity != Warning || msg.Cleared {
		t.Fatalf("published = %+v", msg)
	}

	if err := Clear(ctx, client); err != nil {
		t.Fatal(err)
	}
	if n, _ := Active(ctx, client); n != nil {
		t.Fatalf("notice still active after clear: %+v", n)
	}
	if msg := receive(t, sub); !msg.Cleared {
		t.Fatalf("expected a cleared message, got %+v", msg)
	}
}

func TestRaiseWithoutTTLPersists(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	if err := Raise(ctx, client, Notice{Severity: Info, Message: "hello"}, 0); err != nil {
		t.Fatal(err)
	}
	if ttl := mr.TTL(rediskey.ActiveNotice); ttl != 0 {
		t.Fatalf("expected no expiry, got %v", ttl)
	}
	got, _ := Active(ctx, client)
	if got == nil || got.ExpiresAt != 0 {
		t.Fatalf("active = %+v", got)
	}
}

func TestActiveHonorsDeclaredExpiryBeforeRedisTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	ctx := context.Background()
	now := time.Now().Unix()
	for _, tc := range []struct {
		name      string
		severity  Severity
		expiresAt int64
		active    bool
	}{
		{"warning at deadline", Warning, now, false},
		{"critical at deadline", Critical, now, false},
		{"past deadline", Info, now - 1, false},
		{"future deadline", Warning, now + 60, true},
		{"no deadline", Critical, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(Notice{Severity: tc.severity, Message: "maintenance", ExpiresAt: tc.expiresAt})
			if err != nil {
				t.Fatal(err)
			}
			if err := client.Set(ctx, rediskey.ActiveNotice, b, time.Minute).Err(); err != nil {
				t.Fatal(err)
			}
			n, err := Active(ctx, client)
			if err != nil || (n != nil) != tc.active {
				t.Errorf("active = %+v, err = %v, want active = %v", n, err, tc.active)
			}
			if !mr.Exists(rediskey.ActiveNotice) {
				t.Fatal("test requires the Redis key to still exist")
			}
		})
	}
}

func receive(t *testing.T, sub *redis.PubSub) *Notice {
	t.Helper()
	select {
	case m := <-sub.Channel():
		n, err := Decode([]byte(m.Payload))
		if err != nil {
			t.Fatal(err)
		}
		return n
	case <-time.After(2 * time.Second):
		t.Fatal("no notice published")
	}
	return nil
}

func TestTargetedNoticeIsPublishedButNotStored(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	sub := Subscribe(ctx, client)
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	n := Notice{Severity: Critical, Message: "restarting", ConnectCodes: []string{"ABCDEFGH"}}
	if err := Raise(ctx, client, n, time.Minute); err != nil {
		t.Fatal(err)
	}
	if active, _ := Active(ctx, client); active != nil {
		t.Fatalf("targeted notice was stored as active: %+v", active)
	}
	got := receive(t, sub)
	if !got.Targeted() || !got.Targets("ABCDEFGH") || got.Targets("OTHER123") {
		t.Fatalf("published = %+v", got)
	}

	// an empty but non-nil list survives the round trip as targeted
	if err := Raise(ctx, client, Notice{Severity: Critical, Message: "x", ConnectCodes: []string{}}, 0); err != nil {
		t.Fatal(err)
	}
	if got := receive(t, sub); !got.Targeted() || got.Targets("ABCDEFGH") {
		t.Fatalf("empty targeted notice = %+v", got)
	}
	// and a platform-wide notice stays platform-wide
	if err := Raise(ctx, client, Notice{Severity: Warning, Message: "x"}, 0); err != nil {
		t.Fatal(err)
	}
	if got := receive(t, sub); got.Targeted() || !got.Targets("anything") {
		t.Fatalf("platform notice = %+v", got)
	}
}

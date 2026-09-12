package broker

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/automuteus/automuteus/v8/pkg/notice"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/go-redis/redis/v8"
)

func TestShutdown_WithdrawsCaptureReadyAndRaisesCriticalNotice(t *testing.T) {
	mr := miniredis.RunT(t)
	b := NewBroker(mr.Addr(), "", "")
	b.connections["sock-1"] = "ABCDEFGH"
	b.connections["sock-2"] = "IJKLMNOP"
	ctx := context.Background()
	for _, code := range []string{"ABCDEFGH", "IJKLMNOP"} {
		if err := b.client.Set(ctx, rediskey.CaptureMuteReady(code), "1", 0).Err(); err != nil {
			t.Fatal(err)
		}
	}
	observer := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	sub := notice.Subscribe(ctx, observer)
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	if b.Draining() {
		t.Fatal("broker should not be draining before Shutdown")
	}
	if err := b.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if !b.Draining() {
		t.Error("broker should refuse new clients once Shutdown has begun")
	}

	for _, code := range []string{"ABCDEFGH", "IJKLMNOP"} {
		if mr.Exists(rediskey.CaptureMuteReady(code)) {
			t.Errorf("capture-ready flag for %s still set", code)
		}
	}
	// a shutdown is an event about specific games, never a platform-wide notice that would block new games
	if active, err := notice.Active(ctx, observer); err != nil || active != nil {
		t.Fatalf("active notice = %+v, %v; a shutdown must not become a platform-wide notice", active, err)
	}
	msg := <-sub.Channel()
	published, err := notice.DecodeEvent([]byte(msg.Payload))
	if err != nil || published.Shutdown == nil || published.NoticeChanged {
		t.Fatalf("published = %+v, %v", published, err)
	}
	if codes := codeSet(published.Shutdown.ConnectCodes); len(codes) != 2 || !codes["ABCDEFGH"] || !codes["IJKLMNOP"] {
		t.Fatalf("shutdown should name exactly this broker's clients: %v", published.Shutdown.ConnectCodes)
	}
}

// TestShutdown_OnlyAffectsThisBrokersClients runs two brokers against one Redis, as replicas do in production, and
// checks that shutting one down neither withdraws the other's capture-ready flags nor names the other's games.
func TestShutdown_OnlyAffectsThisBrokersClients(t *testing.T) {
	mr := miniredis.RunT(t)
	ctx := context.Background()
	a := NewBroker(mr.Addr(), "", "")
	b := NewBroker(mr.Addr(), "", "")
	a.connections["sock-a"] = "AAAAAAAA"
	b.connections["sock-b1"] = "BBBBBBBB"
	b.connections["sock-b2"] = "CCCCCCCC"
	for _, code := range []string{"AAAAAAAA", "BBBBBBBB", "CCCCCCCC"} {
		if err := a.client.Set(ctx, rediskey.CaptureMuteReady(code), "1", 0).Err(); err != nil {
			t.Fatal(err)
		}
	}
	observer := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	sub := notice.Subscribe(ctx, observer)
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	if err := a.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}

	if mr.Exists(rediskey.CaptureMuteReady("AAAAAAAA")) {
		t.Error("shut-down broker's capture-ready flag still set")
	}
	for _, code := range []string{"BBBBBBBB", "CCCCCCCC"} {
		if !mr.Exists(rediskey.CaptureMuteReady(code)) {
			t.Errorf("other broker's capture-ready flag for %s was withdrawn", code)
		}
	}
	msg := <-sub.Channel()
	published, err := notice.DecodeEvent([]byte(msg.Payload))
	if err != nil || published.Shutdown == nil {
		t.Fatalf("published = %+v, %v", published, err)
	}
	if codes := codeSet(published.Shutdown.ConnectCodes); len(codes) != 1 || !codes["AAAAAAAA"] {
		t.Errorf("shutdown should name only the shut-down broker's game, got %v", published.Shutdown.ConnectCodes)
	}
	if b.Draining() {
		t.Error("the other broker should keep accepting clients")
	}
	select {
	case extra := <-sub.Channel():
		t.Fatalf("unexpected second notice: %s", extra.Payload)
	case <-time.After(100 * time.Millisecond):
	}
}

func codeSet(codes []string) map[string]bool {
	set := map[string]bool{}
	for _, c := range codes {
		set[c] = true
	}
	return set
}

package notice

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/go-redis/redis/v8"
)

func setup(t *testing.T) (*miniredis.Miniredis, *redis.Client, *redis.PubSub) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	sub := Subscribe(context.Background(), client)
	if _, err := sub.Receive(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sub.Close(); client.Close() })
	return mr, client, sub
}

func receive(t *testing.T, sub *redis.PubSub) *Event {
	t.Helper()
	select {
	case m := <-sub.Channel():
		e, err := DecodeEvent([]byte(m.Payload))
		if err != nil {
			t.Fatal(err)
		}
		return e
	case <-time.After(2 * time.Second):
		t.Fatal("no event published")
	}
	return nil
}

func TestRaiseActiveClear(t *testing.T) {
	mr, client, sub := setup(t)
	ctx := context.Background()

	for _, bad := range []Notice{{Severity: "info", Message: "x"}, {Severity: "loud", Message: "x"}, {Severity: Warning}} {
		if err := Raise(ctx, client, bad); err != ErrInvalid {
			t.Errorf("Raise(%+v) = %v, want ErrInvalid", bad, err)
		}
	}
	if n, err := Active(ctx, client); err != nil || n != nil {
		t.Fatalf("expected no active notice, got %+v, %v", n, err)
	}

	if err := Raise(ctx, client, Notice{Severity: Warning, Message: "db maintenance"}); err != nil {
		t.Fatal(err)
	}
	got, err := Active(ctx, client)
	if err != nil || got == nil || got.Severity != Warning || got.Message != "db maintenance" || got.IssuedAt == 0 {
		t.Fatalf("active = %+v, %v", got, err)
	}
	if ttl := mr.TTL(rediskey.ActiveNotice); ttl != 0 {
		t.Fatalf("notices must stay until cleared; ttl = %v", ttl)
	}
	if e := receive(t, sub); !e.NoticeChanged || e.Shutdown != nil {
		t.Fatalf("published = %+v", e)
	}

	// raising again replaces the active notice
	if err := Raise(ctx, client, Notice{Severity: Critical, Message: "going down"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := Active(ctx, client); got == nil || got.Severity != Critical {
		t.Fatalf("active after replace = %+v", got)
	}
	receive(t, sub)

	if err := Clear(ctx, client); err != nil {
		t.Fatal(err)
	}
	if n, _ := Active(ctx, client); n != nil {
		t.Fatalf("notice still active after clear: %+v", n)
	}
	if e := receive(t, sub); !e.NoticeChanged {
		t.Fatalf("expected a notice-changed event on clear, got %+v", e)
	}
}

func TestAnnounceShutdownIsPublishedButNeverStored(t *testing.T) {
	_, client, sub := setup(t)
	ctx := context.Background()

	if err := AnnounceShutdown(ctx, client, []string{"ABCDEFGH", "IJKLMNOP"}); err != nil {
		t.Fatal(err)
	}
	if active, _ := Active(ctx, client); active != nil {
		t.Fatalf("shutdown was stored as a notice: %+v", active)
	}
	e := receive(t, sub)
	if e.Shutdown == nil || len(e.Shutdown.ConnectCodes) != 2 || e.NoticeChanged {
		t.Fatalf("published = %+v", e)
	}

	// a replica with no clients still announces, with an empty (not null) list
	if err := AnnounceShutdown(ctx, client, nil); err != nil {
		t.Fatal(err)
	}
	if e := receive(t, sub); e.Shutdown == nil || e.Shutdown.ConnectCodes == nil || len(e.Shutdown.ConnectCodes) != 0 {
		t.Fatalf("empty shutdown = %+v", e)
	}
}

func TestDecodeEventRejectsMalformedEvents(t *testing.T) {
	for _, raw := range []string{`{}`, `{"noticeChanged":true,"shutdown":{"connectCodes":[]}}`, `{"noticeChanged":false}`} {
		if e, err := DecodeEvent([]byte(raw)); err != ErrInvalidEvent {
			t.Errorf("DecodeEvent(%s) = %+v, %v; want ErrInvalidEvent", raw, e, err)
		}
	}
	for _, raw := range []string{`{"noticeChanged":true}`, `{"shutdown":{"connectCodes":["ABCDEFGH"]}}`} {
		if _, err := DecodeEvent([]byte(raw)); err != nil {
			t.Errorf("DecodeEvent(%s): %v", raw, err)
		}
	}
}

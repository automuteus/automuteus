package api

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/automuteus/automuteus/v8/pkg/notice"
	"github.com/go-redis/redis/v8"
)

func TestDataStore_StatsChangesRelayAnnouncements(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewStore(client, nil, Config{})
	other := NewStore(client, nil, Config{})
	changes := store.StatsChanges(ctx)
	// give the subscription a moment to be registered before publishing
	deadline := time.Now().Add(2 * time.Second)
	for mr.PubSubNumSub("automuteus:stats:changed")["automuteus:stats:changed"] == 0 {
		if time.Now().After(deadline) {
			t.Fatal("subscription never registered")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// a replica's own announcement is skipped: it already forgot before announcing
	if err := store.AnnounceStatsChanged(ctx, "023456789012345678"); err != nil {
		t.Fatal(err)
	}
	if err := other.AnnounceStatsChanged(ctx, "123456789012345678"); err != nil {
		t.Fatal(err)
	}
	if err := notice.AnnounceStatsChanged(ctx, client); err != nil {
		t.Fatal(err)
	}
	// a malformed message is skipped, not fatal
	if err := client.Publish(ctx, "automuteus:stats:changed", "{").Err(); err != nil {
		t.Fatal(err)
	}
	if err := other.AnnounceStatsChanged(ctx, "223456789012345678"); err != nil {
		t.Fatal(err)
	}

	next := func() notice.StatsChanged {
		t.Helper()
		select {
		case c, ok := <-changes:
			if !ok {
				t.Fatal("channel closed early")
			}
			return c
		case <-time.After(2 * time.Second):
			t.Fatal("no change relayed")
		}
		return notice.StatsChanged{}
	}
	if c := next(); len(c.GuildIDs) != 1 || c.GuildIDs[0] != "123456789012345678" {
		t.Fatalf("first change = %+v", c)
	}
	if c := next(); !c.All() {
		t.Fatalf("second change = %+v, want every guild", c)
	}
	if c := next(); len(c.GuildIDs) != 1 || c.GuildIDs[0] != "223456789012345678" {
		t.Fatalf("third change = %+v", c)
	}

	cancel()
	select {
	case _, ok := <-changes:
		if ok {
			t.Fatal("change delivered after cancel")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("channel not closed after cancel")
	}
}

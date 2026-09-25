package notice

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
)

func TestAnnounceStatsChanged(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	sub := SubscribeStatsChanged(context.Background(), client)
	if _, err := sub.Receive(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sub.Close(); client.Close() })

	receive := func() *StatsChanged {
		t.Helper()
		select {
		case m := <-sub.Channel():
			c, err := DecodeStatsChanged([]byte(m.Payload))
			if err != nil {
				t.Fatal(err)
			}
			return c
		case <-time.After(2 * time.Second):
			t.Fatal("no announcement published")
		}
		return nil
	}

	if err := AnnounceStatsChanged(context.Background(), client, "123", "456"); err != nil {
		t.Fatal(err)
	}
	if c := receive(); c.All() || len(c.GuildIDs) != 2 || c.GuildIDs[0] != "123" || c.GuildIDs[1] != "456" {
		t.Fatalf("announcement = %+v, want guilds 123 and 456", c)
	}

	if err := AnnounceStatsChanged(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	if c := receive(); !c.All() {
		t.Fatalf("announcement = %+v, want every guild", c)
	}

	// the platform event channel is left alone, so shards never see stats traffic
	if got := mr.PubSubNumSub("automuteus:notices"); len(got) != 1 || got["automuteus:notices"] != 0 {
		t.Fatalf("notice channel subscribers = %v", got)
	}
}

func TestDecodeStatsChangedRejectsGarbage(t *testing.T) {
	if _, err := DecodeStatsChanged([]byte("{")); err == nil {
		t.Fatal("expected an error")
	}
}

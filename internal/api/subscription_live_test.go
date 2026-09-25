package api

import (
	"context"
	"os"
	"testing"
	"time"

	pgstorage "github.com/automuteus/automuteus/v8/pkg/storage"
	"github.com/automuteus/automuteus/v8/storage"
	"github.com/jackc/pgx/v4/pgxpool"
)

// TestLiveGuildSubscription checks the premium page's subscription lookup against the tables cmd/ipn writes: a
// guild's own subscription wins over an inherited one, a transferred guild shows none, and lapsed ones are ignored.
func TestLiveGuildSubscription(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("set disposable TEST_POSTGRES_URL")
	}
	ctx := context.Background()
	pool, err := pgxpool.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := storage.ApplySchemas(ctx, pool, false); err != nil {
		t.Fatal(err)
	}
	if err := storage.ApplyPaymentsSchema(ctx, pool); err != nil {
		t.Fatal(err)
	}
	const origin, sub, transferred, own, untracked = uint64(900000000000000031), uint64(900000000000000032), uint64(900000000000000033), uint64(900000000000000034), uint64(900000000000000035)
	all := []uint64{origin, sub, transferred, own, untracked}
	cleanup := func() {
		pool.Exec(ctx, "DELETE FROM premium_subscriptions WHERE guild_id = ANY($1)", all)
		pool.Exec(ctx, "UPDATE guilds SET inherits_from = NULL, transferred_to = NULL WHERE guild_id = ANY($1)", all)
		pool.Exec(ctx, "DELETE FROM guilds WHERE guild_id = ANY($1)", all)
	}
	cleanup()
	t.Cleanup(cleanup)
	now := time.Now()
	exec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	for _, g := range all {
		exec("INSERT INTO guilds (guild_id, guild_name, premium, tx_time_unix) VALUES ($1, 'sub test', 3, $2)", g, now.Unix())
	}
	exec("UPDATE guilds SET inherits_from = $2 WHERE guild_id = $1", sub, origin)
	exec("UPDATE guilds SET inherits_from = $2, transferred_to = $1 WHERE guild_id = $1", transferred, origin)
	exec("INSERT INTO premium_subscriptions (provider, external_id, guild_id, tier, status, last_payment_at) VALUES ('paypal', 'I-ORIGIN', $1, 3, 'active', $2)", origin, now.Add(-5*24*time.Hour).Unix())
	exec("INSERT INTO premium_subscriptions (provider, external_id, guild_id, tier, status, last_payment_at) VALUES ('paypal', 'I-OWN', $1, 1, 'cancelled', $2)", own, now.Add(-2*24*time.Hour).Unix())
	exec("INSERT INTO premium_subscriptions (provider, external_id, guild_id, tier, status, last_payment_at) VALUES ('paypal', 'I-OLD', $1, 3, 'active', $2)", untracked, now.Add(-40*24*time.Hour).Unix())
	exec("UPDATE guilds SET inherits_from = $2 WHERE guild_id = $1", own, origin)

	for _, tc := range []struct {
		guild     uint64
		want      *pgstorage.Subscription
		inherited bool
	}{
		{origin, &pgstorage.Subscription{Status: "active", Tier: 3}, false},
		{sub, &pgstorage.Subscription{Status: "active", Tier: 3}, true},
		// Its own subscription, even a lower cancelled one, before the origin's.
		{own, &pgstorage.Subscription{Status: "cancelled", Tier: 1}, false},
		{transferred, nil, false},
		{untracked, nil, false},
	} {
		got, err := pgstorage.GuildSubscription(ctx, pool, tc.guild, now)
		if err != nil {
			t.Fatalf("guild %d: %v", tc.guild, err)
		}
		if (got == nil) != (tc.want == nil) || got != nil && (got.Status != tc.want.Status || got.Tier != tc.want.Tier || got.Inherited != tc.inherited) {
			t.Errorf("guild %d: got %+v, want %+v inherited=%t", tc.guild, got, tc.want, tc.inherited)
		}
	}

	store := &DataStore{postgres: pool, stats: pool, config: Config{Official: true}}
	status, err := store.Subscription(ctx, "900000000000000032")
	if err != nil || status == nil || status.Status != "active" || !status.Inherited || status.EndsAt <= now.Unix() {
		t.Fatalf("store: %+v %v", status, err)
	}
	if status, err := (&DataStore{postgres: pool, stats: pool}).Subscription(ctx, "900000000000000032"); err != nil || status != nil {
		t.Fatalf("self-hosted store answered %+v %v", status, err)
	}
}

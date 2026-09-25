package ipn

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/automuteus/automuteus/v8/storage"
	"github.com/jackc/pgx/v4/pgxpool"
)

const receiver = "shop@example.com"

type fakeVerifier struct {
	mu     sync.Mutex
	answer bool
	err    error
	calls  int
}

func (f *fakeVerifier) Verify(context.Context, []byte) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.answer, f.err
}

// liveListener applies the schemas as the owner, then connects as a role holding only the grants payments.sql lists,
// so the listener is tested with the privileges it gets in production.
func liveListener(t *testing.T, now time.Time) (*Listener, *pgxpool.Pool, *fakeVerifier) {
	t.Helper()
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("set disposable TEST_POSTGRES_URL")
	}
	ctx := context.Background()
	owner, err := pgxpool.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(owner.Close)
	if err := storage.ApplySchemas(ctx, owner, false); err != nil {
		t.Fatal(err)
	}
	if err := storage.ApplyPaymentsSchema(ctx, owner); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		"TRUNCATE payment_events, premium_subscriptions, transactions",
		"DELETE FROM guilds WHERE guild_id >= 900000000000000100 AND guild_id < 900000000000000200",
		"DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'ipn_test_user') THEN CREATE ROLE ipn_test_user LOGIN PASSWORD 'ipn'; END IF; END $$",
	} {
		if _, err := owner.Exec(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	grants, err := os.ReadFile("../../storage/payments.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(grants), "\n") {
		if g := strings.TrimSpace(strings.TrimPrefix(line, "--")); strings.HasPrefix(g, "GRANT ") {
			if _, err := owner.Exec(ctx, strings.Replace(g, "TO ipn_user", "TO ipn_test_user", 1)); err != nil {
				t.Fatalf("%s: %v", g, err)
			}
		}
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.User, cfg.ConnConfig.Password = "ipn_test_user", "ipn"
	pool, err := pgxpool.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := CheckSchema(ctx, pool); err != nil {
		t.Fatal(err)
	}
	v := &fakeVerifier{answer: true}
	return &Listener{DB: pool, Verifier: v, Receiver: receiver, Now: func() time.Time { return now }}, owner, v
}

// notification builds a form body the way PayPal sends it.
func notification(track string, paid time.Time, kv ...string) []byte {
	v := url.Values{"ipn_track_id": {track}, "receiver_email": {receiver}, "business": {receiver},
		"mc_currency": {"USD"}, "charset": {"windows-1252"}, "payment_date": {paid.In(pacific).Format("15:04:05 Jan 02, 2006 MST")}}
	for i := 0; i < len(kv); i += 2 {
		v.Set(kv[i], kv[i+1])
	}
	return []byte(v.Encode())
}

type guildRow struct {
	premium int16
	txTime  *int32
}

func guild(t *testing.T, db *pgxpool.Pool, id string) *guildRow {
	t.Helper()
	var g guildRow
	err := db.QueryRow(context.Background(), "SELECT premium, tx_time_unix FROM guilds WHERE guild_id = $1::numeric", id).Scan(&g.premium, &g.txTime)
	if err != nil {
		t.Fatalf("guild %s: %v", id, err)
	}
	return &g
}

func expectGuild(t *testing.T, db *pgxpool.Pool, id string, tier int16, txTime int64) {
	t.Helper()
	g := guild(t, db, id)
	if g.premium != tier || g.txTime == nil || int64(*g.txTime) != txTime {
		var got interface{} = nil
		if g.txTime != nil {
			got = *g.txTime
		}
		t.Fatalf("guild %s: premium %d at %v, want %d at %d", id, g.premium, got, tier, txTime)
	}
}

func count(t *testing.T, db *pgxpool.Pool, query string, args ...interface{}) int {
	t.Helper()
	var n int
	if err := db.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func handle(t *testing.T, l *Listener, body []byte, want int) {
	t.Helper()
	if got := l.Handle(context.Background(), body); got != want {
		t.Fatalf("status %d, want %d for %s", got, want, body)
	}
}

func TestLiveSubscriptionLifecycle(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC).Truncate(time.Second)
	l, db, v := liveListener(t, now)
	const g = "900000000000000101" // no guild row yet: the old listener's UPDATE silently did nothing here
	first := now.Add(-25 * 24 * time.Hour)

	handle(t, l, notification("t1", first, "txn_type", "subscr_signup", "subscr_id", "I-A", "custom", g,
		"item_name", "AutoMuteUs Silver", "mc_amount3", "3.50"), http.StatusOK)
	if count(t, db, "SELECT count(*) FROM guilds WHERE guild_id = $1::numeric", g) != 0 {
		t.Fatal("a signup alone granted premium")
	}
	pay1 := notification("t2", first, "txn_type", "subscr_payment", "txn_id", "TX1", "subscr_id", "I-A", "custom", g,
		"item_name", "AutoMuteUs Silver", "mc_gross", "3.50", "payment_status", "Completed", "first_name", "Zo\xeb")
	handle(t, l, pay1, http.StatusOK)
	expectGuild(t, db, g, 2, first.Unix())

	// A retry of a processed notification changes nothing and is not re-verified.
	calls := v.calls
	handle(t, l, pay1, http.StatusOK)
	if v.calls != calls || count(t, db, "SELECT count(*) FROM transactions WHERE tx_id = 'TX1'") != 1 {
		t.Fatal("retry was reprocessed")
	}

	second := now.Add(-2 * 24 * time.Hour)
	handle(t, l, notification("t3", second, "txn_type", "subscr_payment", "txn_id", "TX2", "subscr_id", "I-A", "custom", g,
		"item_name", "AutoMuteUs Silver", "mc_gross", "3.50", "payment_status", "Completed"), http.StatusOK)
	expectGuild(t, db, g, 2, second.Unix())
	// An old payment delivered late does not move the date back.
	handle(t, l, notification("t2-late", first, "txn_type", "subscr_payment", "txn_id", "TX0", "subscr_id", "I-A", "custom", g,
		"item_name", "AutoMuteUs Silver", "mc_gross", "3.50", "payment_status", "Completed"), http.StatusOK)
	expectGuild(t, db, g, 2, second.Unix())

	handle(t, l, notification("t4", now, "txn_type", "subscr_cancel", "subscr_id", "I-A", "custom", g), http.StatusOK)
	handle(t, l, notification("t5", now, "txn_type", "subscr_eot", "subscr_id", "I-A", "custom", g), http.StatusOK)
	var status string
	db.QueryRow(context.Background(), "SELECT status FROM premium_subscriptions WHERE external_id = 'I-A'").Scan(&status)
	if status != "ended" {
		t.Fatalf("status %q", status)
	}
	// Ending never revokes: the last payment runs out on its own.
	expectGuild(t, db, g, 2, second.Unix())

	// windows-1252 names are stored as UTF-8, not the old listener's \u001a.
	var name string
	var gross float64
	var guildID string
	err := db.QueryRow(context.Background(), "SELECT tx->>'FirstName', gross, guild_id::text FROM transactions WHERE tx_id = 'TX1'").Scan(&name, &gross, &guildID)
	if err != nil || name != "Zoë" || gross != 3.5 || guildID != g {
		t.Fatalf("ledger row: %v %q %v %q", err, name, gross, guildID)
	}
	if n := count(t, db, "SELECT count(*) FROM payment_events WHERE processed_at IS NOT NULL"); n != 6 {
		t.Fatalf("%d processed events", n)
	}
}

func TestLiveKeepsBetterPremium(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC).Truncate(time.Second)
	l, db, _ := liveListener(t, now)
	ctx := context.Background()
	const legacy, manual, double = "900000000000000111", "900000000000000112", "900000000000000113"
	legacyPaid := now.Add(-5 * 24 * time.Hour).Unix()
	db.Exec(ctx, "INSERT INTO guilds (guild_id, guild_name, premium, tx_time_unix) VALUES ($1::numeric, 'x', 3, $2)", legacy, legacyPaid)
	db.Exec(ctx, "INSERT INTO guilds (guild_id, guild_name, premium, tx_time_unix) VALUES ($1::numeric, 'x', 4, NULL)", manual)

	pay := func(track, g, sub, txn, item string, paid time.Time) {
		t.Helper()
		handle(t, l, notification(track, paid, "txn_type", "subscr_payment", "txn_id", txn, "subscr_id", sub, "custom", g,
			"item_name", item, "payment_status", "Completed"), http.StatusOK)
	}
	// Gold paid before subscriptions were tracked outranks a new Silver until it runs out.
	pay("k1", legacy, "I-L", "TXL", "AutoMuteUs Silver", now.Add(-time.Hour))
	expectGuild(t, db, legacy, 3, legacyPaid)
	// A Trial with no expiry was granted by hand; a payment never shortens it.
	pay("k2", manual, "I-M", "TXM", "AutoMuteUs Gold", now.Add(-time.Hour))
	if g := guild(t, db, manual); g.premium != 4 || g.txTime != nil {
		t.Fatalf("manual grant changed: %+v", g)
	}

	// Two subscriptions for one server: the higher tier wins, and the lower takes over when it lapses.
	goldPaid, bronzePaid := now.Add(-20*24*time.Hour), now.Add(-2*24*time.Hour)
	pay("k3", double, "I-G", "TXG", "AutoMuteUs Gold", goldPaid)
	pay("k4", double, "I-B", "TXB", "AutoMuteUs Bronze", bronzePaid)
	expectGuild(t, db, double, 3, goldPaid.Unix())
	later := &Listener{DB: l.DB, Verifier: l.Verifier, Receiver: receiver, Now: func() time.Time { return now.Add(12 * 24 * time.Hour) }}
	if err := later.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	expectGuild(t, db, double, 1, bronzePaid.Unix())
	// And once everything has lapsed, reconciling leaves the row for the bot to expire.
	muchLater := &Listener{DB: l.DB, Verifier: l.Verifier, Receiver: receiver, Now: func() time.Time { return now.Add(60 * 24 * time.Hour) }}
	if err := muchLater.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	expectGuild(t, db, double, 1, bronzePaid.Unix())
}

func TestLiveGrantsNothing(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC).Truncate(time.Second)
	l, db, v := liveListener(t, now)
	ctx := context.Background()
	const g = "900000000000000121"
	paid := now.Add(-3 * 24 * time.Hour).Unix()
	db.Exec(ctx, "INSERT INTO guilds (guild_id, guild_name, premium, tx_time_unix) VALUES ($1::numeric, 'x', 3, $2)", g, paid)

	for _, body := range [][]byte{
		// A donation naming the server used to downgrade it to free.
		notification("n1", now, "txn_type", "web_accept", "txn_id", "D1", "custom", g, "item_name", "Donation", "mc_gross", "5.00", "payment_status", "Completed"),
		notification("n2", now, "txn_type", "subscr_payment", "txn_id", "D2", "subscr_id", "I-X", "custom", g, "item_name", "Mystery", "mc_gross", "9.99", "payment_status", "Completed"),
		notification("n3", now, "txn_type", "subscr_payment", "txn_id", "D3", "subscr_id", "I-Y", "custom", g, "item_name", "AutoMuteUs Gold", "payment_status", "Pending"),
		notification("n4", now, "txn_type", "subscr_payment", "txn_id", "D4", "subscr_id", "I-Z", "custom", g, "item_name", "AutoMuteUs Gold", "payment_status", "Completed", "receiver_email", "thief@example.com", "business", "thief@example.com"),
		notification("n5", now, "txn_type", "subscr_payment", "txn_id", "D5", "subscr_id", "I-W", "custom", g, "item_name", "AutoMuteUs Gold", "payment_status", "Completed", "mc_currency", "JPY"),
		notification("n6", now, "txn_id", "R1", "parent_txn_id", "D4", "custom", g, "mc_gross", "-5.50", "payment_status", "Refunded"),
		notification("n7", now, "txn_type", "subscr_payment", "txn_id", "D7", "subscr_id", "I-V", "custom", "not-a-guild", "item_name", "AutoMuteUs Gold", "payment_status", "Completed"),
	} {
		handle(t, l, body, http.StatusOK)
	}
	expectGuild(t, db, g, 3, paid)
	if n := count(t, db, "SELECT count(*) FROM premium_subscriptions"); n != 0 {
		t.Fatalf("%d subscriptions recorded", n)
	}
	// Every transaction still lands in the ledger, as before; unusable server IDs become NULL instead of failing.
	if n := count(t, db, "SELECT count(*) FROM transactions"); n != 7 {
		t.Fatalf("%d ledger rows", n)
	}
	if n := count(t, db, "SELECT count(*) FROM transactions WHERE tx_id = 'D7' AND guild_id IS NULL"); n != 1 {
		t.Fatal("invalid custom not stored as NULL")
	}
	if n := count(t, db, "SELECT count(*) FROM payment_events WHERE error IS NOT NULL AND processed_at IS NOT NULL"); n != 6 {
		t.Fatalf("%d events noted", n)
	}

	// Sandbox notifications on the live listener, INVALID ones, and unverifiable ones store nothing.
	events := count(t, db, "SELECT count(*) FROM payment_events")
	handle(t, l, notification("s1", now, "test_ipn", "1", "txn_id", "S1", "custom", g, "item_name", "AutoMuteUs Gold", "payment_status", "Completed"), http.StatusOK)
	v.answer = false
	handle(t, l, notification("s2", now, "txn_id", "S2", "custom", g, "item_name", "AutoMuteUs Gold", "payment_status", "Completed"), http.StatusOK)
	v.answer, v.err = true, errors.New("PayPal down")
	retry := notification("s3", now, "txn_id", "S3", "custom", g, "item_name", "AutoMuteUs Bronze", "payment_status", "Completed")
	handle(t, l, retry, http.StatusInternalServerError)
	if n := count(t, db, "SELECT count(*) FROM payment_events"); n != events {
		t.Fatalf("stored %d unverified events", n-events)
	}
	handle(t, l, []byte("%zz"), http.StatusBadRequest)
	// PayPal's retry once verification is back goes through.
	v.err = nil
	handle(t, l, retry, http.StatusOK)
	if n := count(t, db, "SELECT count(*) FROM transactions WHERE tx_id = 'S3'"); n != 1 {
		t.Fatal("retry not processed")
	}
}

func TestLiveConcurrentDeliveries(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC).Truncate(time.Second)
	l, db, _ := liveListener(t, now)
	const g = "900000000000000131"
	body := notification("c1", now, "txn_type", "subscr_payment", "txn_id", "C1", "subscr_id", "I-C", "custom", g,
		"item_name", "AutoMuteUs Gold", "payment_status", "Completed")
	var wg sync.WaitGroup
	statuses := make([]int, 8)
	for i := range statuses {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			statuses[i] = l.Handle(context.Background(), body)
		}(i)
	}
	wg.Wait()
	for _, s := range statuses {
		if s != http.StatusOK {
			t.Fatalf("statuses %v", statuses)
		}
	}
	expectGuild(t, db, g, 3, now.Unix())
	if n := count(t, db, "SELECT count(*) FROM transactions WHERE tx_id = 'C1'"); n != 1 {
		t.Fatalf("%d ledger rows", n)
	}
	if n := count(t, db, "SELECT count(*) FROM payment_events"); n != 1 {
		t.Fatalf("%d events", n)
	}
}

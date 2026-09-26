package ipn

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

const (
	provider = "paypal"
	// PayPal's bodies are a few KB; anything far larger is not from PayPal.
	maxBody = 64 << 10
	// A subscription grants its tier for this long after each payment, as the bot reads guilds.tx_time_unix.
	subPeriod = premium.SubDays * 24 * time.Hour
)

// Listener handles PayPal IPN deliveries. It answers 200 only once a notification's effects are committed, so PayPal
// keeps retrying (for up to four days) through database or verification outages; every other answer is safe to retry
// because processing is idempotent per notification.
type Listener struct {
	DB       *pgxpool.Pool
	Verifier Verifier
	// Receiver is the merchant email; payments to anyone else are recorded but grant nothing.
	Receiver string
	// Sandbox accepts only sandbox notifications, and live mode only live ones, so a sandbox payment can never grant
	// real premium.
	Sandbox bool
	Now     func() time.Time
	// Announce, when set, is told each guild whose premium changed, once the change is committed, so the API's
	// cached stats pages rebuild with the new tier instead of waiting out their TTL. Failures are logged only.
	Announce func(ctx context.Context, guildIDs ...string) error
}

func (l *Listener) now() time.Time {
	if l.Now != nil {
		return l.Now()
	}
	return time.Now()
}

func (l *Listener) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		http.Error(w, "unreadable body", http.StatusRequestEntityTooLarge)
		return
	}
	// PayPal gives up on a delivery after about 30 seconds and retries it later.
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	w.WriteHeader(l.Handle(ctx, body))
}

// Handle processes one delivery and returns the HTTP status to answer with.
func (l *Listener) Handle(ctx context.Context, body []byte) int {
	n, err := Parse(body)
	if err != nil {
		log.Printf("ipn: unparsable notification: %v", err)
		return http.StatusBadRequest
	}
	key := n.EventKey(body)
	var done bool
	err = l.DB.QueryRow(ctx, "SELECT processed_at IS NOT NULL FROM payment_events WHERE provider = $1 AND event_key = $2", provider, key).Scan(&done)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("ipn: look up %s: %v", key, err)
		return http.StatusInternalServerError
	}
	if done {
		return http.StatusOK
	}
	if n.Test() != l.Sandbox {
		log.Printf("ipn: ignoring %s: test_ipn=%t but sandbox=%t", key, n.Test(), l.Sandbox)
		return http.StatusOK
	}
	// Verify before storing anything, so only PayPal can write to the database.
	ok, err := l.Verifier.Verify(ctx, body)
	if err != nil {
		log.Printf("ipn: %s: %v", key, err)
		return http.StatusInternalServerError
	}
	if !ok {
		log.Printf("ipn: %s: PayPal says INVALID; ignoring", key)
		return http.StatusOK
	}
	id, err := l.record(ctx, key, body, n)
	if err != nil {
		log.Printf("ipn: record %s: %v", key, err)
		return http.StatusInternalServerError
	}
	if err := l.process(ctx, id, n); err != nil {
		log.Printf("ipn: event %d (%s): %v", id, n.Kind(), err)
		// The request context may be what failed; the error is still worth keeping.
		saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := l.DB.Exec(saveCtx, "UPDATE payment_events SET error = $2 WHERE event_id = $1", id, truncate(err.Error(), 1000)); err != nil {
			log.Printf("ipn: event %d: save error: %v", id, err)
		}
		return http.StatusInternalServerError
	}
	return http.StatusOK
}

// record stores the verified body before anything interprets it, so every genuine notification can be replayed.
func (l *Listener) record(ctx context.Context, key string, body []byte, n *Notification) (int64, error) {
	var guild interface{}
	if id, ok := n.GuildID(); ok {
		guild = strconv.FormatUint(id, 10)
	}
	var id int64
	err := l.DB.QueryRow(ctx, `INSERT INTO payment_events (provider, event_key, body, kind, txn_id, subscription, guild_id, verified)
VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), $7::numeric, true)
ON CONFLICT (provider, event_key) DO UPDATE SET attempts = payment_events.attempts + 1, verified = true
RETURNING event_id`,
		provider, key, body, truncate(n.Kind(), 64), truncate(n.Get("txn_id"), 64), truncate(n.Get("subscr_id"), 64), guild).Scan(&id)
	return id, err
}

// process applies a recorded notification in one transaction and marks it processed.
func (l *Listener) process(ctx context.Context, id int64, n *Notification) error {
	tx, err := l.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())

	// Serialises concurrent deliveries of the same notification.
	var done bool
	if err := tx.QueryRow(ctx, "SELECT processed_at IS NOT NULL FROM payment_events WHERE event_id = $1 FOR UPDATE", id).Scan(&done); err != nil {
		return err
	}
	if done {
		return tx.Commit(ctx)
	}
	note, changed, err := l.apply(ctx, tx, id, n)
	if err != nil {
		return err
	}
	var noteArg interface{}
	if note != "" {
		noteArg = truncate(note, 1000)
	}
	if _, err := tx.Exec(ctx, "UPDATE payment_events SET processed_at = now(), error = $2 WHERE event_id = $1", id, noteArg); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if changed != "" {
		l.announce(changed)
	}
	return nil
}

// announce tells the stats caches the guilds' premium changed. It runs on its own short deadline: the notification
// is already committed and PayPal must not be made to retry it because Redis was slow.
func (l *Listener) announce(guildIDs ...string) {
	if l.Announce == nil || len(guildIDs) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := l.Announce(ctx, guildIDs...); err != nil {
		log.Printf("ipn: announce premium change for %v: %v", guildIDs, err)
	}
}

// apply makes the notification's changes and returns a note for payment_events.error when it did not do what the
// notification asked (a payment that granted nothing, say), and the guild whose premium row it changed, if any.
func (l *Listener) apply(ctx context.Context, tx pgx.Tx, id int64, n *Notification) (note, changed string, err error) {
	now := l.now()
	paid := n.PaymentTime(now)
	txnID := n.Get("txn_id")
	// The ledger has always recorded every verified transaction, whether or not it granted anything.
	if txnID != "" {
		if err := writeLedger(ctx, tx, n, paid); err != nil {
			return "", "", fmt.Errorf("ledger: %w", err)
		}
	}
	if reason := l.reject(n); reason != "" {
		log.Printf("ipn: event %d (%s, txn %s): %s; recorded only", id, n.Kind(), txnID, reason)
		return reason, "", nil
	}
	guild, hasGuild := n.GuildID()
	tier := n.Tier()
	sub := n.Get("subscr_id")
	status := n.Get("payment_status")

	switch {
	case n.Kind() == "subscr_signup":
		if !hasGuild || tier == premium.FreeTier || sub == "" {
			return "signup without a server, tier, or subscription ID", "", nil
		}
		// A signup only records the subscription; premium starts with its first payment.
		_, err := tx.Exec(ctx, `INSERT INTO premium_subscriptions (provider, external_id, guild_id, tier, status)
VALUES ($1, $2, $3::numeric, $4, 'active') ON CONFLICT (provider, external_id) DO UPDATE SET updated_at = now()`,
			provider, truncate(sub, 64), strconv.FormatUint(guild, 10), int16(tier))
		return "", "", err

	case n.Kind() == "subscr_cancel":
		// Cancelled subscriptions stay paid up until PayPal sends subscr_eot at the end of the term.
		_, err := tx.Exec(ctx, `UPDATE premium_subscriptions SET status = 'cancelled', cancelled_at = now(), updated_at = now()
WHERE provider = $1 AND external_id = $2 AND status = 'active'`, provider, sub)
		log.Printf("ipn: event %d: subscription %s cancelled", id, sub)
		return "", "", err

	case n.Kind() == "subscr_eot":
		var guildID *string
		err := tx.QueryRow(ctx, `UPDATE premium_subscriptions SET status = 'ended', ended_at = now(), updated_at = now()
WHERE provider = $1 AND external_id = $2 RETURNING guild_id::text`, provider, sub).Scan(&guildID)
		if errors.Is(err, pgx.ErrNoRows) {
			// A subscription from before this listener; its last payment simply runs out.
			return "", "", nil
		}
		if err != nil {
			return "", "", err
		}
		log.Printf("ipn: event %d: subscription %s ended", id, sub)
		changed, err := recomputed(ctx, tx, *guildID, now)
		return "", changed, err

	case txnID != "" && status == "Completed":
		if !hasGuild || tier == premium.FreeTier {
			return "payment for no server or no premium tier; ledger only", "", nil
		}
		// A payment outside any subscription still buys one period, as it always has; it is tracked as a
		// subscription that will not renew.
		external, subStatus := sub, "active"
		if external == "" {
			external, subStatus = "txn:"+txnID, "cancelled"
		}
		var guildID string
		err := tx.QueryRow(ctx, `INSERT INTO premium_subscriptions (provider, external_id, guild_id, tier, status, last_payment_at)
VALUES ($1, $2, $3::numeric, $4, $5, $6)
ON CONFLICT (provider, external_id) DO UPDATE SET
	last_payment_at = GREATEST(premium_subscriptions.last_payment_at, EXCLUDED.last_payment_at), updated_at = now()
RETURNING guild_id::text`,
			provider, truncate(external, 64), strconv.FormatUint(guild, 10), int16(tier), subStatus, int32(paid.Unix())).Scan(&guildID)
		if err != nil {
			return "", "", err
		}
		log.Printf("ipn: event %d: txn %s pays %s for guild %s", id, txnID, premium.TierStrings[tier], guildID)
		changed, err := recomputed(ctx, tx, guildID, now)
		return "", changed, err

	case status == "Refunded" || status == "Reversed":
		// Deliberately not automatic: a refund may be partial, or for a payment that was already replaced.
		log.Printf("ipn: event %d: txn %s (parent %s) %s; review premium for guild %s by hand",
			id, txnID, n.Get("parent_txn_id"), status, n.Get("custom"))
		return "refund or reversal: review by hand", "", nil
	}
	return "", "", nil
}

// recomputed is recompute answering with the guild ID when its premium row changed, for apply's return.
func recomputed(ctx context.Context, tx pgx.Tx, guildID string, now time.Time) (string, error) {
	changed, err := recompute(ctx, tx, guildID, now)
	if err != nil || !changed {
		return "", err
	}
	return guildID, nil
}

// reject says why a verified notification must not change premium, or "".
func (l *Listener) reject(n *Notification) string {
	if !strings.EqualFold(n.Get("receiver_email"), l.Receiver) && !strings.EqualFold(n.Get("business"), l.Receiver) {
		return "sent to a different receiver"
	}
	if c := n.Get("mc_currency"); c != "" && !strings.EqualFold(c, "USD") {
		return "currency " + c
	}
	return ""
}

// writeLedger keeps transactions at one row per PayPal transaction, updated as the transaction's status changes. The
// table has no unique key, so the row is updated or inserted under a lock on its ID rather than upserted.
func writeLedger(ctx context.Context, tx pgx.Tx, n *Notification, paid time.Time) error {
	body, err := n.ledgerJSON(paid)
	if err != nil {
		return err
	}
	txnID := truncate(n.Get("txn_id"), 64)
	var guild interface{}
	if id, ok := n.GuildID(); ok {
		guild = strconv.FormatUint(id, 10)
	}
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext('transactions:' || $1))", txnID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, "UPDATE transactions SET tx_time = $2, gross = $3, tx = $4, guild_id = $5::numeric WHERE tx_id = $1",
		txnID, int32(paid.Unix()), n.Gross(), body, guild)
	if err != nil || tag.RowsAffected() > 0 {
		return err
	}
	_, err = tx.Exec(ctx, "INSERT INTO transactions (tx_id, tx_time, gross, tx, guild_id) VALUES ($1, $2, $3, $4, $5::numeric)",
		txnID, int32(paid.Unix()), n.Gross(), body, guild)
	return err
}

type candidate struct {
	tier    premium.Tier
	paidAt  int64
	tracked bool
}

// rank orders tiers by what they unlock; Trial sits below the paid tiers so buying one replaces it.
func rank(t premium.Tier) int {
	switch t {
	case premium.TrialTier:
		return 1
	case premium.BronzeTier:
		return 2
	case premium.SilverTier:
		return 3
	case premium.GoldTier:
		return 4
	case premium.SelfHostTier:
		return 5
	}
	return 0
}

// recompute sets a guild's premium and tx_time_unix to its best paid-up subscription. Premium the listener does not
// track (granted by hand, or paid before subscriptions were recorded) is kept while it lasts, and a guild whose
// subscriptions have all lapsed is left alone for the bot's own expiry to handle, so this never takes premium away.
// It reports whether the guild's row changed.
func recompute(ctx context.Context, tx pgx.Tx, guildID string, now time.Time) (bool, error) {
	if _, err := tx.Exec(ctx, "INSERT INTO guilds (guild_id, guild_name, premium) VALUES ($1::numeric, '', 0) ON CONFLICT (guild_id) DO NOTHING", guildID); err != nil {
		return false, err
	}
	var current int16
	var txTime *int32
	if err := tx.QueryRow(ctx, "SELECT premium, tx_time_unix FROM guilds WHERE guild_id = $1::numeric FOR UPDATE", guildID).Scan(&current, &txTime); err != nil {
		return false, err
	}
	if current != 0 && txTime == nil {
		// Premium with no expiry is only ever granted by hand; never shorten it to a subscription period.
		return false, nil
	}
	cutoff := now.Add(-subPeriod).Unix()
	rows, err := tx.Query(ctx, `SELECT tier, last_payment_at FROM premium_subscriptions
WHERE guild_id = $1::numeric AND status IN ('active', 'cancelled') AND last_payment_at > $2`, guildID, cutoff)
	if err != nil {
		return false, err
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		var tier int16
		var paidAt int32
		if err := rows.Scan(&tier, &paidAt); err != nil {
			rows.Close()
			return false, err
		}
		c.tier, c.paidAt, c.tracked = premium.Tier(tier), int64(paidAt), true
		candidates = append(candidates, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return false, err
	}
	if current != 0 && int64(*txTime) > cutoff {
		// The guild's current premium counts unless it is a tracked subscription's payment, which is already a
		// candidate if it still should be.
		var ours bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM premium_subscriptions WHERE guild_id = $1::numeric AND last_payment_at = $2)",
			guildID, *txTime).Scan(&ours); err != nil {
			return false, err
		}
		if !ours {
			candidates = append(candidates, candidate{tier: premium.Tier(current), paidAt: int64(*txTime)})
		}
	}
	var best *candidate
	for i := range candidates {
		c := &candidates[i]
		if best == nil || rank(c.tier) > rank(best.tier) || (rank(c.tier) == rank(best.tier) && c.paidAt > best.paidAt) {
			best = c
		}
	}
	if best == nil || !best.tracked {
		return false, nil
	}
	if int16(best.tier) == current && txTime != nil && int64(*txTime) == best.paidAt {
		return false, nil
	}
	_, err = tx.Exec(ctx, "UPDATE guilds SET premium = $2, tx_time_unix = $3 WHERE guild_id = $1::numeric", guildID, int16(best.tier), int32(best.paidAt))
	if err != nil {
		return false, err
	}
	log.Printf("ipn: guild %s now %s from %d", guildID, premium.TierStrings[best.tier], best.paidAt)
	return true, nil
}

// Reconcile recomputes every guild with a subscription paid within the last period, so a guild falls back to its
// next-best subscription when a better one lapses without PayPal saying so.
func (l *Listener) Reconcile(ctx context.Context) error {
	now := l.now()
	rows, err := l.DB.Query(ctx, "SELECT DISTINCT guild_id::text FROM premium_subscriptions WHERE last_payment_at > $1",
		now.Add(-subPeriod-24*time.Hour).Unix())
	if err != nil {
		return err
	}
	var guilds []string
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			rows.Close()
			return err
		}
		guilds = append(guilds, g)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	var errs []error
	var changed []string
	for _, g := range guilds {
		var did bool
		err := l.DB.BeginFunc(ctx, func(tx pgx.Tx) (err error) {
			did, err = recompute(ctx, tx, g, now)
			return err
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("guild %s: %w", g, err))
		} else if did {
			changed = append(changed, g)
		}
	}
	l.announce(changed...)
	return errors.Join(errs...)
}

// CheckSchema fails when payments.sql has not been applied or the role lacks the grants it lists.
func CheckSchema(ctx context.Context, db *pgxpool.Pool) error {
	var ok bool
	err := db.QueryRow(ctx, `SELECT has_table_privilege('payment_events', 'SELECT, INSERT, UPDATE')
	AND has_sequence_privilege('payment_events_event_id_seq', 'USAGE')
	AND has_table_privilege('premium_subscriptions', 'SELECT, INSERT, UPDATE')
	AND has_table_privilege('transactions', 'SELECT, INSERT, UPDATE')
	AND has_table_privilege('guilds', 'SELECT, INSERT')
	AND has_column_privilege('guilds', 'premium', 'UPDATE')
	AND has_column_privilege('guilds', 'tx_time_unix', 'UPDATE')`).Scan(&ok)
	if err != nil {
		return fmt.Errorf("payment tables missing (apply storage/payments.sql): %w", err)
	}
	if !ok {
		return errors.New("missing grants on the payment tables; see the end of storage/payments.sql")
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.ToValidUTF8(s[:n], "")
}

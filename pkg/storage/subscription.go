package storage

import (
	"context"
	"errors"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/georgysavva/scany/pgxscan"
	"github.com/jackc/pgx/v4"
)

// Subscription is what the payment listener (cmd/ipn) knows about the subscription paying for a guild's premium.
type Subscription struct {
	// Inherited is set when the subscription belongs to the guild this one inherits premium from.
	Inherited bool `db:"inherited"`
	// Status is active (renews each period) or cancelled (paid up until it ends, then stops).
	Status        string       `db:"status"`
	Tier          premium.Tier `db:"tier"`
	LastPaymentAt int64        `db:"last_payment_at"`
}

// EndsAt is when the current paid period runs out.
func (s Subscription) EndsAt() time.Time {
	return time.Unix(s.LastPaymentAt, 0).Add(premium.SubDays * 24 * time.Hour)
}

// The guild's own subscription comes first, then the one it inherits from; among several the same order cmd/ipn
// uses to pick the tier written to guilds. The transferred_to check mirrors the bot: a transferred guild is free.
var guildSubscriptionQuery = "SELECT s.guild_id <> $1 AS inherited, s.status, s.tier, s.last_payment_at " +
	"FROM premium_subscriptions s " +
	"WHERE s.status IN ('active', 'cancelled') AND s.last_payment_at > $2 " +
	"AND (s.guild_id = $1 OR s.guild_id = (SELECT inherits_from FROM guilds WHERE guild_id = $1 AND transferred_to IS NULL)) " +
	"ORDER BY s.guild_id = $1 DESC, s.tier DESC, s.last_payment_at DESC LIMIT 1"

// GuildSubscription returns the paid-up subscription behind a guild's premium, or nil when there is none tracked.
// Premium from before the listener existed, or granted by hand, has no subscription row.
func GuildSubscription(ctx context.Context, q pgxscan.Querier, guildID uint64, now time.Time) (*Subscription, error) {
	var s Subscription
	cutoff := now.Add(-premium.SubDays * 24 * time.Hour).Unix()
	err := pgxscan.Get(ctx, q, &s, guildSubscriptionQuery, guildID, cutoff)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

package storage

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/georgysavva/scany/pgxscan"
)

// GetGuildPremiumStatus preserves errors for decisions such as worker eviction.
// The legacy game path can continue falling back to free-tier routing on failures.
func (psqlInterface *PsqlInterface) GetGuildPremiumStatus(ctx context.Context, official bool, guildID string) (premium.Tier, int, error) {
	if !official {
		return premium.SelfHostTier, premium.NoExpiryCode, nil
	}
	conn, err := psqlInterface.Pool.Acquire(ctx)
	if err != nil {
		return premium.FreeTier, 0, err
	}
	defer conn.Release()
	return checkedGuildPremiumStatus(ctx, conn.Conn(), guildID, 0)
}

func checkedGuildPremiumStatus(ctx context.Context, conn PgxIface, guildID string, depth int) (premium.Tier, int, error) {
	if depth > 3 {
		return premium.FreeTier, 0, fmt.Errorf("premium inheritance exceeds depth limit for %s", guildID)
	}
	gid, err := strconv.ParseUint(guildID, 10, 64)
	if err != nil {
		return premium.FreeTier, 0, err
	}
	var guild PostgresGuild
	err = pgxscan.Get(ctx, conn, &guild, "SELECT "+guildColumns+" FROM guilds WHERE guild_id = $1", gid)
	if err != nil {
		// Workers can be invited to guilds the primary bot has never seen.
		// A missing inheritance source is an inconsistent entitlement; defer cleanup.
		if depth == 0 && pgxscan.NotFound(err) {
			return premium.FreeTier, 0, nil
		}
		return premium.FreeTier, 0, err
	}
	if depth == 0 && guild.TransferredTo != nil {
		return premium.FreeTier, 0, nil
	}
	days := premium.NoExpiryCode
	if guild.TxTimeUnix != nil {
		days = int(premium.SubDays - (time.Now().Unix()-int64(*guild.TxTimeUnix))/SecsInADay)
		if days > 0 {
			return premium.Tier(guild.Premium), days, nil
		}
	}
	if guild.InheritsFrom != nil {
		return checkedGuildPremiumStatus(ctx, conn, strconv.FormatUint(*guild.InheritsFrom, 10), depth+1)
	}
	return premium.Tier(guild.Premium), days, nil
}

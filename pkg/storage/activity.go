package storage

import (
	"context"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

// GuildPlayedSince reports whether a match started in the guild at or after since.
func (psqlInterface *PsqlInterface) GuildPlayedSince(ctx context.Context, guildID string, since time.Time) (bool, error) {
	return guildPlayedSince(ctx, psqlInterface.Pool, guildID, since)
}

func guildPlayedSince(ctx context.Context, db interface {
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}, guildID string, since time.Time) (bool, error) {
	gid, err := strconv.ParseUint(guildID, 10, 64)
	if err != nil {
		return false, err
	}
	var played bool
	// EXISTS, not MAX or ORDER BY ... LIMIT: those tempt the planner into walking games_pkey backwards looking for
	// this guild, which reads the whole table for a guild that never played.
	err = db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM games WHERE guild_id = $1 AND start_time >= $2)", gid, since.Unix()).Scan(&played)
	return played, err
}

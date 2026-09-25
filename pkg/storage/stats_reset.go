package storage

import (
	"context"

	"github.com/jackc/pgconn"
)

// Execer is the part of a pool or transaction the stats resets need.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
}

// ResetGuildStats deletes every recorded game of a guild; the players' rows and the game events go with them by
// cascade. It returns how many games were deleted.
func ResetGuildStats(ctx context.Context, db Execer, guildID string) (int64, error) {
	tag, err := db.Exec(ctx, "DELETE FROM games WHERE guild_id = $1", guildID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ResetGuildUserStats removes one player from every recorded game of one guild, and nothing in any other guild.
// The games themselves and their events stay, so the other players' stats are untouched. It returns how many
// games the player was removed from.
func ResetGuildUserStats(ctx context.Context, db Execer, guildID, userID string) (int64, error) {
	tag, err := db.Exec(ctx, "DELETE FROM users_games WHERE guild_id = $1 AND user_id = $2", guildID, userID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

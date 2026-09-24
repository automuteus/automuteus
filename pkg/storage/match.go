package storage

import (
	"context"
	"errors"

	"github.com/automuteus/automuteus/v8/pkg/capture"
	"github.com/georgysavva/scany/pgxscan"
	"github.com/jackc/pgx/v4"
)

// The single-match reads below back the HTTP API's match summary page. Like the guild statistics, they take a
// context and any pgxscan.Querier and return errors rather than logging them.

var (
	// The guild is part of the lookup so a match ID from one guild cannot be read through another.
	guildMatchQuery = "SELECT game_id, guild_id, connect_code, start_time, win_type, end_time, play_map, region " +
		"FROM games WHERE game_id = $1 AND guild_id = $2"

	matchPlayersQuery = "SELECT user_id, guild_id, game_id, player_name, player_color, player_role, player_won " +
		"FROM users_games WHERE game_id = $1 ORDER BY player_role DESC, player_color, user_id"

	// Phase changes and player updates make up the timeline and the game over report completes the roster; lobby
	// events are not needed. The payload is read as text so it scans the same whatever JSON value was stored.
	matchEventsQuery = "SELECT event_id, user_id, game_id, event_time, event_type, payload::text AS payload " +
		"FROM game_events WHERE game_id = $1 AND event_type IN ($2, $3, $4) ORDER BY event_time, event_id"
)

// GuildMatch returns one match of a guild, or nil if the guild has no match with that ID.
func GuildMatch(ctx context.Context, q pgxscan.Querier, guildID uint64, matchID int64) (*PostgresGame, error) {
	var g PostgresGame
	err := pgxscan.Get(ctx, q, &g, guildMatchQuery, matchID, guildID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// MatchPlayers lists the linked players recorded for a match, impostors first. Players who were not linked, or
// opted out, are never recorded, and nothing is recorded until the match ends.
func MatchPlayers(ctx context.Context, q pgxscan.Querier, matchID int64) ([]*PostgresUserGame, error) {
	var r []*PostgresUserGame
	err := pgxscan.Select(ctx, q, &r, matchPlayersQuery, matchID)
	return r, err
}

// MatchEvents lists the phase, player, and game over events of a match in the order they happened.
func MatchEvents(ctx context.Context, q pgxscan.Querier, matchID int64) ([]*PostgresGameEvent, error) {
	var r []*PostgresGameEvent
	err := pgxscan.Select(ctx, q, &r, matchEventsQuery, matchID, int16(capture.State), int16(capture.Player), int16(capture.GameOver))
	return r, err
}

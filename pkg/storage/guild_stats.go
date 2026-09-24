package storage

import (
	"context"
	"strconv"
	"strings"

	"github.com/automuteus/automuteus/v8/pkg/game"
	"github.com/georgysavva/scany/pgxscan"
)

// The guild-wide statistics below back the HTTP API's stats page. Unlike the older PsqlInterface rankings used by
// the /stats slash command, they take a context, return errors instead of logging them, and push the leaderboard
// minimum and size into SQL so a page load never scans every player of a guild only to keep the top few.
// They accept any pgxscan.Querier so a pool, a connection, or a mock can serve them.

// GuildSummary counts the finished games of a guild and how many each side won. Aborted and still-running games
// are excluded, as they are from the slash command.
type GuildSummary struct {
	GamesPlayed  int64 `db:"games"`
	CrewmateWins int64 `db:"crewmate_wins"`
	ImpostorWins int64 `db:"impostor_wins"`
}

// GuildPlayerGames is one row of the most-games leaderboard.
type GuildPlayerGames struct {
	UserID uint64 `db:"user_id"`
	Count  int64  `db:"total"`
}

// AnyRole asks GuildWinRanking to count games in either role.
const AnyRole int16 = -1

var (
	crewmateWinTypes = winTypeList(game.HumansByVote, game.HumansByTask, game.HumansDisconnect)
	impostorWinTypes = winTypeList(game.ImpostorByVote, game.ImpostorByKill, game.ImpostorBySabotage, game.ImpostorDisconnect)

	guildSummaryQuery = "SELECT COUNT(*) AS games, " +
		"COUNT(*) FILTER (WHERE win_type IN (" + crewmateWinTypes + ")) AS crewmate_wins, " +
		"COUNT(*) FILTER (WHERE win_type IN (" + impostorWinTypes + ")) AS impostor_wins " +
		"FROM games WHERE guild_id = $1 AND end_time <> -1 AND win_type <> $2"

	guildMostGamesQuery = "SELECT user_id, COUNT(*) AS total FROM users_games WHERE guild_id = $1 " +
		"GROUP BY user_id ORDER BY total DESC, user_id LIMIT $2"

	// A negative role matches every row, so one statement serves the overall and per-role boards.
	guildWinRankingQuery = "SELECT user_id, " +
		"COUNT(*) FILTER (WHERE player_won) AS win, " +
		"COUNT(*) AS total, " +
		"COUNT(*) FILTER (WHERE player_won)::decimal / COUNT(*) * 100 AS win_rate " +
		"FROM users_games WHERE guild_id = $1 AND ($2 < 0 OR player_role = $2) " +
		"GROUP BY user_id HAVING COUNT(*) >= $3 " +
		"ORDER BY win_rate DESC, win DESC, total DESC, user_id LIMIT $4"

	// Pairs are keyed with the lower user ID first so each duo appears once. Both players share a role and a
	// game, so they share the outcome; counting a's wins is counting the duo's wins.
	guildDuoRankingSelect = "SELECT a.user_id, b.user_id AS teammate_id, " +
		"COUNT(*) AS total, " +
		"COUNT(*) FILTER (WHERE a.player_won) AS win, " +
		"COUNT(*) FILTER (WHERE a.player_won)::decimal / COUNT(*) * 100 AS win_rate " +
		"FROM users_games a " +
		"INNER JOIN users_games b ON b.game_id = a.game_id AND b.user_id > a.user_id AND b.player_role = $2 " +
		"WHERE a.guild_id = $1 AND a.player_role = $2 " +
		"GROUP BY a.user_id, b.user_id HAVING COUNT(*) >= $3 "
	guildBestDuoQuery  = guildDuoRankingSelect + "ORDER BY win_rate DESC, win DESC, total DESC, a.user_id, b.user_id LIMIT $4"
	guildWorstDuoQuery = guildDuoRankingSelect + "ORDER BY win_rate ASC, win ASC, total DESC, a.user_id, b.user_id LIMIT $4"

	// The first death event of each finished game names the first target. A player's rate is measured against
	// their crewmate games, since only crewmates can be killed. The earliest death is picked before anyone is
	// excluded, and then only counts if that player still has a crewmate record for the game: when the first
	// victim was unlinked or opted out (a null user), or has since reset their stats (their users_games rows
	// are deleted but the events stay), the game simply has no first target, rather than the next victim being
	// credited or a deleted history being counted against retained games. Aborted games keep their events but
	// record no players, so they drop out here too.
	guildFirstTargetQuery = "SELECT ug.user_id, COUNT(*) AS total_death, t.total, " +
		"COUNT(*)::decimal / t.total * 100 AS death_rate " +
		"FROM games g " +
		"JOIN LATERAL (SELECT game_events.user_id FROM game_events " +
		"WHERE game_events.game_id = g.game_id AND payload ->> 'Action' = $2 " +
		"ORDER BY event_time, event_id FETCH FIRST 1 ROW ONLY) ge ON TRUE " +
		"JOIN users_games ug ON ug.game_id = g.game_id AND ug.user_id = ge.user_id AND ug.player_role = " + crewmateRole + " " +
		"JOIN LATERAL (SELECT COUNT(*) AS total FROM users_games " +
		"WHERE users_games.user_id = ug.user_id AND users_games.guild_id = $1 AND users_games.player_role = " + crewmateRole + ") t ON TRUE " +
		"WHERE g.guild_id = $1 AND g.end_time <> -1 AND g.win_type <> " + abortedResult + " " +
		"GROUP BY ug.user_id, t.total HAVING t.total >= $3 " +
		"ORDER BY death_rate DESC, total_death DESC, ug.user_id LIMIT $4"

	// The game never reports who made a kill, so a crewmate's death counts against every impostor of that game.
	// encounter is how many games the pair shared in those roles.
	guildKilledByQuery = "SELECT c.user_id, i.user_id AS teammate_id, " +
		"COUNT(*) FILTER (WHERE d.game_id IS NOT NULL) AS total_death, " +
		"COUNT(*) AS encounter, " +
		"COUNT(*) FILTER (WHERE d.game_id IS NOT NULL)::decimal / COUNT(*) * 100 AS death_rate " +
		"FROM users_games c " +
		"INNER JOIN users_games i ON i.game_id = c.game_id AND i.player_role = " + impostorRole + " " +
		"LEFT JOIN LATERAL (SELECT game_events.game_id FROM game_events " +
		"WHERE game_events.game_id = c.game_id AND game_events.user_id = c.user_id AND payload ->> 'Action' = $2 " +
		"FETCH FIRST 1 ROW ONLY) d ON TRUE " +
		"WHERE c.guild_id = $1 AND c.player_role = " + crewmateRole + " " +
		"GROUP BY c.user_id, i.user_id HAVING COUNT(*) >= $3 " +
		"ORDER BY death_rate DESC, total_death DESC, encounter DESC, c.user_id, i.user_id LIMIT $4"
)

const (
	crewmateRole = "0"
	impostorRole = "1"
)

var (
	diedAction    = strconv.Itoa(int(game.DIED))
	abortedResult = strconv.Itoa(int(game.Aborted))
)

func winTypeList(types ...game.GameResult) string {
	parts := make([]string, len(types))
	for i, t := range types {
		parts[i] = strconv.Itoa(int(t))
	}
	return strings.Join(parts, ",")
}

// GuildSummaryStats counts a guild's finished games and the wins of each side.
func GuildSummaryStats(ctx context.Context, q pgxscan.Querier, guildID uint64) (GuildSummary, error) {
	var s GuildSummary
	err := pgxscan.Get(ctx, q, &s, guildSummaryQuery, guildID, int16(game.Aborted))
	return s, err
}

// GuildMostGames ranks the guild's linked players by games recorded.
func GuildMostGames(ctx context.Context, q pgxscan.Querier, guildID uint64, limit int) ([]*GuildPlayerGames, error) {
	r := []*GuildPlayerGames{}
	err := pgxscan.Select(ctx, q, &r, guildMostGamesQuery, guildID, limit)
	return r, err
}

// GuildWinRanking ranks players by winrate in the given role (or AnyRole), keeping only those with at least
// minGames games in it.
func GuildWinRanking(ctx context.Context, q pgxscan.Querier, guildID uint64, role int16, minGames, limit int) ([]*PostgresPlayerRanking, error) {
	r := []*PostgresPlayerRanking{}
	err := pgxscan.Select(ctx, q, &r, guildWinRankingQuery, guildID, role, minGames, limit)
	return r, err
}

// GuildDuoRanking ranks pairs of players who shared a role in at least minGames games, best winrate first, or
// worst first when worst is set.
func GuildDuoRanking(ctx context.Context, q pgxscan.Querier, guildID uint64, role game.GameRole, minGames, limit int, worst bool) ([]*PostgresBestTeammatePlayerRanking, error) {
	query := guildBestDuoQuery
	if worst {
		query = guildWorstDuoQuery
	}
	r := []*PostgresBestTeammatePlayerRanking{}
	err := pgxscan.Select(ctx, q, &r, query, guildID, int16(role), minGames, limit)
	return r, err
}

// GuildFirstTargetRanking ranks players by how often they were the first to die, among those with at least
// minGames games as a crewmate.
func GuildFirstTargetRanking(ctx context.Context, q pgxscan.Querier, guildID uint64, minGames, limit int) ([]*PostgresUserMostFrequentFirstTargetRanking, error) {
	r := []*PostgresUserMostFrequentFirstTargetRanking{}
	err := pgxscan.Select(ctx, q, &r, guildFirstTargetQuery, guildID, diedAction, minGames, limit)
	return r, err
}

// GuildKilledByRanking ranks crewmate/impostor pairs by how often the crewmate died with that impostor in the
// game, among pairs that shared at least minGames games in those roles.
func GuildKilledByRanking(ctx context.Context, q pgxscan.Querier, guildID uint64, minGames, limit int) ([]*PostgresUserMostFrequentKilledByanking, error) {
	r := []*PostgresUserMostFrequentKilledByanking{}
	err := pgxscan.Select(ctx, q, &r, guildKilledByQuery, guildID, diedAction, minGames, limit)
	return r, err
}

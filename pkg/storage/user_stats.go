package storage

import (
	"context"
	"errors"
	"strconv"

	"github.com/automuteus/automuteus/v8/pkg/game"
	"github.com/georgysavva/scany/pgxscan"
)

// The per-player statistics below back the HTTP API's player stats page. They follow the guild statistics'
// conventions (context, returned errors, any pgxscan.Querier) and count the same way the guild boards do, so a
// player's numbers on their own page match the rows they appear in on the guild page.

// UserSummary is a player's record in one guild, by role. FirstGame and LastGame are start times, nil when the
// player has no games.
type UserSummary struct {
	Games         int64  `db:"games"`
	Wins          int64  `db:"wins"`
	CrewmateGames int64  `db:"crewmate_games"`
	CrewmateWins  int64  `db:"crewmate_wins"`
	ImpostorGames int64  `db:"impostor_games"`
	ImpostorWins  int64  `db:"impostor_wins"`
	FirstGame     *int64 `db:"first_game"`
	LastGame      *int64 `db:"last_game"`
}

// UserMatch is one of a player's matches, with how they played it.
type UserMatch struct {
	GameID      int64  `db:"game_id"`
	StartTime   int32  `db:"start_time"`
	EndTime     int32  `db:"end_time"`
	WinType     int16  `db:"win_type"`
	PlayMap     *int16 `db:"play_map"`
	PlayerName  string `db:"player_name"`
	PlayerColor int16  `db:"player_color"`
	PlayerRole  int16  `db:"player_role"`
	PlayerWon   bool   `db:"player_won"`
}

// UserFates is how a player's games in one role ended for them. Died and Exiled count games, not events, so a
// capture that resent a death does not count it twice; the *Won counts are the games of each kind the player's
// side still won. FirstDeaths is how many games they were the first to die in, and Eliminated how many they
// were killed or voted out in.
type UserFates struct {
	Role        int16 `db:"role"`
	Games       int64 `db:"games"`
	Wins        int64 `db:"wins"`
	Died        int64 `db:"died"`
	DiedWon     int64 `db:"died_won"`
	Exiled      int64 `db:"exiled"`
	ExiledWon   int64 `db:"exiled_won"`
	Eliminated  int64 `db:"eliminated"`
	FirstDeaths int64 `db:"first_deaths"`
}

// UserRank is a player's place on one guild board and how many players the board ranks.
type UserRank struct {
	Position int64 `db:"position"`
	Players  int64 `db:"players"`
}

// UserMapRecord is a player's record on one map.
type UserMapRecord struct {
	PlayMap int16 `db:"play_map"`
	Games   int64 `db:"total"`
	Wins    int64 `db:"win"`
}

// UserWeekGames counts a player's games in the week that ended WeeksAgo weeks before the reference time.
type UserWeekGames struct {
	WeeksAgo int64 `db:"weeks_ago"`
	Games    int64 `db:"games"`
}

// WeekSeconds is the length of the buckets UserActivity counts games in.
const WeekSeconds = 7 * 24 * 60 * 60

var (
	exiledAction = strconv.Itoa(int(game.EXILED))

	userSummaryQuery = "SELECT COUNT(*) AS games, " +
		"COUNT(*) FILTER (WHERE ug.player_won) AS wins, " +
		"COUNT(*) FILTER (WHERE ug.player_role = " + crewmateRole + ") AS crewmate_games, " +
		"COUNT(*) FILTER (WHERE ug.player_role = " + crewmateRole + " AND ug.player_won) AS crewmate_wins, " +
		"COUNT(*) FILTER (WHERE ug.player_role = " + impostorRole + ") AS impostor_games, " +
		"COUNT(*) FILTER (WHERE ug.player_role = " + impostorRole + " AND ug.player_won) AS impostor_wins, " +
		"MIN(g.start_time)::bigint AS first_game, MAX(g.start_time)::bigint AS last_game " +
		"FROM users_games ug INNER JOIN games g ON g.game_id = ug.game_id " +
		"WHERE ug.guild_id = $1 AND ug.user_id = $2"

	userRecentMatchesQuery = "SELECT g.game_id, g.start_time, g.end_time, g.win_type, g.play_map, " +
		"ug.player_name, ug.player_color, ug.player_role, ug.player_won " +
		"FROM users_games ug INNER JOIN games g ON g.game_id = ug.game_id " +
		"WHERE ug.guild_id = $1 AND ug.user_id = $2 " +
		"ORDER BY g.start_time DESC, g.game_id DESC LIMIT $3"

	// Each of the player's games is checked once for their own death and exile events, and once for who died
	// first, as the guild's first-target board does: the earliest death of the game, whoever it was.
	userFatesQuery = "SELECT ug.player_role AS role, COUNT(*) AS games, " +
		"COUNT(*) FILTER (WHERE ug.player_won) AS wins, " +
		"COUNT(*) FILTER (WHERE e.died) AS died, " +
		"COUNT(*) FILTER (WHERE e.died AND ug.player_won) AS died_won, " +
		"COUNT(*) FILTER (WHERE e.exiled) AS exiled, " +
		"COUNT(*) FILTER (WHERE e.exiled AND ug.player_won) AS exiled_won, " +
		"COUNT(*) FILTER (WHERE e.died OR e.exiled) AS eliminated, " +
		"COUNT(*) FILTER (WHERE f.user_id = ug.user_id) AS first_deaths " +
		"FROM users_games ug " +
		"JOIN LATERAL (SELECT COALESCE(bool_or(payload ->> 'Action' = $3), false) AS died, " +
		"COALESCE(bool_or(payload ->> 'Action' = $4), false) AS exiled " +
		"FROM game_events WHERE game_events.game_id = ug.game_id AND game_events.user_id = ug.user_id) e ON TRUE " +
		"LEFT JOIN LATERAL (SELECT game_events.user_id FROM game_events " +
		"WHERE game_events.game_id = ug.game_id AND payload ->> 'Action' = $3 " +
		"ORDER BY event_time, event_id FETCH FIRST 1 ROW ONLY) f ON TRUE " +
		"WHERE ug.guild_id = $1 AND ug.user_id = $2 " +
		"GROUP BY ug.player_role"

	// The rank queries rank exactly as the guild boards order their rows, over every player the board would
	// admit, and keep only the requested player's row. No row means the player is not ranked.
	userWinRankQuery = "SELECT position, players FROM (SELECT user_id, " +
		"ROW_NUMBER() OVER (ORDER BY win_rate DESC, win DESC, total DESC, user_id) AS position, " +
		"COUNT(*) OVER () AS players FROM (SELECT user_id, " +
		"COUNT(*) FILTER (WHERE player_won) AS win, " +
		"COUNT(*) AS total, " +
		"COUNT(*) FILTER (WHERE player_won)::decimal / COUNT(*) * 100 AS win_rate " +
		"FROM users_games WHERE guild_id = $1 AND ($2 < 0 OR player_role = $2) " +
		"GROUP BY user_id HAVING COUNT(*) >= $3) r) ranked WHERE user_id = $4"

	userGamesRankQuery = "SELECT position, players FROM (SELECT user_id, " +
		"ROW_NUMBER() OVER (ORDER BY COUNT(*) DESC, user_id) AS position, " +
		"COUNT(*) OVER () AS players " +
		"FROM users_games WHERE guild_id = $1 GROUP BY user_id) ranked WHERE user_id = $2"

	// Every self-join below restricts the other player to the guild too. Sharing a game implies it, but the
	// planner does not know that, and without it production hashed the whole users_games table (see the guild duo
	// query).
	userPlayedWithQuery = "SELECT b.user_id, COUNT(*) AS total " +
		"FROM users_games a INNER JOIN users_games b ON b.guild_id = $1 AND b.game_id = a.game_id AND b.user_id <> a.user_id " +
		"WHERE a.guild_id = $1 AND a.user_id = $2 " +
		"GROUP BY b.user_id ORDER BY total DESC, b.user_id LIMIT $3"

	// The guild duo boards restricted to one player, who is always user_id; teammate_id is the other.
	userTeammatesSelect = "SELECT a.user_id, b.user_id AS teammate_id, " +
		"COUNT(*) AS total, " +
		"COUNT(*) FILTER (WHERE a.player_won) AS win, " +
		"COUNT(*) FILTER (WHERE a.player_won)::decimal / COUNT(*) * 100 AS win_rate " +
		"FROM users_games a " +
		"INNER JOIN users_games b ON b.guild_id = $1 AND b.game_id = a.game_id AND b.user_id <> a.user_id AND b.player_role = $3 " +
		"WHERE a.guild_id = $1 AND a.user_id = $2 AND a.player_role = $3 " +
		"GROUP BY a.user_id, b.user_id HAVING COUNT(*) >= $4 "
	userBestTeammatesQuery  = userTeammatesSelect + "ORDER BY win_rate DESC, win DESC, total DESC, b.user_id LIMIT $5"
	userWorstTeammatesQuery = userTeammatesSelect + "ORDER BY win_rate ASC, win ASC, total DESC, b.user_id LIMIT $5"

	// The guild's killed-by board restricted to one crewmate. The same caveat applies: the game never reports
	// who made a kill, so a death counts against every impostor of that game.
	userKilledByQuery = "SELECT c.user_id, i.user_id AS teammate_id, " +
		"COUNT(*) FILTER (WHERE d.game_id IS NOT NULL) AS total_death, " +
		"COUNT(*) AS encounter, " +
		"COUNT(*) FILTER (WHERE d.game_id IS NOT NULL)::decimal / COUNT(*) * 100 AS death_rate " +
		"FROM users_games c " +
		"INNER JOIN users_games i ON i.guild_id = $1 AND i.game_id = c.game_id AND i.player_role = " + impostorRole + " " +
		"LEFT JOIN LATERAL (SELECT game_events.game_id FROM game_events " +
		"WHERE game_events.game_id = c.game_id AND game_events.user_id = c.user_id AND payload ->> 'Action' = $3 " +
		"FETCH FIRST 1 ROW ONLY) d ON TRUE " +
		"WHERE c.guild_id = $1 AND c.user_id = $2 AND c.player_role = " + crewmateRole + " " +
		"GROUP BY c.user_id, i.user_id HAVING COUNT(*) >= $4 " +
		"ORDER BY death_rate DESC, total_death DESC, encounter DESC, i.user_id LIMIT $5"

	userColorsQuery = "SELECT player_color AS mode, COUNT(*) AS count FROM users_games " +
		"WHERE guild_id = $1 AND user_id = $2 GROUP BY player_color ORDER BY count DESC, player_color LIMIT $3"

	userNamesQuery = "SELECT player_name AS mode, COUNT(*) AS count FROM users_games " +
		"WHERE guild_id = $1 AND user_id = $2 GROUP BY player_name ORDER BY count DESC, player_name LIMIT $3"

	userMapsQuery = "SELECT g.play_map, COUNT(*) AS total, COUNT(*) FILTER (WHERE ug.player_won) AS win " +
		"FROM users_games ug INNER JOIN games g ON g.game_id = ug.game_id " +
		"WHERE ug.guild_id = $1 AND ug.user_id = $2 AND g.play_map IS NOT NULL " +
		"GROUP BY g.play_map ORDER BY total DESC, g.play_map"

	// Week 0 is the seven days up to and including the reference time, week 1 the seven before, and so on.
	userActivityQuery = "SELECT ($3 - g.start_time) / " + strconv.Itoa(WeekSeconds) + " AS weeks_ago, COUNT(*) AS games " +
		"FROM users_games ug INNER JOIN games g ON g.game_id = ug.game_id " +
		"WHERE ug.guild_id = $1 AND ug.user_id = $2 AND g.start_time <= $3 AND g.start_time > $3 - $4 * " + strconv.Itoa(WeekSeconds) + " " +
		"GROUP BY weeks_ago"

	// Only games with a known winner make or break a streak.
	userResultsQuery = "SELECT ug.player_won FROM users_games ug INNER JOIN games g ON g.game_id = ug.game_id " +
		"WHERE ug.guild_id = $1 AND ug.user_id = $2 AND g.win_type IN (" + crewmateWinTypes + "," + impostorWinTypes + ") " +
		"ORDER BY g.start_time, g.game_id"
)

// UserSummaryStats counts a player's games and wins in a guild, overall and per role.
func UserSummaryStats(ctx context.Context, q pgxscan.Querier, guildID, userID uint64) (UserSummary, error) {
	var s UserSummary
	err := pgxscan.Get(ctx, q, &s, userSummaryQuery, guildID, userID)
	return s, err
}

// UserRecentMatches lists a player's latest matches in a guild, newest first. Only finished matches record
// players, so aborted and running ones never appear.
func UserRecentMatches(ctx context.Context, q pgxscan.Querier, guildID, userID uint64, limit int) ([]*UserMatch, error) {
	r := []*UserMatch{}
	err := pgxscan.Select(ctx, q, &r, userRecentMatchesQuery, guildID, userID, limit)
	return r, err
}

// UserFatesByRole reports, per role the player has played, how their games ended for them.
func UserFatesByRole(ctx context.Context, q pgxscan.Querier, guildID, userID uint64) ([]*UserFates, error) {
	r := []*UserFates{}
	err := pgxscan.Select(ctx, q, &r, userFatesQuery, guildID, userID, diedAction, exiledAction)
	return r, err
}

// UserWinRank places a player on the guild's winrate board for a role (or AnyRole), among players with at least
// minGames games in it. It returns nil when the player is not ranked.
func UserWinRank(ctx context.Context, q pgxscan.Querier, guildID, userID uint64, role int16, minGames int) (*UserRank, error) {
	return oneRank(ctx, q, userWinRankQuery, guildID, role, minGames, userID)
}

// UserGamesRank places a player on the guild's most-games board. It returns nil when the player has no games.
func UserGamesRank(ctx context.Context, q pgxscan.Querier, guildID, userID uint64) (*UserRank, error) {
	return oneRank(ctx, q, userGamesRankQuery, guildID, userID)
}

func oneRank(ctx context.Context, q pgxscan.Querier, query string, args ...interface{}) (*UserRank, error) {
	r := []*UserRank{}
	if err := pgxscan.Select(ctx, q, &r, query, args...); err != nil {
		return nil, err
	}
	if len(r) == 0 {
		return nil, nil
	}
	return r[0], nil
}

// UserPlayedWith ranks the linked players a player shared the most games with, in any role.
func UserPlayedWith(ctx context.Context, q pgxscan.Querier, guildID, userID uint64, limit int) ([]*GuildPlayerGames, error) {
	r := []*GuildPlayerGames{}
	err := pgxscan.Select(ctx, q, &r, userPlayedWithQuery, guildID, userID, limit)
	return r, err
}

// UserTeammates ranks the players who shared a role with the player in at least minGames games, best winrate
// first, or worst first when worst is set.
func UserTeammates(ctx context.Context, q pgxscan.Querier, guildID, userID uint64, role game.GameRole, minGames, limit int, worst bool) ([]*PostgresBestTeammatePlayerRanking, error) {
	query := userBestTeammatesQuery
	if worst {
		query = userWorstTeammatesQuery
	}
	r := []*PostgresBestTeammatePlayerRanking{}
	err := pgxscan.Select(ctx, q, &r, query, guildID, userID, int16(role), minGames, limit)
	return r, err
}

// UserKilledBy ranks the impostors a crewmate most often died with in the game, among impostors they shared at
// least minGames games with.
func UserKilledBy(ctx context.Context, q pgxscan.Querier, guildID, userID uint64, minGames, limit int) ([]*PostgresUserMostFrequentKilledByanking, error) {
	r := []*PostgresUserMostFrequentKilledByanking{}
	err := pgxscan.Select(ctx, q, &r, userKilledByQuery, guildID, userID, diedAction, minGames, limit)
	return r, err
}

// UserColors ranks the in-game colors a player used by games.
func UserColors(ctx context.Context, q pgxscan.Querier, guildID, userID uint64, limit int) ([]*Int16ModeCount, error) {
	r := []*Int16ModeCount{}
	err := pgxscan.Select(ctx, q, &r, userColorsQuery, guildID, userID, limit)
	return r, err
}

// UserNames ranks the in-game names a player used by games.
func UserNames(ctx context.Context, q pgxscan.Querier, guildID, userID uint64, limit int) ([]*StringModeCount, error) {
	r := []*StringModeCount{}
	err := pgxscan.Select(ctx, q, &r, userNamesQuery, guildID, userID, limit)
	return r, err
}

// UserMaps reports a player's record on each map, most played first. Games whose map was not recorded are left
// out.
func UserMaps(ctx context.Context, q pgxscan.Querier, guildID, userID uint64) ([]*UserMapRecord, error) {
	r := []*UserMapRecord{}
	err := pgxscan.Select(ctx, q, &r, userMapsQuery, guildID, userID)
	return r, err
}

// UserActivity counts a player's games in each of the weeks weeks up to now (Unix seconds). Weeks without games
// have no row.
func UserActivity(ctx context.Context, q pgxscan.Querier, guildID, userID uint64, now int64, weeks int) ([]*UserWeekGames, error) {
	if weeks <= 0 {
		return nil, errors.New("weeks must be positive")
	}
	r := []*UserWeekGames{}
	err := pgxscan.Select(ctx, q, &r, userActivityQuery, guildID, userID, now, weeks)
	return r, err
}

// UserResults lists whether the player won each of their games with a known winner, oldest first.
func UserResults(ctx context.Context, q pgxscan.Querier, guildID, userID uint64) ([]bool, error) {
	r := []bool{}
	err := pgxscan.Select(ctx, q, &r, userResultsQuery, guildID, userID)
	return r, err
}

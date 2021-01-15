package storage

import (
	"context"
	"github.com/automuteus/utils/pkg/game"
	"github.com/georgysavva/scany/pgxscan"
	"log"
	"strconv"
)

func (psqlInterface *PsqlInterface) NumGamesPlayedOnGuild(guildID string) int64 {
	gid, _ := strconv.ParseInt(guildID, 10, 64)
	var r int64
	err := pgxscan.Get(context.Background(), psqlInterface.Pool, &r, "SELECT COUNT(*) FROM games WHERE guild_id=$1 AND end_time != -1;", gid)
	if err != nil {
		return -1
	}
	return r
}

func (psqlInterface *PsqlInterface) NumGamesWonAsRoleOnServer(guildID string, role game.GameRole) int64 {
	gid, _ := strconv.ParseInt(guildID, 10, 64)
	var r int64
	var err error
	if role == game.CrewmateRole {
		err = pgxscan.Get(context.Background(), psqlInterface.Pool, &r, "SELECT COUNT(*) FROM games WHERE guild_id=$1 AND (win_type=0 OR win_type=1 OR win_type=6)", gid)
	} else {
		err = pgxscan.Get(context.Background(), psqlInterface.Pool, &r, "SELECT COUNT(*) FROM games WHERE guild_id=$1 AND (win_type=2 OR win_type=3 OR win_type=4 OR win_type=5)", gid)
	}
	if err != nil {
		log.Println(err)
		return -1
	}
	return r
}

func (psqlInterface *PsqlInterface) NumGamesPlayedByUser(userID string) int64 {
	var r int64
	err := pgxscan.Get(context.Background(), psqlInterface.Pool, &r, "SELECT COUNT(*) FROM users_games WHERE user_id=$1;", userID)
	if err != nil {
		return -1
	}
	return r
}

func (psqlInterface *PsqlInterface) NumGuildsPlayedInByUser(userID string) int64 {
	var r int64
	err := pgxscan.Get(context.Background(), psqlInterface.Pool, &r, "SELECT COUNT(DISTINCT guild_id) FROM users_games WHERE user_id=$1;", userID)
	if err != nil {
		return -1
	}
	return r
}

func (psqlInterface *PsqlInterface) NumGamesPlayedByUserOnServer(userID, guildID string) int64 {
	var r int64
	gid, _ := strconv.ParseInt(guildID, 10, 64)
	err := pgxscan.Get(context.Background(), psqlInterface.Pool, &r, "SELECT COUNT(*) FROM users_games WHERE user_id=$1 AND guild_id=$2", userID, gid)
	if err != nil {
		return -1
	}
	return r
}

func (psqlInterface *PsqlInterface) NumWinsAsRoleOnServer(userID, guildID string, role int16) int64 {
	var r int64
	err := pgxscan.Get(context.Background(), psqlInterface.Pool, &r, "SELECT COUNT(*) FROM users_games WHERE user_id=$1 AND guild_id=$2 AND player_role=$3 AND player_won=true;", userID, guildID, role)
	if err != nil {
		return -1
	}
	return r
}

func (psqlInterface *PsqlInterface) NumWinsAsRole(userID string, role int16) int64 {
	var r int64
	err := pgxscan.Get(context.Background(), psqlInterface.Pool, &r, "SELECT COUNT(*) FROM users_games WHERE user_id=$1 AND player_role=$2 AND player_won=true;", userID, role)
	if err != nil {
		return -1
	}
	return r
}

func (psqlInterface *PsqlInterface) NumGamesAsRoleOnServer(userID, guildID string, role int16) int64 {
	var r int64
	err := pgxscan.Get(context.Background(), psqlInterface.Pool, &r, "SELECT COUNT(*) FROM users_games WHERE user_id=$1 AND guild_id=$2 AND player_role=$3;", userID, guildID, role)
	if err != nil {
		return -1
	}
	return r
}

func (psqlInterface *PsqlInterface) NumGamesAsRole(userID string, role int16) int64 {
	var r int64
	err := pgxscan.Get(context.Background(), psqlInterface.Pool, &r, "SELECT COUNT(*) FROM users_games WHERE user_id=$1 AND player_role=$2;", userID, role)
	if err != nil {
		return -1
	}
	return r
}

func (psqlInterface *PsqlInterface) NumWinsOnServer(userID, guildID string) int64 {
	var r int64
	err := pgxscan.Get(context.Background(), psqlInterface.Pool, &r, "SELECT COUNT(*) FROM users_games WHERE user_id=$1 AND guild_id=$2 AND player_won=true;", userID, guildID)
	if err != nil {
		return -1
	}
	return r
}

func (psqlInterface *PsqlInterface) NumWins(userID string) int64 {
	var r int64
	err := pgxscan.Get(context.Background(), psqlInterface.Pool, &r, "SELECT COUNT(*) FROM users_games WHERE user_id=$1 AND player_won=true;", userID)
	if err != nil {
		return -1
	}
	return r
}

type Int16ModeCount struct {
	Count int64 `db:"count"`
	Mode  int16 `db:"mode"`
}
type Uint64ModeCount struct {
	Count int64  `db:"count"`
	Mode  uint64 `db:"mode"`
}

type StringModeCount struct {
	Count int64  `db:"count"`
	Mode  string `db:"mode"`
}

//func (psqlInterface *PsqlInterface) ColorRankingForPlayer(userID string) []*Int16ModeCount {
//	r := []*Int16ModeCount{}
//	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT count(*),mode() within GROUP (ORDER BY player_color) AS mode FROM users_games WHERE user_id=$1 GROUP BY player_color ORDER BY count desc;", userID)
//
//	if err != nil {
//		log.Println(err)
//	}
//	return r
//}
func (psqlInterface *PsqlInterface) ColorRankingForPlayerOnServer(userID, guildID string) []*Int16ModeCount {
	r := []*Int16ModeCount{}
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT count(*),mode() within GROUP (ORDER BY player_color) AS mode FROM users_games WHERE user_id=$1 AND guild_id=$2 GROUP BY player_color ORDER BY count desc;", userID, guildID)

	if err != nil {
		log.Println(err)
	}
	return r
}

//func (psqlInterface *PsqlInterface) NamesRankingForPlayer(userID string) []*StringModeCount {
//	r := []*StringModeCount{}
//	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT count(*),mode() within GROUP (ORDER BY player_name) AS mode FROM users_games WHERE user_id=$1 GROUP BY player_name ORDER BY count desc;", userID)
//
//	if err != nil {
//		log.Println(err)
//	}
//	return r
//}

func (psqlInterface *PsqlInterface) NamesRankingForPlayerOnServer(userID, guildID string) []*StringModeCount {
	r := []*StringModeCount{}
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT count(*),mode() within GROUP (ORDER BY player_name) AS mode FROM users_games WHERE user_id=$1 AND guild_id=$2 GROUP BY player_name ORDER BY count desc;", userID, guildID)

	if err != nil {
		log.Println(err)
	}
	return r
}

func (psqlInterface *PsqlInterface) TotalGamesRankingForServer(guildID uint64) []*Uint64ModeCount {
	r := []*Uint64ModeCount{}
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT count(*),mode() within GROUP (ORDER BY user_id) AS mode FROM users_games WHERE guild_id=$1 GROUP BY user_id ORDER BY count desc;", guildID)

	if err != nil {
		log.Println(err)
	}
	return r
}

func (psqlInterface *PsqlInterface) OtherPlayersRankingForPlayerOnServer(userID, guildID string) []*PostgresOtherPlayerRanking {
	r := []*PostgresOtherPlayerRanking{}
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT distinct B.user_id,"+
		"count(*) over (partition by B.user_id),"+
		"(count(*) over (partition by B.user_id)::decimal / (SELECT count(*) from users_games where user_id=$1 AND guild_id=$2))*100 as percent "+
		"FROM users_games A INNER JOIN users_games B ON A.game_id = B.game_id AND A.user_id != B.user_id "+
		"WHERE A.user_id=$1 AND A.guild_id=$2 "+
		"ORDER BY percent desc", userID, guildID)

	if err != nil {
		log.Println(err)
	}
	return r
}

func (psqlInterface *PsqlInterface) TotalWinRankingForServerByRole(guildID uint64, role int16) []*PostgresPlayerRanking {
	r := []*PostgresPlayerRanking{}
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT DISTINCT user_id,"+
		"COUNT(user_id) FILTER ( WHERE player_won = TRUE ) AS win, "+
		// "COUNT(user_id) FILTER ( WHERE player_won = FALSE ) AS loss," +
		"COUNT(*) AS total, "+
		"(COUNT(user_id) FILTER ( WHERE player_won = TRUE )::decimal / COUNT(*)) * 100 AS win_rate "+
		// "(COUNT(user_id) FILTER ( WHERE player_won = FALSE )::decimal / COUNT(*)) * 100 AS loss_rate" +
		"FROM users_games "+
		"WHERE guild_id = $1 AND player_role = $2 "+
		"GROUP BY user_id "+
		"ORDER BY win_rate DESC", guildID, role)

	if err != nil {
		log.Println(err)
	}
	return r
}

func (psqlInterface *PsqlInterface) TotalWinRankingForServer(guildID uint64) []*PostgresPlayerRanking {
	r := []*PostgresPlayerRanking{}
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT DISTINCT user_id,"+
		"COUNT(user_id) FILTER ( WHERE player_won = TRUE ) AS win, "+
		// "COUNT(user_id) FILTER ( WHERE player_won = FALSE ) AS loss," +
		"COUNT(*) AS total, "+
		"(COUNT(user_id) FILTER ( WHERE player_won = TRUE )::decimal / COUNT(*)) * 100 AS win_rate "+
		// "(COUNT(user_id) FILTER ( WHERE player_won = FALSE )::decimal / COUNT(*)) * 100 AS loss_rate" +
		"FROM users_games "+
		"WHERE guild_id = $1 "+
		"GROUP BY user_id "+
		"ORDER BY win_rate DESC", guildID)

	if err != nil {
		log.Println(err)
	}
	return r
}

func (psqlInterface *PsqlInterface) DeleteAllGamesForServer(guildID string) error {
	_, err := psqlInterface.Pool.Exec(context.Background(), "DELETE FROM games WHERE guild_id=$1", guildID)
	return err
}

func (psqlInterface *PsqlInterface) DeleteAllGamesForUser(userID string) error {
	_, err := psqlInterface.Pool.Exec(context.Background(), "DELETE FROM users_games WHERE user_id=$1", userID)
	return err
}

func (psqlInterface *PsqlInterface) BestTeammateByRole(userID, guildID string, role int16, leaderboardMin int) []*PostgresBestTeammatePlayerRanking {
	r := []*PostgresBestTeammatePlayerRanking{}
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT DISTINCT users_games.user_id, "+
		"uG.user_id as teammate_id,"+
		"COUNT(users_games.player_won) as total, "+
		"COUNT(users_games.player_won) FILTER ( WHERE users_games.player_won = TRUE ) as win, "+
		"(COUNT(users_games.user_id) FILTER ( WHERE users_games.player_won = TRUE )::decimal / COUNT(*)) * 100 AS win_rate "+
		"FROM users_games "+
		"INNER JOIN users_games uG ON users_games.game_id = uG.game_id AND users_games.user_id <> uG.user_id "+
		"WHERE users_games.guild_id = $1 AND users_games.player_role = $2 AND uG.player_role = $2 AND users_games.user_id = $3 "+
		"GROUP BY users_games.user_id, uG.user_id "+
		"HAVING COUNT(users_games.player_won) >= $4 "+
		"ORDER BY win_rate DESC, win DESC, total DESC", guildID, role, userID, leaderboardMin)

	if err != nil {
		log.Println(err)
	}
	return r
}

func (psqlInterface *PsqlInterface) WorstTeammateByRole(userID, guildID string, role int16, leaderboardMin int) []*PostgresWorstTeammatePlayerRanking {
	r := []*PostgresWorstTeammatePlayerRanking{}
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT DISTINCT users_games.user_id, "+
		"uG.user_id as teammate_id,"+
		"COUNT(users_games.player_won) as total, "+
		"COUNT(users_games.player_won) FILTER ( WHERE users_games.player_won = FALSE ) as loose, "+
		"(COUNT(users_games.user_id) FILTER ( WHERE users_games.player_won = FALSE )::decimal / COUNT(*)) * 100 AS loose_rate "+
		"FROM users_games "+
		"INNER JOIN users_games uG ON users_games.game_id = uG.game_id AND users_games.user_id <> uG.user_id "+
		"WHERE users_games.guild_id = $1 AND users_games.player_role = $2 AND uG.player_role = $2 AND users_games.user_id = $3 "+
		"GROUP BY users_games.user_id, uG.user_id "+
		"HAVING COUNT(users_games.player_won) >= $4 "+
		"ORDER BY loose_rate DESC, loose DESC, total DESC", guildID, role, userID, leaderboardMin)

	if err != nil {
		log.Println(err)
	}
	return r
}

func (psqlInterface *PsqlInterface) BestTeammateForServerByRole(guildID string, role int16, leaderboardMin int) []*PostgresBestTeammatePlayerRanking {
	r := []*PostgresBestTeammatePlayerRanking{}
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT DISTINCT "+
		"CASE WHEN users_games.user_id > uG.user_id THEN users_games.user_id ELSE uG.user_id END, "+
		"CASE WHEN users_games.user_id > uG.user_id THEN uG.user_id ELSE users_games.user_id END as teammate_id, "+
		"COUNT(users_games.player_won) as total, "+
		"COUNT(users_games.player_won) FILTER ( WHERE users_games.player_won = TRUE ) as win, "+
		"(COUNT(users_games.user_id) FILTER ( WHERE users_games.player_won = TRUE )::decimal / COUNT(*)) * 100 AS win_rate "+
		"FROM users_games "+
		"INNER JOIN users_games uG ON users_games.game_id = uG.game_id AND users_games.user_id <> uG.user_id "+
		"WHERE users_games.guild_id = $1 AND users_games.player_role = $2 and uG.player_role = $2"+
		"GROUP BY users_games.user_id, uG.user_id "+
		"HAVING COUNT(users_games.player_won) >= $3 "+
		"ORDER BY win_rate DESC, win DESC, total DESC", guildID, role, leaderboardMin)

	if err != nil {
		log.Println(err)
	}
	return r
}

func (psqlInterface *PsqlInterface) WorstTeammateForServerByRole(guildID string, role int16, leaderboardMin int) []*PostgresWorstTeammatePlayerRanking {
	r := []*PostgresWorstTeammatePlayerRanking{}
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT DISTINCT "+
		"CASE WHEN users_games.user_id > uG.user_id THEN users_games.user_id ELSE uG.user_id END, "+
		"CASE WHEN users_games.user_id > uG.user_id THEN uG.user_id ELSE users_games.user_id END as teammate_id,"+
		"COUNT(users_games.player_won) as total, "+
		"COUNT(users_games.player_won) FILTER ( WHERE users_games.player_won = FALSE ) as loose, "+
		"(COUNT(users_games.user_id) FILTER ( WHERE users_games.player_won = FALSE )::decimal / COUNT(*)) * 100 AS loose_rate "+
		"FROM users_games "+
		"INNER JOIN users_games uG ON users_games.game_id = uG.game_id AND users_games.user_id <> uG.user_id "+
		"WHERE users_games.guild_id = $1 AND users_games.player_role = $2 AND uG.player_role = $2"+
		"GROUP BY users_games.user_id, uG.user_id "+
		"HAVING COUNT(users_games.player_won) >= $3 "+
		"ORDER BY loose_rate DESC, loose DESC, total DESC", guildID, role, leaderboardMin)

	if err != nil {
		log.Println(err)
	}
	return r
}

func (psqlInterface *PsqlInterface) UserWinByActionAndRole(userdID, guildID string, action string, role int16) []*PostgresUserActionRanking {
	r := []*PostgresUserActionRanking{}
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT users_games.user_id, "+
		"COUNT(ge.user_id) FILTER ( WHERE payload ->> 'Action' = $1 ) as total_action, "+
		"total_user.total as total, "+
		"total_user.win_rate as win_rate "+
		"FROM users_games "+
		"LEFT JOIN (SELECT user_id, guild_id, player_role, "+
		"COUNT(users_games.player_won) as total, "+
		"(COUNT(users_games.user_id) FILTER ( WHERE users_games.player_won = TRUE )::decimal / COUNT(*)) * 100 AS win_rate "+
		"FROM users_games "+
		"GROUP BY user_id, player_role, guild_id "+
		") total_user on total_user.user_id = users_games.user_id and users_games.player_role = total_user.player_role and users_games.guild_id = total_user.guild_id "+
		"LEFT JOIN game_events ge ON users_games.game_id = ge.game_id AND ge.user_id = users_games.user_id "+
		"WHERE users_games.user_id = $2 AND users_games.guild_id = $3 "+
		"AND users_games.player_role = $4 "+
		"GROUP BY users_games.user_id, total, win_rate "+
		"ORDER BY win_rate DESC, total DESC;", action, userdID, guildID, role)

	if err != nil {
		log.Println(err)
	}
	return r
}

func (psqlInterface *PsqlInterface) UserFrequentFirstTarget(userID, guildID string, action string, leaderboardSize int) []*PostgresUserMostFrequentFirstTargetRanking {
	r := []*PostgresUserMostFrequentFirstTargetRanking{}
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT COUNT(*) AS total_death, "+
		"users_games.user_id, total, "+
		"COUNT(*)::decimal / total * 100 as death_rate "+
		"FROM users_games "+
		"LEFT JOIN LATERAL (SELECT game_events.user_id "+
		"FROM game_events WHERE game_events.game_id = users_games.game_id and payload ->> 'Action' = $1 "+
		"ORDER BY event_time FETCH FIRST 1 ROW ONLY ) AS ge ON TRUE "+
		"LEFT JOIN LATERAL (SELECT count(*) as total from users_games where users_games.user_id = ge.user_id and player_role = 0) as TOTAL_GAME ON TRUE "+
		"WHERE users_games.guild_id = $2 AND users_games.user_id = ge.user_id AND users_games.user_id = $3"+
		"GROUP BY users_games.user_id, total  "+
		"ORDER BY total_death DESC "+
		"LIMIT $4;", action, guildID, userID, leaderboardSize)

	if err != nil {
		log.Println(err)
	}
	return r
}

func (psqlInterface *PsqlInterface) UserMostFrequentFirstTargetForServer(guildID string, action string, leaderboardSize int) []*PostgresUserMostFrequentFirstTargetRanking {
	r := []*PostgresUserMostFrequentFirstTargetRanking{}
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT COUNT(*) AS total_death, "+
		"users_games.user_id, total, "+
		"COUNT(*)::decimal / total * 100 as death_rate "+
		"FROM users_games "+
		"LEFT JOIN LATERAL (SELECT game_events.user_id "+
		"FROM game_events WHERE game_events.game_id = users_games.game_id and payload ->> 'Action' = $1 "+
		"ORDER BY event_time FETCH FIRST 1 ROW ONLY ) AS ge ON TRUE "+
		"LEFT JOIN LATERAL (SELECT count(*) as total from users_games where users_games.user_id = ge.user_id and player_role = 0) as TOTAL_GAME ON TRUE "+
		"WHERE users_games.guild_id = $2 AND users_games.user_id = ge.user_id AND total > 3"+
		"GROUP BY users_games.user_id, total  "+
		"ORDER BY death_rate DESC, total_death DESC "+
		"LIMIT $3;", action, guildID, leaderboardSize)

	if err != nil {
		log.Println(err)
	}
	return r
}

func (psqlInterface *PsqlInterface) UserMostFrequentKilledBy(userID, guildID string) []*PostgresUserMostFrequentKilledByanking {
	r := []*PostgresUserMostFrequentKilledByanking{}
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT users_games.user_id, "+
		"usG.user_id as teammate_id, "+
		"COUNT(ge.user_id) FILTER ( WHERE payload ->> 'Action' = $1 ) as total_death, "+
		"COUNT(usG.user_id) as encounter, (COUNT(ge.user_id) FILTER ( WHERE payload ->> 'Action' = $1 ))::decimal/count(usG.player_name) * 100 as death_rate "+
		"FROM users_games "+
		"LEFT JOIN users_games usG on users_games.game_id = usG.game_id and usG.player_role = $2 "+
		"LEFT JOIN (SELECT user_id, guild_id, player_role, COUNT(users_games.player_won) as total "+
		"FROM users_games "+
		"GROUP BY user_id, player_role, guild_id) total_user on total_user.user_id = users_games.user_id and users_games.player_role = total_user.player_role and users_games.guild_id = total_user.guild_id "+
		"LEFT JOIN game_events ge ON users_games.game_id = ge.game_id AND ge.user_id = $3 "+
		"WHERE users_games.guild_id = $4 AND users_games.user_id = $3 AND users_games.player_role = $5 "+
		"GROUP BY users_games.user_id, usG.user_id, users_games.user_id, total "+
		"ORDER BY death_rate DESC, total_death DESC, encounter DESC;", strconv.Itoa(int(game.DIED)), strconv.Itoa(int(game.ImposterRole)), userID, guildID, strconv.Itoa(int(game.CrewmateRole)))
	if err != nil {
		log.Println(err)
	}
	return r
}

func (psqlInterface *PsqlInterface) UserMostFrequentKilledByServer(guildID string) []*PostgresUserMostFrequentKilledByanking {
	r := []*PostgresUserMostFrequentKilledByanking{}
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT users_games.user_id, "+
		"usG.user_id as teammate_id, "+
		"COUNT(ge.user_id) FILTER ( WHERE payload ->> 'Action' = $1 ) as total_death, "+
		"COUNT(usG.user_id) as encounter, (COUNT(ge.user_id) FILTER ( WHERE payload ->> 'Action' = $1 ))::decimal/count(usG.player_name) * 100 as death_rate "+
		"FROM users_games "+
		"INNER JOIN users_games usG on users_games.game_id = usG.game_id and usG.player_role = $2 "+
		"INNER JOIN (SELECT user_id, guild_id, player_role, COUNT(users_games.player_won) as total "+
		"FROM users_games "+
		"GROUP BY user_id, player_role, guild_id) total_user on total_user.user_id = users_games.user_id and users_games.player_role = total_user.player_role and users_games.guild_id = total_user.guild_id "+
		"INNER JOIN game_events ge ON users_games.game_id = ge.game_id AND ge.user_id = users_games.user_id "+
		"WHERE users_games.guild_id = $3 AND users_games.player_role = $4 "+
		"GROUP BY users_games.user_id, usG.user_id, users_games.user_id, total "+
		"ORDER BY death_rate DESC, total_death DESC, encounter DESC;", strconv.Itoa(int(game.DIED)), strconv.Itoa(int(game.ImposterRole)), guildID, strconv.Itoa(int(game.CrewmateRole)))
	if err != nil {
		log.Println(err)
	}
	return r
}

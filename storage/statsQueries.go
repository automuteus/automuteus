package storage

import (
	"context"
	"github.com/georgysavva/scany/pgxscan"
	"log"
	"strconv"
)

func (psqlInterface *PsqlInterface) NumGamesPlayedOnGuild(guildID string) int64 {
	gid, _ := strconv.ParseInt(guildID, 10, 64)
	r := []int64{}
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT COUNT(*) FROM games WHERE guild_id=$1 AND end_time != -1;", gid)
	if err != nil || len(r) < 1 {
		return -1
	}
	return r[0]
}

func (psqlInterface *PsqlInterface) NumGamesPlayedByUser(userID string) int64 {
	r := []int64{}
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT COUNT(*) FROM users_games WHERE user_id=$1;", userID)
	if err != nil || len(r) < 1 {
		return -1
	}
	return r[0]
}

func (psqlInterface *PsqlInterface) NumGuildsPlayedInByUser(userID string) int64 {
	r := []int64{}
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT COUNT(DISTINCT guild_id) FROM users_games WHERE user_id=$1;", userID)
	if err != nil || len(r) < 1 {
		return -1
	}
	return r[0]
}

func (psqlInterface *PsqlInterface) NumGamesPlayedByUserOnServer(userID, guildID string) int64 {
	r := []int64{}
	gid, _ := strconv.ParseInt(guildID, 10, 64)
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT COUNT(*) FROM users_games WHERE user_id=$1 AND guild_id=$2", userID, gid)
	if err != nil || len(r) < 1 {
		return -1
	}
	return r[0]
}

func (psqlInterface *PsqlInterface) NumWinsAsRoleOnServer(userID, guildID string, role int16) int64 {
	r := []int64{}
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT COUNT(*) FROM users_games WHERE user_id=$1 AND guild_id=$2 AND player_role=$3 AND player_won=true;", userID, guildID, role)
	if err != nil || len(r) < 1 {
		return -1
	}
	return r[0]
}

func (psqlInterface *PsqlInterface) NumWinsAsRole(userID string, role int16) int64 {
	r := []int64{}
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT COUNT(*) FROM users_games WHERE user_id=$1 AND player_role=$2 AND player_won=true;", userID, role)
	if err != nil || len(r) < 1 {
		return -1
	}
	return r[0]
}

func (psqlInterface *PsqlInterface) NumGamesAsRoleOnServer(userID, guildID string, role int16) int64 {
	r := []int64{}
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT COUNT(*) FROM users_games WHERE user_id=$1 AND guild_id=$2 AND player_role=$3;", userID, guildID, role)
	if err != nil || len(r) < 1 {
		return -1
	}
	return r[0]
}

func (psqlInterface *PsqlInterface) NumGamesAsRole(userID string, role int16) int64 {
	r := []int64{}
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT COUNT(*) FROM users_games WHERE user_id=$1 AND player_role=$2;", userID, role)
	if err != nil || len(r) < 1 {
		return -1
	}
	return r[0]
}

func (psqlInterface *PsqlInterface) NumWinsOnServer(userID, guildID string) int64 {
	r := []int64{}
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT COUNT(*) FROM users_games WHERE user_id=$1 AND guild_id=$2 AND player_won=true;", userID, guildID)
	if err != nil || len(r) < 1 {
		return -1
	}
	return r[0]
}

func (psqlInterface *PsqlInterface) NumWins(userID string) int64 {
	r := []int64{}
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &r, "SELECT COUNT(*) FROM users_games WHERE user_id=$1 AND player_won=true;", userID)
	if err != nil || len(r) < 1 {
		return -1
	}
	return r[0]
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

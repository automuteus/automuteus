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
	err := pgxscan.Select(context.Background(), psqlInterface.pool, &r, "SELECT COUNT(*) FROM games WHERE guild_id=$1;", gid)
	if err != nil || len(r) < 1 {
		return -1
	}
	return r[0]
}

func (psqlInterface *PsqlInterface) NumGamesPlayedTotal() int64 {
	r := []int64{}
	err := pgxscan.Select(context.Background(), psqlInterface.pool, &r, "SELECT COUNT(*) FROM games;")
	if err != nil || len(r) < 1 {
		return -1
	}
	return r[0]
}

func (psqlInterface *PsqlInterface) NumGamesPlayedByUser(userID string) int64 {
	r := []int64{}
	err := pgxscan.Select(context.Background(), psqlInterface.pool, &r, "SELECT COUNT(*) FROM users_games WHERE user_id=$1;", userID)
	if err != nil || len(r) < 1 {
		return -1
	}
	return r[0]
}

func (psqlInterface *PsqlInterface) NumGamesPlayedByUserOnServer(userID, guildID string) int64 {
	r := []int64{}
	gid, _ := strconv.ParseInt(guildID, 10, 64)
	err := pgxscan.Select(context.Background(), psqlInterface.pool, &r, "SELECT COUNT(*) FROM users_games WHERE user_id=$1 AND guild_id=$2", userID, gid)
	if err != nil || len(r) < 1 {
		return -1
	}
	return r[0]
}

type IntModeCount struct {
	Count int64 `db:"count"`
	Mode  int16 `db:"mode"`
}

type StringModeCount struct {
	Count int64  `db:"count"`
	Mode  string `db:"mode"`
}

func (psqlInterface *PsqlInterface) ColorRankingForPlayer(userID string) []*IntModeCount {
	r := []*IntModeCount{}
	err := pgxscan.Select(context.Background(), psqlInterface.pool, &r, "SELECT count(*),mode() within GROUP (ORDER BY player_color) AS mode FROM users_games WHERE user_id=$1 GROUP BY player_color ORDER BY count desc;", userID)

	if err != nil {
		log.Println(err)
	}
	return r
}

func (psqlInterface *PsqlInterface) NamesRanking(userID string) []*StringModeCount {
	r := []*StringModeCount{}
	err := pgxscan.Select(context.Background(), psqlInterface.pool, &r, "SELECT count(*),mode() within GROUP (ORDER BY player_name) AS mode FROM users_games WHERE user_id=$1 GROUP BY player_name ORDER BY count desc;", userID)

	if err != nil {
		log.Println(err)
	}
	return r
}

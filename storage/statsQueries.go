package storage

import (
	"context"
	"github.com/georgysavva/scany/pgxscan"
	"log"
)

func (psqlInterface *PsqlInterface) NumGamesPlayedOnGuild(guildID string) int64 {
	r := []int64{}
	err := pgxscan.Select(context.Background(), psqlInterface.pool, &r, "SELECT COUNT(*) FROM guilds_games WHERE guild_id=$1;", guildID)
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
	hashed := HashUserID(userID)
	err := pgxscan.Select(context.Background(), psqlInterface.pool, &r, "SELECT COUNT(*) FROM users_games WHERE hashed_user_id=$1;", hashed)
	if err != nil || len(r) < 1 {
		return -1
	}
	return r[0]
}

func (psqlInterface *PsqlInterface) NumGamesPlayedByUserOnServer(userID, guildID string) int64 {
	r := []int64{}
	hashed := HashUserID(userID)
	err := pgxscan.Select(context.Background(), psqlInterface.pool, &r, "SELECT COUNT(*) FROM guilds_games JOIN users_games ON (guilds_games.game_id = users_games.game_id) WHERE hashed_user_id=$1 AND guild_id=$2", hashed, guildID)
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
	hashed := HashUserID(userID)
	err := pgxscan.Select(context.Background(), psqlInterface.pool, &r, "SELECT count(*),mode() within GROUP (ORDER BY player_color) AS mode FROM users_games WHERE hashed_user_id=$1 GROUP BY player_color;", hashed)

	if err != nil {
		log.Println(err)
	}
	return r
}

func (psqlInterface *PsqlInterface) NamesRanking(userID string) []*StringModeCount {
	r := []*StringModeCount{}
	hashed := HashUserID(userID)
	err := pgxscan.Select(context.Background(), psqlInterface.pool, &r, "SELECT count(*),mode() within GROUP (ORDER BY player_name) AS mode FROM users_games WHERE hashed_user_id=$1 GROUP BY player_name;", hashed)

	if err != nil {
		log.Println(err)
	}
	return r
}

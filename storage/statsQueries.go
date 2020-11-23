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

type ModeAndCount struct {
	Count int64 `db:"count"`
	Mode  int16 `db:"mode"`
}

func (psqlInterface *PsqlInterface) ColorRankingForPlayer(userID string) []*ModeAndCount {
	r := []*ModeAndCount{}
	hashed := HashUserID(userID)
	err := pgxscan.Select(context.Background(), psqlInterface.pool, &r, "SELECT count(*),mode() within GROUP (ORDER BY player_color) AS mode FROM users_games WHERE hashed_user_id=$1 GROUP BY player_color;", hashed)

	if err != nil {
		log.Println(err)
	}
	return r
}

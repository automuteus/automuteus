package storage

import (
	"context"
	"github.com/georgysavva/scany/pgxscan"
)

func (psqlInterface *PsqlInterface) NumGamesPlayedOnGuild(guildID string) int64 {
	r := []int64{}
	err := pgxscan.Select(context.Background(), psqlInterface.pool, &r, "SELECT COUNT(*) FROM guilds_games WHERE guild_id=$1;", guildID)
	if err != nil || len(r) < 1 {
		return -1
	}
	return r[0]
}

func (psqlInterface *PsqlInterface) NumGamesTotal() int64 {
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

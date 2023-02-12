package rediskey

import (
	"context"
	"github.com/georgysavva/scany/pgxscan"
	"github.com/jackc/pgx/v4/pgxpool"
)

func queryTotalUsers(ctx context.Context, pool *pgxpool.Pool) int64 {
	var r []int64
	err := pgxscan.Select(ctx, pool, &r, "SELECT COUNT(*) FROM users")
	if err != nil || len(r) < 1 {
		return NotFound
	}
	return r[0]
}

func queryTotalGames(ctx context.Context, pool *pgxpool.Pool) int64 {
	var r []int64
	err := pgxscan.Select(ctx, pool, &r, "SELECT COUNT (*) FROM games WHERE start_time != -1 AND end_time != -1")
	if err != nil || len(r) < 1 {
		return NotFound
	}
	return r[0]
}

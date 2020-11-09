package storage

import (
	"context"
	"github.com/jackc/pgx/v4/pgxpool"
)

type PsqlInterface struct {
	pool *pgxpool.Pool

	//TODO does this require a lock? How should stuff be written/read from psql in an async way? Is this even a concern?
	//https://brandur.org/postgres-connections
}

type PsqlParameters struct {
	Addr     string
	Username string
	Password string
}

var psqlctx = context.Background()

func (psqlInterface *PsqlInterface) Init(params interface{}) error {
	newParams := params.(PsqlParameters)
	dbpool, err := pgxpool.Connect(context.Background(), newParams.Addr)
	if err != nil {
		return err
	}
	psqlInterface.pool = dbpool
	return nil
}

func (psqlInterface *PsqlInterface) Close() {
	psqlInterface.pool.Close()
}

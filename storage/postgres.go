package storage

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v4/pgxpool"
	"io/ioutil"
	"log"
	"os"
)

type PsqlInterface struct {
	pool *pgxpool.Pool

	//TODO does this require a lock? How should stuff be written/read from psql in an async way? Is this even a concern?
	//https://brandur.org/postgres-connections
}

func ConstructPsqlConnectURL(host, port, username, password string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s", username, password, host, port)
}

type PsqlParameters struct {
	Addr     string
	Username string
	Password string
}

var psqlctx = context.Background()

func (psqlInterface *PsqlInterface) Init(addr string) error {
	dbpool, err := pgxpool.Connect(context.Background(), addr)
	if err != nil {
		return err
	}
	psqlInterface.pool = dbpool
	return nil
}

func (psqlInterface *PsqlInterface) LoadAndExecFromFile(filepath string) error {
	f, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer f.Close()

	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}
	tag, err := psqlInterface.pool.Exec(context.Background(), string(bytes))
	if err != nil {
		return err
	}
	log.Println(tag.String())
	return nil
}

func (psqlInterface *PsqlInterface) Close() {
	psqlInterface.pool.Close()
}

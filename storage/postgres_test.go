package storage

import (
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"log"
	"os"
	"testing"
)

func TestPsqlInterface_Init(t *testing.T) {
	psql := PsqlInterface{}

	err := psql.Init(ConstructPsqlConnectURL(os.Getenv("POSTGRES_ADDR"), "dquane_postgres", os.Getenv("POSTGRES_PASS"), "postgres_test", "disable"))
	if err != nil {
		log.Fatal(err)
	}
	defer psql.Close()

	m, err := migrate.New(
		"file://database/migrations",
		ConstructPsqlConnectURL(os.Getenv("POSTGRES_ADDR"), "dquane_postgres", os.Getenv("POSTGRES_PASS"), "postgres_test", "disable"))
	if err != nil {
		log.Fatal(err)
	}
	if err := m.Up(); err != nil {
		log.Fatal(err)
	}
	//gid := uint64(141082723635691521)

	psql.GetGuildPremiumStatus("141082723635691521")

	game, err := psql.GetGame("B7B80986", "78467")
	if err != nil {
		log.Fatal(err)
	}

	events, err := psql.GetGameEvents("78467")
	if err != nil {
		log.Fatal(err)
	}

	stats := StatsFromGameAndEvents(game, events)
	log.Println(stats.ToString())
}

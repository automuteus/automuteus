package storage

import (
	"log"
	"os"
	"testing"
)

func TestPsqlInterface_Init(t *testing.T) {
	psql := PsqlInterface{}

	err := psql.Init(ConstructPsqlConnectURL(os.Getenv("POSTGRES_ADDR"), "dquane_postgres", os.Getenv("POSTGRES_PASS")))
	if err != nil {
		log.Fatal(err)
	}
	defer psql.Close()

	err = psql.LoadAndExecFromFile("./postgres.sql")
	if err != nil {
		log.Fatal(err)
	}
	//gid := uint64(141082723635691521)

	psql.GetGuildPremiumStatus("141082723635691521")

	game, err := psql.GetGame("141082723635691521", "B7B80986", "78467")
	if err != nil {
		log.Fatal(err)
	}

	events, err := psql.GetGameEvents("78467")
	if err != nil {
		log.Fatal(err)
	}

	users, err := psql.GetGameUsers("78467")
	if err != nil {
		log.Fatal(err)
	}

	stats := StatsFromGameAndEvents(game, events, users)
	log.Println(stats.ToString())
}

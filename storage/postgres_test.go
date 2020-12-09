package storage

import (
	"log"
	"testing"
)

func TestPsqlInterface_Init(t *testing.T) {
	psql := PsqlInterface{}

	err := psql.Init(ConstructPsqlConnectURL("<WHOOPS>", "<CREDENTIALS>", "<CHANGED>"))
	if err != nil {
		log.Fatal(err)
	}
	defer psql.Close()

	psql.GetGuildPremiumStatus("141082723635691521")

	//game, err := psql.GetGame("B7B80986", "78467")
	//if err != nil {
	//	log.Fatal(err)
	//}
	//
	//events, err := psql.GetGameEvents("78467")
	//if err != nil {
	//	log.Fatal(err)
	//}
	//
	//stats := StatsFromGameAndEvents(game, events)
	//log.Println(stats.ToString())
}

package storage

import (
	"log"
	"testing"
	"time"
)

func TestPsqlInterface_Init(t *testing.T) {
	psql := PsqlInterface{}

	err := psql.Init(ConstructPsqlConnectURL("192.168.1.8", "5433", "postgres", "mysecretpassword"))
	if err != nil {
		log.Fatal(err)
	}
	defer psql.Close()

	err = psql.loadAndExecFromFile("./postgres.sql")
	if err != nil {
		log.Fatal(err)
	}

	guildID := "1234146913"
	guildName := "testGuildName"
	hashedID := "wgsdfgsdf"

	err = psql.EnsureGuildExists(guildID, guildName)
	if err != nil {
		log.Fatal(err)
	}

	err = psql.EnsureUserExists("1234567", hashedID)
	if err != nil {
		log.Fatal(err)
	}

	err = psql.EnsureGuildUserExists(guildID, hashedID)
	if err != nil {
		log.Fatal(err)
	}

	gameID := int32(12345678)
	game := PostgresGame{
		GameID:      gameID,
		ConnectCode: "ABCDEFGH",
		StartTime:   time.Now().Unix(),
		WinType:     "ImposterWin",
		EndTime:     time.Now().Add(time.Hour).Unix(),
	}
	player := PostgresUserGame{
		HashedUserID: hashedID,
		GameID:       gameID,
		PlayerName:   "BadPlayer2",
		PlayerColor:  3,
		PlayerRole:   "Crewmate",
		Winner:       false,
	}

	err = psql.InsertGameAndPlayers(guildID, &game, []*PostgresUserGame{&player})
	if err != nil {
		log.Fatal(err)
	}

}

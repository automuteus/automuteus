package storage

import (
	"github.com/denverquane/amongusdiscord/game"
	"log"
	"testing"
)

func TestPsqlInterface_Init(t *testing.T) {
	psql := PsqlInterface{}

	err := psql.Init(ConstructPsqlConnectURL("192.168.1.8:5433", "postgres", "mysecretpassword"))
	if err != nil {
		log.Fatal(err)
	}
	defer psql.Close()

	//err = psql.LoadAndExecFromFile("./postgres.sql")
	//if err != nil {
	//	log.Fatal(err)
	//}
	//
	//guildID := "1234146913"
	//guildName := "testGuildName"
	//hashedID := "wgsdfgsdf"
	//
	//err = psql.EnsureGuildExists(guildID, guildName)
	//if err != nil {
	//	log.Fatal(err)
	//}

	uid := "140581066283941888"
	gamesPlayed := psql.NumGamesPlayedByUser(uid)

	r := psql.ColorRankingForPlayer(uid)
	log.Printf("Games played: %d", gamesPlayed)

	for _, v := range r {
		log.Printf("Mode: %s, Count: %d\n", game.GetColorStringForInt(int(v.Mode)), v.Count)
	}

	rs := psql.NumGamesPlayedByUserOnServer(uid, "141082723635691521")
	log.Println(rs)

	dd := psql.NamesRanking(uid)
	for _, v := range dd {
		log.Printf("Mode: %s, Count: %d\n", v.Mode, v.Count)
	}

	//err = psql.EnsureGuildUserExists(guildID, hashedID)
	//if err != nil {
	//	log.Fatal(err)
	//}
	//
	//gameID := int64(12345678)
	//game := PostgresGame{
	//	GameID:      gameID,
	//	ConnectCode: "ABCDEFGH",
	//	StartTime:   time.Now().Unix(),
	//	WinType:     0,
	//	EndTime:     time.Now().Add(time.Hour).Unix(),
	//}
	//player := PostgresUserGame{
	//	HashedUserID: hashedID,
	//	GameID:       gameID,
	//	PlayerName:   "BadPlayer2",
	//	PlayerColor:  3,
	//	PlayerRole:   "",
	//}
	//
	//err = psql.InsertGameAndPlayers(guildID, &game, []*PostgresUserGame{&player})
	//if err != nil {
	//	log.Fatal(err)
	//}

}

package storage

import (
	"log"
	"testing"
)

func TestPsqlInterface_Init(t *testing.T) {
	PsqlInterface := PsqlInterface{}

	err := PsqlInterface.Init(ConstructPsqlConnectURL("192.168.1.8", "5433", "postgres", "mysecretpassword"))
	if err != nil {
		log.Fatal(err)
	}
	defer PsqlInterface.Close()

	err = PsqlInterface.LoadAndExecFromFile("./postgres.sql")
	if err != nil {
		log.Fatal(err)
	}

	guildID := "1234146913"

	guild, err := PsqlInterface.GetGuild(guildID)
	if err != nil {
		log.Fatal(err)
	}

	if guild == nil {
		err = PsqlInterface.InsertGuild(guildID, "testGuildName")
		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Printf("Guild ID: %s, Name: %s, Premium: %s\n", guild.GuildID, guild.GuildName, guild.Premium)
	}

}

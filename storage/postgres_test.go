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

}

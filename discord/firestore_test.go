package discord

import (
	"encoding/json"
	"github.com/denverquane/amongusdiscord/storage"
	"log"
	"os"
	"testing"
)

func TestFirestoreAdd(t *testing.T) {
	log.Println(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))

	storageClient := &storage.FirestoreDriver{}
	err := storageClient.Init("testgke-290421")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	pgd := PGDDefault("testguild")
	var intf map[string]interface{}

	bytes, err := json.Marshal(pgd)
	if err != nil {
		log.Println(err)
		t.Fail()
	}
	err = json.Unmarshal(bytes, &intf)
	if err != nil {
		log.Println(err)
		t.Fail()
	}
	log.Println(intf)
	err = storageClient.WriteGuildData("testguild", intf)
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	data, err := storageClient.GetGuildData("testguild")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	var newPgd PersistentGuildData
	bytes, err = json.Marshal(data)
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	err = json.Unmarshal(bytes, &newPgd)
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	log.Println(newPgd.CommandPrefix)

}

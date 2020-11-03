package storage

import (
	"log"
	"testing"
)

func TestHashUserID(t *testing.T) {
	log.Printf(string(HashGuildID("758776223752781854")))
}

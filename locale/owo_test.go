package locale

import (
	"log"
	"testing"
)

func TestOwoToml(t *testing.T) {
	err := OwoToml("../locales/active.en.toml", "../locales/active.zu.toml")
	if err != nil {
		log.Println(err)
		return
	}
}

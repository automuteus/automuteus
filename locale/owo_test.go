package locale

import "testing"

func TestOwoToml(t *testing.T) {
	OwoToml("../locales/active.en.toml", "../locales/active.zu.toml")
}

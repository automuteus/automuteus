package command

import (
	"embed"
	"testing"
)

//go:embed locales/active.*.toml
var localizedCommandFiles embed.FS

func TestParseLocalizations(t *testing.T) {
	entries, err := localizedCommandFiles.ReadDir("locales")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		bytes, err := localizedCommandFiles.ReadFile("locales/" + entry.Name())
		if err != nil {
			t.Error(err)
		}
		if _, err = parseLocalization(string(bytes)); err != nil {
			t.Error(err)
		}
	}
}

package locale

import (
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"testing"
)

func TestInitLang(t *testing.T) {
	InitLang("", "en")
	langs := GetLanguages()
	if len(langs) != 1 {
		t.Error("Shouldn't have loaded more than a single language")
	}
	InitLang("testdata", "en")
	langs = GetLanguages()
	if len(langs) != 2 {
		t.Error("Expected 2 languages to be loaded, the default, and testdata/active.ru.toml")
	}
}

func TestLocalizeMessage(t *testing.T) {
	InitLang("", "")
	output := LocalizeMessage(&i18n.Message{
		ID:    "settings.HandleSettingsCommand.default",
		Other: "Sorry, `{{.Arg}}` is not a valid setting!\n",
	},
		map[string]interface{}{
			"Arg": "something",
		})
	if output != "Sorry, `something` is not a valid setting!\n" {
		t.Error("Substitution was not performed properly: " + output)
	}

	output = LocalizeMessage(&i18n.Message{
		ID:    "settings.HandleSettingsCommand.default",
		Other: "Sorry, `{{.Arg}}` is not a valid setting!\n",
	},
		map[string]interface{}{
			"Arg": "something",
		}, "ru")
	if output != "Sorry, `something` is not a valid setting!\n" {
		t.Error("Substitution should not succeed if ru has not been loaded: " + output)
	}
	InitLang("testdata", "en")
	output = LocalizeMessage(&i18n.Message{
		ID:    "settings.HandleSettingsCommand.default",
		Other: "Sorry, `{{.Arg}}` is not a valid setting!\n",
	},
		map[string]interface{}{
			"Arg": "something",
		}, "ru")
	if output != "Извини, `something` не является допустимым параметром!\n" {
		t.Error("Substitution should succeed if ru has not been loaded: " + output)
	}
}

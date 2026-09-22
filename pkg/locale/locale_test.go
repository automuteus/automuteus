package locale

import (
	"os"
	"testing"

	"github.com/automuteus/automuteus/v8/locales"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// The embedded set is what every binary sees; it must contain the default and every file under locales/.
func TestInitLang_LoadsEmbeddedLanguages(t *testing.T) {
	InitLang("en")
	langs := GetLanguages()
	entries, err := locales.FS.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(langs) != len(entries) {
		t.Errorf("loaded %d languages, want one per embedded file (%d): %v", len(langs), len(entries), langs)
	}
	for _, want := range []string{"en", "de", "ru", "zh"} {
		if _, ok := langs[want]; !ok {
			t.Errorf("embedded language %q missing from %v", want, langs)
		}
	}
	if len(GetBundle().LanguageTags()) < 2 {
		t.Error("bundle should carry more than the default language")
	}
}

// A process that never calls InitLang (the API) still gets the embedded set from GetLanguages.
func TestGetLanguages_LazyLoads(t *testing.T) {
	bundleInstance = nil
	localeLanguages = map[string]string{}
	langs := GetLanguages()
	if _, ok := langs["de"]; !ok || len(langs) < 2 {
		t.Errorf("GetLanguages should load the embedded translations on first use, got %v", langs)
	}
}

func TestInitLangFS_CustomDirectory(t *testing.T) {
	InitLangFS(os.DirFS("testdata"), "en")
	langs := GetLanguages()
	if len(langs) != 2 {
		t.Errorf("expected the default plus testdata/active.ru.toml, got %v", langs)
	}
	if _, ok := langs["ru"]; !ok {
		t.Errorf("ru should be loaded from testdata, got %v", langs)
	}
}

func TestInitLangFS_MissingDirectoryKeepsDefault(t *testing.T) {
	InitLangFS(os.DirFS("does-not-exist"), "en")
	langs := GetLanguages()
	if len(langs) != 1 {
		t.Errorf("only the default should be present when the directory is unreadable, got %v", langs)
	}
	if _, ok := langs["en"]; !ok {
		t.Error("default language must always be present")
	}
}

func TestLocalizeMessage(t *testing.T) {
	InitLangFS(os.DirFS("does-not-exist"), "")
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
	InitLangFS(os.DirFS("testdata"), "en")
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

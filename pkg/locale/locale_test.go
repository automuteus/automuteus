package locale

import (
	"os"
	"strings"
	"sync"
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
// resetLocale returns the package to its never-initialized state.
func resetLocale() {
	publish(nil, map[string]string{})
}

func TestGetLanguages_LazyLoads(t *testing.T) {
	resetLocale()
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

// The API process never initializes locales explicitly and serves requests concurrently, so the very first
// callers of GetLanguages may race. They must all see the complete set and the process must not crash.
func TestGetLanguages_ConcurrentFirstUse(t *testing.T) {
	resetLocale()
	const callers = 64
	results := make(chan int, callers)
	var start sync.WaitGroup
	start.Add(1)
	for i := 0; i < callers; i++ {
		go func() {
			start.Wait()
			results <- len(GetLanguages())
		}()
	}
	start.Done()
	want := len(GetLanguages())
	for i := 0; i < callers; i++ {
		if got := <-results; got != want {
			t.Errorf("caller saw %d languages, want %d", got, want)
		}
	}
}

// Re-initialization while readers are active swaps the set atomically: a reader sees the old set or the new one,
// never a partially built one.
func TestInitLang_ReinitializeUnderReaders(t *testing.T) {
	InitLang("en")
	full := len(GetLanguages())
	done := make(chan struct{})
	var readers sync.WaitGroup
	for i := 0; i < 8; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-done:
					return
				default:
					if n := len(GetLanguages()); n != full && n != 2 {
						t.Errorf("reader saw %d languages, want %d or 2", n, full)
						return
					}
				}
			}
		}()
	}
	for i := 0; i < 20; i++ {
		InitLangFS(os.DirFS("testdata"), "en")
		InitLang("en")
	}
	close(done)
	readers.Wait()
}

// The Crowdin hint is only for guilds that actually read a translation; English has nothing to fix.
func TestTranslateHint(t *testing.T) {
	InitLang("en")
	if got := TranslateHint(DefaultLang); got != "" {
		t.Errorf("default language should get no hint, got %q", got)
	}
	if got := TranslateHint(""); got != "" {
		t.Errorf("empty language should get no hint, got %q", got)
	}
	got := TranslateHint("ja")
	if !strings.Contains(got, CrowdinURL) {
		t.Errorf("non-default language hint should link to %s, got %q", CrowdinURL, got)
	}
}

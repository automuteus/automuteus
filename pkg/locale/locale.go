package locale

import (
	"io/fs"
	"log"
	"regexp"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/automuteus/automuteus/v8/locales"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

const DefaultLang = "en"

// CrowdinURL is the public translation project. Surfaced to non-English guilds
// so the people actually reading a translation are the ones invited to fix it.
const CrowdinURL = "https://automuteus.crowdin.com/"

// The published bundle and language set are replaced atomically under mu and
// never mutated after publication, so readers only ever see a complete set.
// initMu serializes lazy initialization so concurrent first callers load once.
var (
	mu              sync.RWMutex
	initMu          sync.Mutex
	bundleInstance  *i18n.Bundle
	localeLanguages = map[string]string{}
)

// messageFile matches the go-i18n active message files; the tag comes from the file name.
var messageFile = regexp.MustCompile(`^active\.(?P<lang>.*)\.toml$`)

// InitLang loads the embedded translations (locales/active.*.toml). defaultLang
// is the fallback language; empty means DefaultLang. Safe to call from any
// goroutine; readers switch to the new set atomically.
func InitLang(defaultLang string) {
	InitLangFS(locales.FS, defaultLang)
}

// InitLangFS loads translations from an arbitrary file system instead of the
// embedded one. It exists for tests that ship their own message files.
func InitLangFS(fsys fs.FS, defaultLang string) {
	if defaultLang == "" {
		defaultLang = DefaultLang
	}
	LoadTranslations(fsys, defaultLang)
}

// GetBundle returns the published bundle, loading the embedded translations on
// first use. Concurrent first callers block on one load rather than each
// loading their own.
func GetBundle() *i18n.Bundle {
	mu.RLock()
	bundle := bundleInstance
	mu.RUnlock()
	if bundle != nil {
		return bundle
	}
	initMu.Lock()
	defer initMu.Unlock()
	mu.RLock()
	bundle = bundleInstance
	mu.RUnlock()
	if bundle == nil {
		InitLang("")
		mu.RLock()
		bundle = bundleInstance
		mu.RUnlock()
	}
	return bundle
}

// GetLanguages returns the loaded language tags mapped to their display names,
// loading the embedded translations first if nothing has been loaded yet, so a
// process that never localizes anything (the API) still sees the full set. The
// returned map is shared and must be treated as read-only.
func GetLanguages() map[string]string {
	GetBundle()
	mu.RLock()
	defer mu.RUnlock()
	return localeLanguages
}

// LoadTranslations reads every active.<tag>.toml at the root of fsys into a new
// bundle and publishes it. The default language is always present in the
// language set, even when it has no message file, since its messages are
// compiled in.
func LoadTranslations(fsys fs.FS, defaultLang string) *i18n.Bundle {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	langs := map[string]string{defaultLang: language.Make(defaultLang).String()}

	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		log.Println(err)
		publish(bundle, langs)
		return bundle
	}
	for _, entry := range entries {
		match := messageFile.FindStringSubmatch(entry.Name())
		if match == nil || entry.IsDir() {
			continue
		}
		fileLang := match[messageFile.SubexpIndex("lang")]

		buf, err := fs.ReadFile(fsys, entry.Name())
		if err != nil {
			log.Println(err)
			continue
		}
		if _, err := bundle.ParseMessageFileBytes(buf, entry.Name()); err != nil {
			log.Println(err)
			continue
		}
		langName, _ := i18n.NewLocalizer(bundle, fileLang).Localize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "locale.language.name",
				Other: "English",
			},
		})
		langs[fileLang] = langName

		log.Printf("[Locale] Loaded language: %s - %s", fileLang, langName)
	}

	publish(bundle, langs)
	return bundle
}

func publish(bundle *i18n.Bundle, langs map[string]string) {
	mu.Lock()
	bundleInstance = bundle
	localeLanguages = langs
	mu.Unlock()
}

// TranslateHint returns a localized line inviting users to improve their
// language on Crowdin. It is empty for the default language, where there is
// nothing to translate, so callers can append it unconditionally.
func TranslateHint(lang string) string {
	if lang == "" || lang == DefaultLang {
		return ""
	}
	return LocalizeMessage(&i18n.Message{
		ID:    "locale.translateHint",
		Other: "Translations are community maintained. Help improve this language on [Crowdin]({{.URL}})!",
	}, map[string]interface{}{
		"URL": CrowdinURL,
	}, lang)
}

// func LocalizeMessage(message *i18n.Message, templateData map[string]interface{}) string {
func LocalizeMessage(args ...interface{}) string {
	if len(args) == 0 {
		return "Noup"
	}

	var templateData map[string]interface{}
	// note, this is the COMPILED default, not the default used in InitLang
	lang := DefaultLang
	message := args[0].(*i18n.Message)
	var pluralCount interface{} = nil

	// omgg, rework this

	// 1
	if len(args[1:]) > 0 {
		if model, ok := args[1].(map[string]interface{}); ok {
			templateData = model
		} else if model, ok := args[1].(string); ok {
			lang = model
		} else if model, ok := args[1].(int); ok {
			pluralCount = model
		}

		// 2
		if len(args[2:]) > 0 {
			if model, ok := args[2].(string); ok {
				lang = model
			} else if model, ok := args[2].(int); ok {
				pluralCount = model
			}

			// 3
			if len(args[3:]) > 0 {
				if model, ok := args[3].(int); ok {
					pluralCount = model
				}
			}
		}
	}

	bundle := GetBundle()
	localizer := i18n.NewLocalizer(bundle, lang)
	msg, err := localizer.Localize(&i18n.LocalizeConfig{
		DefaultMessage: message,
		TemplateData:   templateData,
		PluralCount:    pluralCount,
	})

	// fix go-i18n extract
	msg = strings.ReplaceAll(msg, "\\n", "\n")
	// log.Printf("[Locale] (%s) %s", lang, msg)

	if err != nil {
		log.Printf("[Locale] Warning: %s", err)
	}

	return msg
}

package locale

import (
	"io/fs"
	"log"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/automuteus/automuteus/v8/locales"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

const DefaultLang = "en"

var bundleInstance *i18n.Bundle

var localeLanguages = make(map[string]string)

// messageFile matches the go-i18n active message files; the tag comes from the file name.
var messageFile = regexp.MustCompile(`^active\.(?P<lang>.*)\.toml$`)

// InitLang loads the embedded translations (locales/active.*.toml). defaultLang
// is the fallback language; empty means DefaultLang.
func InitLang(defaultLang string) {
	InitLangFS(locales.FS, defaultLang)
}

// InitLangFS loads translations from an arbitrary file system instead of the
// embedded one. It exists for tests that ship their own message files.
func InitLangFS(fsys fs.FS, defaultLang string) {
	if defaultLang == "" {
		defaultLang = DefaultLang
	}
	bundleInstance = LoadTranslations(fsys, defaultLang)
}

func GetBundle() *i18n.Bundle {
	if bundleInstance == nil {
		InitLang("")
	}
	return bundleInstance
}

// GetLanguages returns the loaded language tags mapped to their display names,
// loading the embedded translations first if nothing has been loaded yet, so a
// process that never localizes anything (the API) still sees the full set.
func GetLanguages() map[string]string {
	GetBundle()
	return localeLanguages
}

// LoadTranslations reads every active.<tag>.toml at the root of fsys into a new
// bundle. The default language is always present in the returned language set,
// even when it has no message file, since its messages are compiled in.
func LoadTranslations(fsys fs.FS, defaultLang string) *i18n.Bundle {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	localeLanguages = make(map[string]string)
	localeLanguages[defaultLang] = language.Make(defaultLang).String()

	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		log.Println(err)
		bundleInstance = bundle
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
		localeLanguages[fileLang] = langName

		log.Printf("[Locale] Loaded language: %s - %s", fileLang, langName)
	}

	bundleInstance = bundle
	return bundle
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

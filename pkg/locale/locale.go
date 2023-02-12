package locale

import (
	"io/ioutil"
	"log"
	"path"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

const (
	DefaultLang       = "en"
	DefaultLocalePath = "locales/"
)

var bundleInstance *i18n.Bundle

var localeLanguages = make(map[string]string)

func InitLang(localePath, defaultLang string) {
	if localePath == "" {
		localePath = DefaultLocalePath
	}
	if defaultLang == "" {
		defaultLang = DefaultLang
	}
	bundleInstance = LoadTranslations(localePath, defaultLang)
}

func GetBundle() *i18n.Bundle {
	if bundleInstance == nil {
		InitLang("", "")
	}
	return bundleInstance
}

func GetLanguages() map[string]string {
	return localeLanguages
}

func LoadTranslations(localePath, defaultLang string) *i18n.Bundle {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	localeLanguages = make(map[string]string)
	localeLanguages[defaultLang] = language.Make(defaultLang).String()

	files, err := ioutil.ReadDir(localePath)
	if err == nil {
		re := regexp.MustCompile(`^active\.(?P<lang>.*)\.toml$`)
		for _, file := range files {
			if match := re.FindStringSubmatch(file.Name()); match != nil {
				fileLang := match[re.SubexpIndex("lang")]

				if _, err := bundle.LoadMessageFile(path.Join(localePath, file.Name())); err != nil {
					log.Println(err)
				} else {
					langName, _ := i18n.NewLocalizer(bundle, fileLang).Localize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "locale.language.name",
							Other: "English", /* language.Make(fileLang).String() */
						},
					})
					localeLanguages[fileLang /* msgFile.Tag.String() */] = langName

					log.Printf("[Locale] Loaded language: %s - %s", fileLang, langName)
				}
			}
		}
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

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

const DefaultLang = "en"

var LocalePath = ""

var defaultBotLang string
var bundleInstance *i18n.Bundle

var localeLanguages = make(map[string]string)

func InitLang(localePath, lang string) {
	defaultBotLang = lang
	if defaultBotLang == "" {
		defaultBotLang = DefaultLang
	}
	LocalePath = localePath
	if localePath == "" {
		LocalePath = "locales/"
	}
	GetBundle()
}

func GetBundle() *i18n.Bundle {
	if bundleInstance == nil {
		LoadTranslations()
	}
	return bundleInstance
}

func GetLanguages() map[string]string {
	return localeLanguages
}

func LoadTranslations() *i18n.Bundle {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	localeLanguages = make(map[string]string)
	localeLanguages[DefaultLang] = language.Make(DefaultLang).String()

	defaultBotLangLoaded := defaultBotLang == DefaultLang
	files, err := ioutil.ReadDir(LocalePath)
	if err == nil {
		re := regexp.MustCompile(`^active\.(?P<lang>.*)\.toml$`)
		for _, file := range files {
			if match := re.FindStringSubmatch(file.Name()); match != nil {
				fileLang := match[re.SubexpIndex("lang")]

				if _, err := bundle.LoadMessageFile(path.Join(LocalePath, file.Name())); err != nil {
					if defaultBotLang != DefaultLang && fileLang != DefaultLang {
						log.Println("[Locale] Eroor load message file:", err)
					}
				} else {
					langName, _ := i18n.NewLocalizer(bundle, fileLang).Localize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "locale.language.name",
							Other: "English", /* language.Make(fileLang).String() */
						},
					})
					localeLanguages[fileLang /* msgFile.Tag.String() */] = langName

					log.Printf("[Locale] Loaded language: %s - %s", fileLang, langName)
					if defaultBotLang == fileLang {
						defaultBotLangLoaded = true
						log.Printf("[Locale] Selected language is %s \n", defaultBotLang)
					}
				}
			}
		}
	}
	if !defaultBotLangLoaded {
		log.Printf("[Locale] Localization file with language %s not found. The default lang is set: %s\n", defaultBotLang, DefaultLang)
		defaultBotLang = DefaultLang
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
	lang := defaultBotLang
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

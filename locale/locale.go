package locale

import (
	"io/ioutil"
	"log"
	"path"
	"regexp"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

const localesDir = "locales/"
const DefaultLang = "en"

var curLang string
var bundleInstance *i18n.Bundle

func InitLang(lang string) {
	curLang = lang
	if curLang == "" {
		curLang = DefaultLang
	}
	GetBundle()
}

func GetBundle() *i18n.Bundle {
	if bundleInstance == nil {
		return LoadTranslations()
	}
	return bundleInstance
}

func LoadTranslations() *i18n.Bundle {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	curLangLoaded := false
	files, err := ioutil.ReadDir(localesDir)
	if err == nil {
		re := regexp.MustCompile(`^active\.(?P<lang>.*)\.toml$`)
		for _, file := range files {
			if match := re.FindStringSubmatch(file.Name()); match != nil {
				fileLang := match[re.SubexpIndex("lang")]

				if _, err = bundle.LoadMessageFile(path.Join(localesDir, file.Name())); err != nil {
					if curLang != DefaultLang && fileLang != DefaultLang {
						log.Println("[Locale] Eroor load message file: %s", err)
					}
				} else {
					log.Printf("[Locale] Loaded language: %s", fileLang)
					if curLang == fileLang {
						curLangLoaded = true
						log.Printf("[Locale] Selected language is %s \n", curLang)
					}
				}
			}
		}
	}
	if !curLangLoaded {
		log.Printf("[Locale] Localization file with language %s not found. The default lang is set: %s\n", curLang, DefaultLang)
		curLang = DefaultLang
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
	lang := curLang
	message := args[0].(*i18n.Message)

	// omgg
	if len(args[1:]) > 0 {
		if model, ok := args[1].(map[string]interface{}); ok {
			templateData = model
		} else if model, ok := args[1].(string); ok {
			lang = model
		}

		if len(args[1:]) > 1 {
			if model, ok := args[2].(string); ok {
				lang = model
			}
		}
	}

	bundle := GetBundle()
	localizer := i18n.NewLocalizer(bundle, lang)
	msg, err := localizer.Localize(&i18n.LocalizeConfig{
		DefaultMessage: message,
		TemplateData:   templateData,
	})

	if err != nil {
		log.Printf("[Warning] %s", err)
	}

	return msg
}

package locale

import (
	"log"
	"fmt"
	"os"

	"golang.org/x/text/language"
	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const localesDir = "locales/"
const DefaultLang = "en"

var lang string
var bundleInstance *i18n.Bundle

func InitLang(newLang string) {
	lang = newLang
	if lang == "" {
		lang = DefaultLang
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

	if _, err := os.Stat(localesDir); !os.IsNotExist(err) {
		langPath := fmt.Sprintf("%sactive.%s.toml", localesDir, lang)
		
		if _, err := bundle.LoadMessageFile(langPath); err != nil {
			if lang != DefaultLang {
				// log.Println(state)
				log.Println(err)
				log.Printf("Localization file with language %s not found. The default lang is %s\n", langPath, DefaultLang)
				lang = DefaultLang
			}
		} else {
			log.Printf("Selected language is %s \n", lang)
		}
	} else if lang != DefaultLang {
		log.Printf("Folder locales/ not found. The default lang is %s\n", DefaultLang)
		lang = DefaultLang
	}

	bundleInstance = bundle
	return bundle
}

func LocalizeSimpleMessage(message *i18n.Message) string {
	bundle := GetBundle()
	localizer := i18n.NewLocalizer(bundle, lang)
	return localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: message,
	})
}

func LocalizeMessage(message *i18n.Message, templateData map[string]interface{}) string {
	bundle := GetBundle()
	localizer := i18n.NewLocalizer(bundle, lang)
	return localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: message,
		TemplateData:   templateData,
	})
}

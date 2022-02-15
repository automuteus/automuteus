package setting

import (
	"fmt"
	"github.com/automuteus/utils/pkg/locale"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"os"
)

func FnLanguage(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	if sett == nil || len(args) < 2 {
		return nil, false
	}
	if len(args) == 2 {
		return ConstructEmbedForSetting(sett.GetLanguage(), AllSettings[Language], sett), false
	}

	if args[2] == "reload" {
		locale.InitLang(os.Getenv("LOCALE_PATH"), os.Getenv("BOT_LANG"))

		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLanguage.reloaded",
			Other: "Localization files are reloaded ({{.Count}}). Available language codes: {{.Langs}}",
		},
			map[string]interface{}{
				"Langs": locale.GetBundle().LanguageTags(),
				"Count": len(locale.GetBundle().LanguageTags()),
			}), false
	} else if args[2] == "list" {
		// settings.LoadTranslations()

		strLangs := ""
		for langCode, langName := range locale.GetLanguages() {
			strLangs += fmt.Sprintf("\n[%s] - %s", langCode, langName)
		}

		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLanguage.list",
			Other: "Available languages: {{.Langs}}",
		},
			map[string]interface{}{
				"Langs": strLangs,
			}), false
	}

	if len(args[2]) < 2 {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLanguage.tooShort",
			Other: "Sorry, the language code is short. Available language codes: {{.Langs}}.",
		},
			map[string]interface{}{
				"Langs": locale.GetBundle().LanguageTags(),
			}), false
	}

	if len(locale.GetBundle().LanguageTags()) < 2 {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLanguage.notLoaded",
			Other: "Localization files were not loaded! {{.Langs}}",
		},
			map[string]interface{}{
				"Langs": locale.GetBundle().LanguageTags(),
			}), false
	}

	langName := locale.GetLanguages()[args[2]]
	if langName == "" {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLanguage.notFound",
			Other: "Language not found! Available language codes: {{.Langs}}",
		},
			map[string]interface{}{
				"Langs": locale.GetBundle().LanguageTags(),
			}), false
	}

	sett.SetLanguage(args[2])

	return sett.LocalizeMessage(&i18n.Message{
		ID:    "settings.SettingLanguage.set",
		Other: "Localization is set to {{.LangName}}",
	},
		map[string]interface{}{
			"LangName": langName,
		}), true
}

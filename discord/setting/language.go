package setting

import (
	"github.com/automuteus/utils/pkg/locale"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func FnLanguage(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	s := GetSettingByName(Language)
	if sett == nil {
		return nil, false
	}
	if len(args) == 0 {
		return ConstructEmbedForSetting(sett.GetLanguage(), s, sett), false
	}

	if len(args[0]) < 2 {
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

	langName := locale.GetLanguages()[args[0]]
	if langName == "" {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLanguage.notFound",
			Other: "Language not found! Available language codes: {{.Langs}}",
		},
			map[string]interface{}{
				"Langs": locale.GetBundle().LanguageTags(),
			}), false
	}

	sett.SetLanguage(args[0])
	// easy way to check translation completeness; if the "Language" field is still set to English
	if langName == "English" && args[0] != "en" {
		return sett.LocalizeMessage(&i18n.Message{
			ID: "settings.SettingLanguage.set.needsTranslations",
			Other: "Localization is set to `{{.LangCode}}`, but it looks like the translations aren't complete!\n\n" +
				"Help us translate the bot [here](https://automuteus.crowdin.com/)!",
		},
			map[string]interface{}{
				"LangCode": args[0],
			}), true
	}

	return sett.LocalizeMessage(&i18n.Message{
		ID:    "settings.SettingLanguage.set",
		Other: "Localization is set to `{{.LangCode}}`",
	},
		map[string]interface{}{
			"LangCode": args[0],
		}), true
}

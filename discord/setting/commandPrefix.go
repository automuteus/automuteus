package setting

import (
	"github.com/automuteus/utils/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func FnCommandPrefix(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	if sett == nil || len(args) < 2 {
		return nil, false
	}
	if len(args) == 2 {
		embed := ConstructEmbedForSetting(sett.GetCommandPrefix(), AllSettings[Prefix], sett)
		return &embed, false
	}
	if args[2] == "<@!"+settings.OfficialBotMention+">" || args[2] == "<@"+settings.OfficialBotMention+">" {
		args[2] = "@AutoMuteUs"
	}
	if len(args[2]) > 12 {
		// prevent someone from setting something ridiculous lol
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.CommandPrefixSetting.tooLong",
			Other: "Sorry, the prefix `{{.CommandPrefix}}` is too long ({{.Length}} characters, max 10). Try something shorter.",
		},
			map[string]interface{}{
				"CommandPrefix": args[2],
				"Length":        len(args[2]),
			}), false
	}

	sett.SetCommandPrefix(args[2])
	return sett.LocalizeMessage(&i18n.Message{
		ID:    "settings.CommandPrefixSetting.changes",
		Other: "Guild prefix changed from `{{.From}}` to `{{.To}}`. Use that from now on!",
	},
		map[string]interface{}{
			"From": sett.GetCommandPrefix(),
			"To":   args[2],
		}), true
}

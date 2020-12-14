package discord

import (
	"fmt"

	"github.com/denverquane/amongusdiscord/storage"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const ASCIICrewmate = "" +
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀ ⣠⣤⣤⣤⣤⣤⣤⣤⣤⣄⡀⠀⠀⠀⠀⠀⠀⠀⠀\n" +
	"⠀⠀⠀⠀⠀⠀⠀⠀ ⢀⣴⣿⡿⠛⠉⠙⠛⠛⠛⠻⢿⣿⣷⣤⡀⠀⠀⠀⠀⠀\n" +
	"⠀⠀⠀⠀⠀⠀⠀⠀ ⣼⣿⠋⠀⠀⠀⠀      ⠀⢀⣀⣀⠈⢻⣿⣿⡄⠀⠀⠀⠀\n" +
	"⠀⠀⠀⠀⠀⠀⠀  ⣸⣿   ⠀⠀⣠⣶⣾⣿⣿⣿⠿⠿⠿⢿⣿⣿⣿⣄⠀⠀⠀\n" +
	"⠀⠀⠀⠀⠀⠀⠀  ⣿⣿   ⠀⢰⣿⣿⣯⠁⠀⠀⠀⠀⠀⠀⠀  ⠈⠙⢿⣷⡄⠀\n" +
	"⠀⠀⣀⣤⣴⣶⣶⣿⡟⠀⠀⢸⣿⣿⣿⣆⠀⠀⠀⠀⠀⠀⠀⠀    ⠀⠀⣿⣷⠀\n" +
	"⠀⢰⣿⡟⠋⠉⣹⣿⡇⠀⠀⠘⣿⣿⣿⣿⣷⣦⣤⣤⣤⣶⣶⣶⣶⣿⣿⣿⠀\n" +
	"⠀⢸⣿⡇⠀⠀⣿⣿⡇⠀⠀⠀⠹⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠃⠀\n" +
	"⠀⣸⣿⡇⠀⠀⣿⣿⡇⠀⠀⠀⠀⠉⠻⠿⣿⣿⣿⣿⡿⠿⠿⠛⢻⣿⡇⠀⠀\n" +
	"⠀⣿⣿⠁⠀⠀⣿⣿⡇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀  ⢸⣿⣧⠀⠀\n" +
	"⠀⣿⣿⠀⠀⠀⣿⣿⡇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀  ⢸⣿⣿⠀⠀\n" +
	"⠀⣿⣿⠀⠀⠀⣿⣿⡇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀  ⢸⣿⣿⠀⠀\n" +
	"⠀⢿⣿⡆⠀⠀⣿⣿⡇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀  ⠀⢸⣿⡇⠀⠀\n" +
	"⠀⠸⣿⣧⡀⠀⣿⣿⡇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀  ⠀⠀⣿⣿⠃⠀⠀\n" +
	"⠀⠀⠛⢿⣿⣿⣿⣿⣇⠀⠀⠀⠀⠀⣰⣿⣿⣷⣶⣶⣶⣶⠶⢠⣿⣿⠀⠀⠀\n" +
	"⠀⠀⠀⠀⠀⠀⠀⣿⣿⠀⠀⠀⠀⠀⣿⣿⡇⠀⣽⣿⡏⠁⠀⠀⢸⣿⡇⠀⠀⠀\n" +
	"⠀⠀⠀⠀⠀⠀⠀⣿⣿⠀⠀⠀⠀⠀⣿⣿⡇⠀⢹⣿⡆⠀⠀⠀⣸⣿⠇⠀⠀⠀\n" +
	"⠀⠀⠀⠀⠀⠀⠀⢿⣿⣦⣄⣀⣠⣴⣿⣿  ⠀⠈⠻⣿⣿⣿⡿⠏⠀⠀⠀⠀\n" +
	"⠀⠀⠀⠀⠀⠀⠀⠈⠛⠻⠿⠿⠿⠿⠋⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀"

func ASCIIStarfield(sett *storage.GuildSettings, name string, isImpostor bool, count int) string {
	isImpostorStr := sett.LocalizeMessage(&i18n.Message{
		ID:    "ascii.AsciiStarfield.isWasNot",
		Other: "was not An Impostor.",
	})

	if isImpostor {
		isImpostorStr = sett.LocalizeMessage(&i18n.Message{
			ID:    "ascii.AsciiStarfield.isWas",
			Other: "was An Impostor.",
		})
	}

	remains := sett.LocalizeMessage(&i18n.Message{
		ID:    "ascii.AsciiStarfield.remains",
		One:   "Impostor remains",
		Other: "Impostors remain",
	}, count)

	template := "" +
		". 　　　。　　　　•　 　 ﾟ　　 。 　　 .\n\n" +
		"　　　.　　　 　　.　　　　　。　　 。　. 　\n\n" +
		".　　 。　　　　　 ඞ 。 . 　　 • 　　　　•\n\n" +
		"　　ﾟ　　%s %s　。\n\n" +
		"　　'　　　 %d %s 　 　　。\n\n" +
		"　　ﾟ　　　.　　　. ,　　　　.　 .        •　 　ﾟ"

	return fmt.Sprintf(template, name, isImpostorStr, count, remains)
}

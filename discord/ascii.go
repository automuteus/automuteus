package discord

import "fmt"

const AsciiCrewmate = "⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀ ⣠⣤⣤⣤⣤⣤⣤⣤⣤⣄⡀⠀⠀⠀⠀⠀⠀⠀⠀\n" +
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

func AsciiStarfield(name string, imposter bool, count int) string {
	t := "was not"
	if imposter {
		t = "was"
	}

	// decide how much impostors remain and put out matching text --> textremimp
	textremimp := "Impostor remains"
	if count > 1 {
		textremimp = "Impostors remain"
	} else {
		textremimp = "Impostor remains"
	}

	return fmt.Sprintf(". 　　　。　　　　•　 　 ﾟ　　 。 　　 .\n\n　　　.　　　 　　.　　　　　。　　 。　. 　\n\n.　　 。　　　　　 ඞ 。 . 　　 • 　　　　•\n\n　　ﾟ　　%s %s An Impostor.　。\n\n　　'　　　 %d %s 　 　　。\n\n　　ﾟ　　　.　　　. ,　　　　.　 .        •　 　ﾟ", name, t, count, textremimp)
}

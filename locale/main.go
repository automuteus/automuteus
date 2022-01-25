package locale

import "log"

func main() {
	err := OwoToml("../locales/active.en.toml", "../locales/active.zu.toml")
	if err != nil {
		log.Println(err)
		return
	}
}

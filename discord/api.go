package discord

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"net/http"
)

func (bot *Bot) StartAPIServer(port string) {
	r := mux.NewRouter()

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// TODO For any higher-sensitivity info in the future, this should properly identify the origin specifically
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length")

		info := bot.getInfo()
		data, err := json.Marshal(info)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
		} else {
			w.Write(data)
		}
	})

	http.ListenAndServe(":"+port, r)
}

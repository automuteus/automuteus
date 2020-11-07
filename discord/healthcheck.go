package discord

import (
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

func StartHealthCheckServer(port string) {
	r := mux.NewRouter()

	r.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hell yeah"))
	})

	r.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		resp, err := http.Get("https://discordapp.com/api/v7/gateway")
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		w.WriteHeader(resp.StatusCode)
		if resp.StatusCode == http.StatusOK {
			w.Write([]byte("ready"))
		} else {
			w.Write([]byte("unready"))
		}
	})

	http.ListenAndServe(":"+port, r)
}

package discord

import (
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

var GlobalReady = false

func StartHealthCheckServer(port string) {
	r := mux.NewRouter()

	r.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hell yeah"))
	})

	r.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if GlobalReady {
			resp, err := http.Get("https://discordapp.com/api/v8/gateway")
			if err != nil {
				log.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}
			defer resp.Body.Close()
			w.WriteHeader(resp.StatusCode)
			if resp.StatusCode == http.StatusOK {
				w.Write([]byte("ready"))
			} else {
				w.Write([]byte("unready"))
			}

			if resp.StatusCode == http.StatusTooManyRequests {
				w.WriteHeader(http.StatusTooManyRequests)
				log.Println("I'M BEING RATE-LIMITED BY DISCORD")
				return
			}
		} else {
			w.WriteHeader(http.StatusTooEarly)
		}
	})

	http.ListenAndServe(":"+port, r)
}

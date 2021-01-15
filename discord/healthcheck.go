package discord

import (
	"github.com/gorilla/mux"
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
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("hell yeah"))
		} else {
			w.WriteHeader(http.StatusTooEarly)
		}
	})

	http.ListenAndServe(":"+port, r)
}

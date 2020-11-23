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
		resp, err := http.Get("https://discordapp.com/api/v8/gateway")
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

		if resp.StatusCode == http.StatusTooManyRequests {
			w.WriteHeader(http.StatusTooManyRequests)
			log.Println("I'M BEING RATE-LIMITED BY DISCORD")
			return
			//orgId := os.Getenv("SCW_ORGANIZATION_ID")
			//accessKey := os.Getenv("SCW_ACCESS_KEY")
			//secretKey := os.Getenv("SCW_SECRET_KEY")
			//nodeID := os.Getenv("SCW_NODE_ID")
			//if orgId == "" || accessKey == "" || secretKey == "" || nodeID == "" {
			//	log.Println("One of the Scaleway credentials was null, not replacing any nodes!")
			//	return
			//}
			//
			//metrics.TerminateScalewayNode(orgId, accessKey, secretKey, nodeID)
		}
	})

	http.ListenAndServe(":"+port, r)
}

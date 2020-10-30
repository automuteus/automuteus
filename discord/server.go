package discord

import (
	"encoding/json"
	"fmt"
	"github.com/denverquane/amongusdiscord/game"
	socketio "github.com/googollee/go-socket.io"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"strconv"
	"time"
)

func (bot *Bot) socketioServer(port string) {
	server, err := socketio.NewServer(nil)
	if err != nil {
		log.Fatal(err)
	}
	server.OnConnect("/", func(s socketio.Conn) error {
		s.SetContext("")
		log.Println("connected:", s.ID())
		return nil
	})
	server.OnEvent("/", "connectCode", func(s socketio.Conn, msg string) {
		log.Printf("Received connection code: \"%s\"", msg)

		bot.ConnsToGames[s.ID()] = msg

		retryCount := 0

		timer := time.NewTimer(time.Second * time.Duration(3))

		//keep publishing the connection code until someone ACKS it
		go func() {
			receiver := bot.RedisInterface.SubscribeConnectUpdateAck(msg)

			for {
				select {
				case <-receiver.Channel():
					log.Println("Received connection ack for " + msg)
					timer.Stop()
					receiver.Close()
					return

				case <-timer.C:
					if retryCount > 5 {
						log.Println("Exceeded 5 retries for connect ACK")
						s.Close()
						bot.PurgeConnection(s.ID())
						receiver.Close()
						return
					} else {
						log.Println("Publishing connect = true for " + msg)
					}
					retryCount++
					bot.RedisInterface.PublishConnectUpdate(msg, "true")
					timer.Reset(time.Second * time.Duration(3))
					break
				}
			}
		}()

	})
	server.OnEvent("/", "lobby", func(s socketio.Conn, msg string) {
		log.Println("lobby:", msg)
		lobby := game.Lobby{}
		err := json.Unmarshal([]byte(msg), &lobby)
		if err != nil {
			log.Println(err)
		} else {
			if cCode, ok := bot.ConnsToGames[s.ID()]; ok {
				bot.RedisInterface.PublishLobbyUpdate(cCode, msg)
			}
		}
	})
	server.OnEvent("/", "state", func(s socketio.Conn, msg string) {
		log.Println("phase received from capture: ", msg)
		_, err := strconv.Atoi(msg)
		if err != nil {
			log.Println(err)
		} else {
			if cCode, ok := bot.ConnsToGames[s.ID()]; ok {
				bot.RedisInterface.PublishPhaseUpdate(cCode, msg)
			}
		}
	})
	server.OnEvent("/", "player", func(s socketio.Conn, msg string) {
		log.Println("player received from capture: ", msg)
		player := game.Player{}
		err := json.Unmarshal([]byte(msg), &player)
		if err != nil {
			log.Println(err)
		} else {
			if cCode, ok := bot.ConnsToGames[s.ID()]; ok {
				bot.RedisInterface.PublishPlayerUpdate(cCode, msg)
			}
		}
	})
	server.OnError("/", func(s socketio.Conn, e error) {
		log.Println("meet error:", e)
	})
	server.OnDisconnect("/", func(s socketio.Conn, reason string) {
		log.Println("Client connection closed: ", reason)

		if cCode, ok := bot.ConnsToGames[s.ID()]; ok {
			bot.RedisInterface.PublishConnectUpdate(cCode, "false")
		}

		bot.PurgeConnection(s.ID())
	})
	go server.Serve()
	defer server.Close()

	//http.Handle("/socket.io/", server)

	router := mux.NewRouter()
	router.Handle("/socket.io/", server)
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Auto-Mute Us is up and running.")
	})

	log.Printf("Serving at localhost:%s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}

func MessagesServer(port string, bot *Bot) {
	http.HandleFunc("/graceful", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {

			bot.ChannelsMapLock.RLock()
			for _, v := range bot.RedisSubscriberKillChannels {
				v <- true
			}
			bot.ChannelsMapLock.RUnlock()
		}
	})
	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		data := map[string]interface{}{
			"activeConnections": len(bot.ConnsToGames),
			//"totalGuilds":       len(bot.GlobalBroadcastChannels), //probably an inaccurate metric
			"activeGames": len(bot.RedisSubscriberKillChannels),
		}
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			log.Println(err)
		}
		w.Write(jsonBytes)
	})

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

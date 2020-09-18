package capture

import (
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/denverquane/amongusdiscord/game"
	"github.com/gorilla/websocket"
)

// RunClientSocketBroadcast does cool stuff
func RunClientSocketBroadcast(states chan game.GameState, url string) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	c, _, err := websocket.DefaultDialer.Dial("ws://"+url+"/status", nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			log.Printf("Client recv: %s", message)
		}
	}()

	generic := game.GenericWSMessage{
		GuildID: "754465589958803548",
		Payload: []byte("null"),
	}
	jsonBytesBytes, err := json.Marshal(generic)
	if err != nil {
		log.Println(err)
		return
	}
	err = c.WriteMessage(websocket.TextMessage, jsonBytesBytes)
	if err != nil {
		log.Println("Client write:", err)
		return
	}

	for {
		select {
		case <-done:
			return
		case t := <-states:
			jsonBytes, err := json.Marshal(t)
			if err != nil {
				log.Println(err)
				return
			}
			generic := game.GenericWSMessage{
				GuildID: "",
				Payload: jsonBytes,
			}
			jsonBytesBytes, err := json.Marshal(generic)
			if err != nil {
				log.Println(err)
				return
			}
			err = c.WriteMessage(websocket.TextMessage, jsonBytesBytes)
			if err != nil {
				log.Println("Client write:", err)
				return
			}
			log.Printf("Wrote %s\n", jsonBytesBytes)
		case <-interrupt:
			log.Println("interrupt")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}
}

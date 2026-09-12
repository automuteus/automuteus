package main

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/capture"
	"github.com/automuteus/automuteus/v8/pkg/game"
	serverio "github.com/googollee/go-socket.io"
	clientio "github.com/hesh915/go-socket.io-client"
)

// Exercise the actual Socket.IO client against Galactus's server dependency,
// without Redis or Discord. The server must receive string payloads in order.
func TestSocketCompatibility(t *testing.T) {
	server, err := serverio.NewServer(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	connected := make(chan serverio.Conn, 1)
	events := make(chan sentEvent, 8)
	serverErrors := make(chan error, 8)
	server.OnConnect("/", func(conn serverio.Conn) error {
		connected <- conn
		return nil
	})
	server.OnError("/", func(_ serverio.Conn, err error) { serverErrors <- err })
	for _, name := range []string{capture.ConnectCodeEvent, capture.LobbyEvent, capture.StateEvent, capture.PlayerEvent, capture.GameOverEvent} {
		server.OnEvent("/", name, func(_ serverio.Conn, payload string) { events <- sentEvent{name, payload} })
	}
	go server.Serve()
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()
	client, err := clientio.NewClient(httpServer.URL, &clientio.Options{Transport: "websocket"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case conn := <-connected:
		defer conn.Close()
	case <-time.After(5 * time.Second):
		t.Fatal("Socket.IO connection timed out")
	}
	if err := client.Emit(capture.ConnectCodeEvent, "ABCDEFGH"); err != nil {
		t.Fatal(err)
	}
	s := &session{client: client}
	if err := s.lobby(game.Lobby{LobbyCode: "ABCDEF"}); err != nil {
		t.Fatal(err)
	}
	if err := s.player(game.Player{Name: "Player One"}, false); err != nil {
		t.Fatal(err)
	}
	if err := s.gameover(game.HumansByVote); err != nil {
		t.Fatal(err)
	}
	for _, want := range []sentEvent{
		{"connectCode", "ABCDEFGH"},
		{"lobby", `{"LobbyCode":"ABCDEF","Region":0,"Map":0}`},
		{"state", "0"},
		{"player", `{"Action":0,"Name":"Player One","Color":0,"IsDead":false,"Disconnected":false}`},
		{"gameover", `{"GameOverReason":0,"PlayerInfos":[{"Name":"Player One","IsImpostor":false}]}`},
		{"state", "0"},
	} {
		select {
		case got := <-events:
			if got != want {
				t.Fatalf("received %#v, want %#v", got, want)
			}
		case err := <-serverErrors:
			t.Fatal(err)
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for %s", want.name)
		}
	}
}

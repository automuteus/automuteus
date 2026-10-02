package main

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/game"
)

type sentEvent struct{ name, payload string }
type recordingEmitter struct {
	events []sentEvent
	failAt int
}

var errSend = errors.New("connection lost")

func (e *recordingEmitter) Emit(name string, args ...interface{}) error {
	if len(args) != 1 {
		return errors.New("expected exactly one payload")
	}
	payload, ok := args[0].(string)
	if !ok {
		return errors.New("Galactus expects a string payload")
	}
	e.events = append(e.events, sentEvent{name, payload})
	if e.failAt == len(e.events) {
		return errSend
	}
	return nil
}

func TestRoundProtocol(t *testing.T) {
	e := &recordingEmitter{}
	s := &session{client: e}
	for _, send := range []func() error{
		func() error { return s.lobby(game.Lobby{LobbyCode: "ABCDEF", Region: game.NA, PlayMap: game.FUNGLE}) },
		func() error {
			return s.player(game.Player{Action: game.JOINED, Name: "Player One", Color: game.Red}, true)
		},
		func() error { return s.phase(game.TASKS) },
		func() error { return s.gameover(game.ImpostorByKill) },
	} {
		if err := send(); err != nil {
			t.Fatal(err)
		}
	}
	// Literal event names and JSON fields intentionally pin the external protocol.
	want := []sentEvent{
		{"lobby", `{"LobbyCode":"ABCDEF","Region":0,"Map":5}`},
		{"state", "0"},
		{"player", `{"Action":0,"Name":"Player One","Color":0,"IsDead":false,"Disconnected":false}`},
		{"state", "1"},
		{"gameover", `{"GameOverReason":3,"PlayerInfos":[{"Name":"Player One","IsImpostor":true}]}`},
		{"state", "0"},
	}
	if !reflect.DeepEqual(e.events, want) {
		t.Fatalf("events = %#v, want %#v", e.events, want)
	}
	if len(s.players) != 0 {
		t.Fatal("gameover did not reset round participants")
	}
}

func TestRosterUpdatesAndResets(t *testing.T) {
	e := &recordingEmitter{}
	s := &session{client: e}
	for _, player := range []game.Player{{Name: "One", Action: game.JOINED}, {Name: "One", Action: game.LEFT}} {
		if err := s.player(player, player.Action == game.LEFT); err != nil {
			t.Fatal(err)
		}
	}
	if len(s.players) != 1 || !s.isImpostor("One") {
		t.Fatalf("role update or participant retention failed: %#v", s.players)
	}
	if err := s.lobby(game.Lobby{}); err != nil {
		t.Fatal(err)
	}
	if len(s.players) != 0 {
		t.Fatal("new lobby retained old participants")
	}
	if err := s.gameover(game.HumansByVote); err != nil {
		t.Fatal(err)
	}
	var result game.Gameover
	if err := json.Unmarshal([]byte(e.events[len(e.events)-2].payload), &result); err != nil {
		t.Fatal(err)
	}
	if result.PlayerInfos == nil || len(result.PlayerInfos) != 0 {
		t.Fatalf("empty gameover must encode an empty array: %#v", result)
	}
}

func TestSendFailures(t *testing.T) {
	for _, event := range []string{"lobby", "player", "gameover", "state"} {
		t.Run(event, func(t *testing.T) {
			e := &recordingEmitter{failAt: 1}
			s := &session{client: e, players: []game.PlayerInfo{{Name: "Existing"}}}
			var err error
			switch event {
			case "lobby":
				err = s.lobby(game.Lobby{})
			case "player":
				err = s.player(game.Player{Name: "New"}, false)
			case "gameover":
				err = s.gameover(game.HumansByVote)
			case "state":
				err = s.phase(game.LOBBY)
			}
			if !errors.Is(err, errSend) || len(e.events) != 1 || len(s.players) != 1 || s.players[0].Name != "Existing" {
				t.Fatalf("failure was hidden or changed state: err=%v, events=%v, players=%v", err, e.events, s.players)
			}
		})
	}
	e := &recordingEmitter{failAt: 2}
	s := newSession(e)
	if err := s.lobby(game.Lobby{LobbyCode: "ABCDEF"}); !errors.Is(err, errSend) || len(e.events) != 2 || s.view.phase != game.UNINITIALIZED {
		t.Fatalf("failed follow-up state was hidden or recorded: err=%v, events=%v, view=%+v", err, e.events, s.view)
	}
}

func TestViewMirrorsSentEvents(t *testing.T) {
	s := newSession(&recordingEmitter{})
	for _, send := range []func() error{
		func() error { return s.lobby(game.Lobby{LobbyCode: "ABCDEF", PlayMap: game.POLUS}) },
		func() error { return s.player(game.Player{Action: game.JOINED, Name: "One", Color: game.Red}, false) },
		func() error { return s.player(game.Player{Action: game.JOINED, Name: "Two", Color: game.Blue}, false) },
		func() error {
			return s.player(game.Player{Action: game.JOINED, Name: "Three", Color: game.Lime}, false)
		},
		func() error { return s.phase(game.TASKS) },
		func() error {
			return s.player(game.Player{Action: game.DIED, Name: "One", Color: game.Red, IsDead: true}, false)
		},
		func() error {
			return s.player(game.Player{Action: game.DISCONNECTED, Name: "Two", Color: game.Blue}, false)
		},
	} {
		if err := send(); err != nil {
			t.Fatal(err)
		}
	}
	v := s.view
	if !v.hasLobby || v.lobby.LobbyCode != "ABCDEF" || v.phase != game.TASKS || len(v.players) != 2 || !v.players[0].IsDead || v.players[1].Name != "Three" {
		t.Fatalf("view = %+v", v)
	}
	if err := s.gameover(game.HumansByTask); err != nil {
		t.Fatal(err)
	}
	if s.view.phase != game.LOBBY || len(s.view.players) != 2 || s.view.players[0].IsDead {
		t.Fatalf("gameover must return everyone to the lobby alive: %+v", s.view)
	}
	if err := s.lobby(game.Lobby{LobbyCode: "NEWCODE"}); err != nil || len(s.view.players) != 0 {
		t.Fatalf("new lobby kept old players: err=%v, view=%+v", err, s.view)
	}
}

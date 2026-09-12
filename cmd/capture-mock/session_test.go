package main

import (
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
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
	input := "l\nABCDEF\n0\n5\np\n0\nPlayer One\n0\nno\nno\nyes\ns\n1\ng\n3\nq\n"
	if err := commandLoop(newPrompts(strings.NewReader(input), io.Discard), s); err != nil {
		t.Fatal(err)
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
	err := commandLoop(newPrompts(strings.NewReader("l\n\n\n\ns\n1\nq\n"), io.Discard), &session{client: e})
	if !errors.Is(err, errSend) || len(e.events) != 2 {
		t.Fatalf("continued after failed follow-up state: err=%v, events=%v", err, e.events)
	}
}

func TestIncompleteInputDoesNotSend(t *testing.T) {
	for _, input := range []string{"", "l\n", "l\nCODE\n", "p\n0\nPlayer One\n", "s\n", "g\n", "invalid\n"} {
		e := &recordingEmitter{}
		err := commandLoop(newPrompts(strings.NewReader(input), io.Discard), &session{client: e})
		if !errors.Is(err, io.EOF) || len(e.events) != 0 {
			t.Errorf("input %q: err=%v, events=%v", input, err, e.events)
		}
	}
}

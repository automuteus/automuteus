package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// playAll answers every checkpoint with observed, as the operator would.
func playAll(r *scenarioRun, observed bool) ([]logEntry, error) {
	var log []logEntry
	for {
		entries, checkpoint, err := r.advance()
		log = append(log, entries...)
		if err != nil || checkpoint == "" {
			return log, err
		}
		log = append(log, logEntry{entryInfo, checkpoint})
		r.answer(observed)
	}
}

func scenarioNamed(t *testing.T, name string) scenario {
	for _, sc := range scenarios {
		if sc.name == name {
			return sc
		}
	}
	t.Fatalf("no scenario %q", name)
	return scenario{}
}

func countChecks(sc scenario) int {
	n := 0
	for _, st := range sc.steps {
		if st.expect != "" {
			n++
		}
	}
	return n
}

// Every scenario must play through, name the operator's player where it says
// {you}, and leave the bot in the lobby so the next scenario starts clean.
func TestScenariosPlayThrough(t *testing.T) {
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			e := &recordingEmitter{}
			r := newRun(newSession(e), sc, "Operator")
			log, err := playAll(r, true)
			if err != nil {
				t.Fatal(err)
			}
			if len(r.results) != countChecks(sc) || countChecks(sc) == 0 || tally(r.results) != len(r.results) {
				t.Fatalf("recorded %v for %d checkpoints", r.results, countChecks(sc))
			}
			if len(e.events) == 0 || e.events[0].name != "lobby" || e.events[len(e.events)-1] != (sentEvent{"state", "0"}) {
				t.Fatalf("scenario must open with a lobby and end in the lobby phase: %v", e.events)
			}
			for _, entry := range log {
				if strings.Contains(entry.text, self) {
					t.Fatalf("operator name not substituted in %q", entry.text)
				}
			}
			named := false
			for _, ev := range e.events {
				if strings.Contains(ev.payload, self) {
					t.Fatalf("operator name not substituted in payload %q", ev.payload)
				}
				named = named || strings.Contains(ev.payload, `"Name":"Operator"`)
			}
			if !named {
				t.Fatal("scenario never sent an event for the operator's player")
			}
		})
	}
}

// Pins the wire sequence of the baseline scenario.
func TestFullRoundProtocol(t *testing.T) {
	e := &recordingEmitter{}
	if _, err := playAll(newRun(newSession(e), scenarios[0], "Operator"), true); err != nil {
		t.Fatal(err)
	}
	want := []sentEvent{
		{"lobby", `{"LobbyCode":"TESTCODE","Region":0,"Map":0}`},
		{"state", "0"},
		{"player", `{"Action":0,"Name":"Operator","Color":0,"IsDead":false,"Disconnected":false}`},
		{"player", `{"Action":0,"Name":"Mock Blue","Color":1,"IsDead":false,"Disconnected":false}`},
		{"player", `{"Action":0,"Name":"Mock Green","Color":2,"IsDead":false,"Disconnected":false}`},
		{"player", `{"Action":0,"Name":"Mock Purple","Color":8,"IsDead":false,"Disconnected":false}`},
		{"state", "1"},
		{"state", "2"},
		{"state", "1"},
		{"gameover", `{"GameOverReason":1,"PlayerInfos":[{"Name":"Operator","IsImpostor":false},{"Name":"Mock Blue","IsImpostor":false},{"Name":"Mock Green","IsImpostor":false},{"Name":"Mock Purple","IsImpostor":false}]}`},
		{"state", "0"},
	}
	if !reflect.DeepEqual(e.events, want) {
		t.Fatalf("events = %#v, want %#v", e.events, want)
	}
}

func TestImpostorRoleReportedAtGameover(t *testing.T) {
	e := &recordingEmitter{}
	if _, err := playAll(newRun(newSession(e), scenarioNamed(t, "Impostor victory"), "Operator"), true); err != nil {
		t.Fatal(err)
	}
	last := e.events[len(e.events)-2]
	if last.name != "gameover" || !strings.Contains(last.payload, `{"Name":"Operator","IsImpostor":true}`) || strings.Contains(last.payload, `{"Name":"Mock Blue","IsImpostor":true}`) {
		t.Fatalf("gameover roster = %s", last.payload)
	}
}

func TestAnswersAreRecorded(t *testing.T) {
	r := newRun(newSession(&recordingEmitter{}), scenarioNamed(t, "Lobby changes"), "Operator")
	for i := 0; ; i++ {
		_, checkpoint, err := r.advance()
		if err != nil {
			t.Fatal(err)
		}
		if checkpoint == "" {
			break
		}
		r.answer(i != 1)
	}
	if len(r.results) != 4 || tally(r.results) != 3 || r.results[1].observed || !strings.Contains(r.results[0].expect, "Operator (red)") {
		t.Fatalf("results = %v", r.results)
	}
}

func TestAbortReturnsBotToMenu(t *testing.T) {
	e := &recordingEmitter{}
	r := newRun(newSession(e), scenarios[0], "Operator")
	if _, checkpoint, err := r.advance(); err != nil || checkpoint == "" {
		t.Fatalf("checkpoint=%q, err=%v", checkpoint, err)
	}
	if err := r.abort(); !errors.Is(err, errAborted) || len(r.results) != 0 {
		t.Fatalf("err=%v, results=%v", err, r.results)
	}
	if last := e.events[len(e.events)-1]; last != (sentEvent{"state", "3"}) {
		t.Fatalf("aborting must send the menu phase so nobody stays muted; last event %v", last)
	}
	if len(e.events) != 7 {
		t.Fatalf("events continued past the first checkpoint: %v", e.events)
	}
}

func TestFailedSendStopsScenario(t *testing.T) {
	e := &recordingEmitter{failAt: 3}
	log, err := playAll(newRun(newSession(e), scenarios[0], "Operator"), true)
	if !errors.Is(err, errSend) || len(e.events) != 3 || len(log) != 1 {
		t.Fatalf("failed send must stop the scenario: err=%v, events=%v, log=%v", err, e.events, log)
	}
}

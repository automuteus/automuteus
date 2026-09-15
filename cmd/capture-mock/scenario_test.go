package main

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func countChecks(sc scenario) int {
	n := 0
	for _, st := range sc.steps {
		if st.expect != "" {
			n++
		}
	}
	return n
}

// Every scenario must play through on "yes" answers, name the operator's player
// where it says {you}, and leave the bot in the lobby so the next scenario
// starts clean.
func TestScenariosPlayThrough(t *testing.T) {
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			e := &recordingEmitter{}
			var out bytes.Buffer
			p := newPrompts(strings.NewReader(strings.Repeat("y\n", countChecks(sc))), &out)
			results, err := runScenario(p, &session{client: e}, sc, "Operator")
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != countChecks(sc) || countChecks(sc) == 0 {
				t.Fatalf("recorded %d results for %d checkpoints", len(results), countChecks(sc))
			}
			if len(e.events) == 0 || e.events[0].name != "lobby" || e.events[len(e.events)-1] != (sentEvent{"state", "0"}) {
				t.Fatalf("scenario must open with a lobby and end in the lobby phase: %v", e.events)
			}
			if strings.Contains(out.String(), self) {
				t.Fatalf("operator name not substituted in output:\n%s", out.String())
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
	p := newPrompts(strings.NewReader(strings.Repeat("y\n", 10)), io.Discard)
	if _, err := runScenario(p, &session{client: e}, scenarios[0], "Operator"); err != nil {
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
	var sc scenario
	for _, candidate := range scenarios {
		if candidate.name == "Impostor victory" {
			sc = candidate
		}
	}
	p := newPrompts(strings.NewReader(strings.Repeat("y\n", countChecks(sc))), io.Discard)
	if _, err := runScenario(p, &session{client: e}, sc, "Operator"); err != nil {
		t.Fatal(err)
	}
	last := e.events[len(e.events)-2]
	if last.name != "gameover" || !strings.Contains(last.payload, `{"Name":"Operator","IsImpostor":true}`) || strings.Contains(last.payload, `{"Name":"Mock Blue","IsImpostor":true}`) {
		t.Fatalf("gameover roster = %s", last.payload)
	}
}

func TestQuitReturnsBotToMenu(t *testing.T) {
	e := &recordingEmitter{}
	p := newPrompts(strings.NewReader("q\n"), io.Discard)
	results, err := runScenario(p, &session{client: e}, scenarios[0], "Operator")
	if !errors.Is(err, errAborted) || len(results) != 0 {
		t.Fatalf("err=%v, results=%v", err, results)
	}
	if last := e.events[len(e.events)-1]; last != (sentEvent{"state", "3"}) {
		t.Fatalf("aborting must send the menu phase so nobody stays muted; last event %v", last)
	}
	if len(e.events) != 7 {
		t.Fatalf("events continued past the first checkpoint: %v", e.events)
	}
}

func TestIncompleteAnswersStopScenario(t *testing.T) {
	for _, input := range []string{"", "maybe\n", "y\n"} {
		e := &recordingEmitter{}
		p := newPrompts(strings.NewReader(input), io.Discard)
		_, err := runScenario(p, &session{client: e}, scenarios[0], "Operator")
		if !errors.Is(err, io.EOF) {
			t.Errorf("input %q: err=%v", input, err)
		}
		if last := e.events[len(e.events)-1]; last.name == "state" && last.payload == "3" {
			t.Errorf("input %q: EOF is not an operator quit, must not send the menu phase", input)
		}
	}
	e := &recordingEmitter{failAt: 3}
	_, err := runScenario(newPrompts(strings.NewReader("y\n"), io.Discard), &session{client: e}, scenarios[0], "Operator")
	if !errors.Is(err, errSend) || len(e.events) != 3 {
		t.Fatalf("failed send must stop the scenario: err=%v, events=%v", err, e.events)
	}
}

func TestReportMarksUnobservedChecks(t *testing.T) {
	var out bytes.Buffer
	report(&out, scenario{name: "Demo"}, []checkResult{{"first", true}, {"second", false}}, errAborted)
	got := out.String()
	for _, want := range []string{"Demo: 1/2 checks observed (scenario aborted)", "  ok   first", "  FAIL second"} {
		if !strings.Contains(got, want) {
			t.Fatalf("report missing %q:\n%s", want, got)
		}
	}
}

func TestMenuLoop(t *testing.T) {
	// Lobby-changes scenario (4 checks) answered with a mix, an invalid choice,
	// manual mode and back, then quit.
	lobbyChanges := len(scenarios)
	input := "Operator\n" + string(rune('0'+lobbyChanges)) + "\ny\nn\ny\ny\n99\nM\ns\n3\nq\nQ\n"
	e := &recordingEmitter{}
	var out bytes.Buffer
	if err := menuLoop(newPrompts(strings.NewReader(input), &out), &session{client: e}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Lobby changes: 3/4 checks observed") || !strings.Contains(out.String(), "Choose a scenario number, M, or Q.") {
		t.Fatalf("unexpected output:\n%s", out.String())
	}
	if last := e.events[len(e.events)-1]; last != (sentEvent{"state", "3"}) {
		t.Fatalf("manual mode did not send the chosen state: %v", e.events)
	}
	if err := menuLoop(newPrompts(strings.NewReader("Operator\n1\n"), io.Discard), &session{client: &recordingEmitter{}}); !errors.Is(err, io.EOF) {
		t.Fatalf("EOF inside a scenario must end the program: %v", err)
	}
}

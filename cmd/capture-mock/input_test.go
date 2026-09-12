package main

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestPromptsRetryInvalidValues(t *testing.T) {
	p := newPrompts(strings.NewReader(strings.Repeat("invalid\n", 10000)+"-1\n1\n99999999999999999999999\n5\nmaybe\nyes\n\nPlayer One\n"), io.Discard)
	if got := p.choice("Map", 0, map[int]string{0: "Skeld", 5: "Fungle"}); got != 5 {
		t.Fatalf("choice = %d", got)
	}
	if !p.boolean("Impostor", false) || p.boolean("Disconnected", false) {
		t.Fatal("boolean parsing or default failed")
	}
	if got := p.text("Name", "Player"); got != "Player One" || p.err != nil {
		t.Fatalf("name=%q, err=%v", got, p.err)
	}
	p.choice("Map", 0, map[int]string{0: "Skeld"})
	if !errors.Is(p.err, io.EOF) {
		t.Fatalf("EOF lost: %v", p.err)
	}
}

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestReadErrorStopsForm(t *testing.T) {
	e := &recordingEmitter{}
	err := commandLoop(newPrompts(brokenReader{}, io.Discard), &session{client: e})
	if !errors.Is(err, io.ErrUnexpectedEOF) || len(e.events) != 0 {
		t.Fatalf("err=%v, events=%v", err, e.events)
	}
}

// capture-mock sends capture-client events to Galactus for local development.
package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/automuteus/automuteus/v8/pkg/capture"
	socketio "github.com/hesh915/go-socket.io-client"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	var program *tea.Program
	// Socket callbacks only start after connecting, by which time program is set.
	m := newModel(func(msg tea.Msg) { program.Send(msg) })
	program = tea.NewProgram(m)
	final, err := program.Run()
	if err != nil {
		return err
	}
	return final.(*model).err
}

type connectedMsg struct{ client *socketio.Client }
type connectFailedMsg struct{ err error }

// fatalMsg ends the program with an error, e.g. when Galactus disconnects.
type fatalMsg struct{ err error }

// connect dials Galactus and sends the connect code. Socket events are
// delivered to the program through send, since they arrive on other goroutines.
func connect(host, code string, send func(tea.Msg)) tea.Cmd {
	return func() tea.Msg {
		client, err := socketio.NewClient(host, &socketio.Options{Transport: "websocket", Query: make(map[string]string)})
		if err != nil {
			return connectFailedMsg{fmt.Errorf("connect to Galactus: %w", err)}
		}
		handlers := map[string]interface{}{
			"disconnection": func() { send(fatalMsg{errors.New("disconnected from Galactus")}) },
			"error":         func() { send(logEntry{entryError, "Galactus reported a socket error"}) },
			"message":       func(msg string) { send(logEntry{entryGalactus, "Galactus: " + msg}) },
		}
		for event, handler := range handlers {
			if err := client.On(event, handler); err != nil {
				return connectFailedMsg{err}
			}
		}
		if err := client.Emit(capture.ConnectCodeEvent, code); err != nil {
			return connectFailedMsg{fmt.Errorf("send connect code: %w", err)}
		}
		return connectedMsg{client}
	}
}

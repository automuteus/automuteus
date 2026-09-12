// capture-mock sends capture-client events to Galactus for local development.
package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/automuteus/automuteus/v8/pkg/capture"
	"github.com/automuteus/automuteus/v8/pkg/game"
	socketio "github.com/hesh915/go-socket.io-client"
)

func main() {
	if err := run(); err != nil && !errors.Is(err, io.EOF) {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	p := newPrompts(os.Stdin, os.Stdout)
	var host, code string
	for {
		input := p.text("Connect code or aucapture:// link", "")
		if p.err != nil {
			return p.err
		}
		var err error
		host, code, err = parseConnection(input)
		if err == nil {
			break
		}
		fmt.Fprintln(p.out, err)
	}
	log.Printf("Connecting to %s", host)
	client, err := socketio.NewClient(host, &socketio.Options{Transport: "websocket", Query: make(map[string]string)})
	if err != nil {
		return fmt.Errorf("connect to Galactus: %w", err)
	}
	disconnected := make(chan struct{}, 1)
	if err := client.On("disconnection", func() {
		select {
		case disconnected <- struct{}{}:
		default:
		}
	}); err != nil {
		return err
	}
	if err := client.On("error", func() { log.Print("Galactus reported a socket error") }); err != nil {
		return err
	}
	if err := client.On("message", func(msg string) { log.Printf("Galactus: %s", msg) }); err != nil {
		return err
	}
	if err := client.Emit(capture.ConnectCodeEvent, code); err != nil {
		return fmt.Errorf("send connect code: %w", err)
	}
	log.Print("Connect code sent; ready to send events")
	interrupted := make(chan os.Signal, 1)
	signal.Notify(interrupted, os.Interrupt)
	defer signal.Stop(interrupted)
	done := make(chan error, 1)
	go func() { done <- commandLoop(p, &session{client: client}) }()
	select {
	case err := <-done:
		return err
	case <-disconnected:
		return errors.New("disconnected from Galactus")
	case <-interrupted:
		return nil
	}
}

func commandLoop(p *prompts, s *session) error {
	for {
		command := p.text("L Lobby / S State / P Player / G Gameover / Q Quit", "")
		if p.err != nil {
			return p.err
		}
		var send func() error
		switch strings.ToUpper(command) {
		case "Q":
			return nil
		case "L":
			lobby := game.Lobby{LobbyCode: p.text("Lobby code", "TESTCODE")}
			lobby.Region = game.Region(p.choice("Region", int(game.NA), map[int]string{
				int(game.NA): game.NA.ToString(), int(game.AS): game.AS.ToString(), int(game.EU): game.EU.ToString(),
			}))
			maps := make(map[int]string)
			for value, label := range game.MapNames {
				maps[int(value)] = label
			}
			lobby.PlayMap = game.PlayMap(p.choice("Map", int(game.SKELD), maps))
			send = func() error { return s.lobby(lobby) }
		case "S":
			phases := make(map[int]string)
			for value, label := range game.PhaseNames {
				phases[int(value)] = string(label)
			}
			phase := game.Phase(p.choice("State", int(game.LOBBY), phases))
			send = func() error { return s.phase(phase) }
		case "P":
			player := game.Player{Action: game.PlayerAction(p.choice("Player action", int(game.JOINED), map[int]string{
				int(game.JOINED): "JOINED", int(game.LEFT): "LEFT", int(game.DIED): "DIED",
				int(game.CHANGECOLOR): "CHANGECOLOR", int(game.FORCEUPDATED): "FORCEUPDATED",
				int(game.DISCONNECTED): "DISCONNECTED", int(game.EXILED): "EXILED",
			}))}
			player.Name = p.text("Player name", "Player")
			colors := make(map[int]string)
			for label, value := range game.ColorStrings {
				colors[value] = label
			}
			player.Color = p.choice("Color", game.Red, colors)
			player.IsDead = p.boolean("Is dead?", player.Action == game.DIED || player.Action == game.EXILED)
			player.Disconnected = p.boolean("Disconnected?", player.Action == game.DISCONNECTED)
			impostor := p.boolean("Is impostor? (sent at gameover)", s.isImpostor(player.Name))
			send = func() error { return s.player(player, impostor) }
		case "G":
			result := game.GameResult(p.choice("Game result", int(game.HumansByVote), map[int]string{
				int(game.HumansByVote): "HumansByVote", int(game.HumansByTask): "HumansByTask",
				int(game.ImpostorByVote): "ImpostorByVote", int(game.ImpostorByKill): "ImpostorByKill",
				int(game.ImpostorBySabotage): "ImpostorBySabotage", int(game.ImpostorDisconnect): "ImpostorDisconnect",
				int(game.HumansDisconnect): "HumansDisconnect", int(game.Unknown): "Unknown",
			}))
			send = func() error { return s.gameover(result) }
		default:
			fmt.Fprintln(p.out, "Choose L, S, P, G, or Q.")
			continue
		}
		if p.err != nil {
			return p.err
		}
		// Stop on a failed send: continuing would hide a partially sent sequence.
		if err := send(); err != nil {
			return err
		}
	}
}

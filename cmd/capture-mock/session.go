package main

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/automuteus/automuteus/v8/pkg/capture"
	"github.com/automuteus/automuteus/v8/pkg/game"
)

type emitter interface {
	Emit(string, ...interface{}) error
}

type session struct {
	client  emitter
	players []game.PlayerInfo
	view    gameView
}

// gameView mirrors what has been sent since the last lobby, for the status
// panel. It only changes after a successful send.
type gameView struct {
	phase    game.Phase
	lobby    game.Lobby
	hasLobby bool
	// players holds each present player's latest event; leaving or
	// disconnecting removes them.
	players []game.Player
}

func newSession(client emitter) *session {
	return &session{client: client, view: gameView{phase: game.UNINITIALIZED}}
}

func (s *session) send(event string, payload interface{}) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode %s: %w", event, err)
	}
	if err := s.client.Emit(event, string(b)); err != nil {
		return fmt.Errorf("send %s: %w", event, err)
	}
	return nil
}

func (s *session) phase(phase game.Phase) error {
	if err := s.client.Emit(capture.StateEvent, strconv.Itoa(int(phase))); err != nil {
		return fmt.Errorf("send state: %w", err)
	}
	s.view.phase = phase
	return nil
}

func (s *session) lobby(lobby game.Lobby) error {
	if err := s.send(capture.LobbyEvent, lobby); err != nil {
		return err
	}
	s.players = nil
	s.view.lobby, s.view.hasLobby, s.view.players = lobby, true, nil
	// Emit in order on the same connection; no arbitrary delay is needed.
	return s.phase(game.LOBBY)
}

func (s *session) player(player game.Player, impostor bool) error {
	if err := s.send(capture.PlayerEvent, player); err != nil {
		return err
	}
	s.view.update(player)
	for i := range s.players {
		if s.players[i].Name == player.Name {
			s.players[i].IsImpostor = impostor
			return nil
		}
	}
	// Keep departed players too: they still participated in this round.
	s.players = append(s.players, game.PlayerInfo{Name: player.Name, IsImpostor: impostor})
	return nil
}

func (s *session) isImpostor(name string) bool {
	for _, player := range s.players {
		if player.Name == name {
			return player.IsImpostor
		}
	}
	return false
}

func (s *session) gameover(result game.GameResult) error {
	players := s.players
	if players == nil {
		players = []game.PlayerInfo{}
	}
	if err := s.send(capture.GameOverEvent, game.Gameover{GameOverReason: result, PlayerInfos: players}); err != nil {
		return err
	}
	s.players = nil
	// Everyone is back in the lobby and alive, as in the game.
	for i := range s.view.players {
		s.view.players[i].IsDead = false
	}
	return s.phase(game.LOBBY)
}

func (v *gameView) update(player game.Player) {
	for i := range v.players {
		if v.players[i].Name == player.Name {
			if player.Action == game.LEFT || player.Action == game.DISCONNECTED {
				v.players = append(v.players[:i], v.players[i+1:]...)
			} else {
				v.players[i] = player
			}
			return
		}
	}
	if player.Action != game.LEFT && player.Action != game.DISCONNECTED {
		v.players = append(v.players, player)
	}
}

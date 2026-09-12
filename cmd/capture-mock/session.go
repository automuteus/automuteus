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
	return nil
}

func (s *session) lobby(lobby game.Lobby) error {
	if err := s.send(capture.LobbyEvent, lobby); err != nil {
		return err
	}
	s.players = nil
	// Emit in order on the same connection; no arbitrary delay is needed.
	return s.phase(game.LOBBY)
}

func (s *session) player(player game.Player, impostor bool) error {
	if err := s.send(capture.PlayerEvent, player); err != nil {
		return err
	}
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
	return s.phase(game.LOBBY)
}

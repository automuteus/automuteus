package amongus

import (
	"github.com/j0nas500/automuteus/v8/pkg/game"
)

type PlayerData struct {
	Color   int    `json:"color"`
	Name    string `json:"name"`
	IsAlive bool   `json:"isAlive"`
}

const UnlinkedPlayerName = "UnlinkedPlayer"

var UnlinkedPlayer = PlayerData{
	Color:   -1,
	Name:    UnlinkedPlayerName,
	IsAlive: true,
}

func (auData *PlayerData) isDifferent(player game.Player) bool {
	return auData.IsAlive != !player.IsDead || auData.Color != player.Color || auData.Name != player.Name
}

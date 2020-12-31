package amongus

import (
	"fmt"
	"github.com/automuteus/utils/pkg/game"
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

// ToString a user
func (auData *PlayerData) ToString() string {
	return fmt.Sprintf("{ Name: %s, Color: %s, Alive: %v }\n", auData.Name, game.GetColorStringForInt(auData.Color), auData.IsAlive)
}

func (auData *PlayerData) isDifferent(player game.Player) bool {
	return auData.IsAlive != !player.IsDead || auData.Color != player.Color || auData.Name != player.Name
}

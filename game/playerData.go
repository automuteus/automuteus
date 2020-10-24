package game

import "fmt"

//TODO make this private?
// AmongUserData struct
type PlayerData struct {
	Color   int    `json:"color"`
	Name    string `json:"name"`
	IsAlive bool   `json:"isAlive"`
}

// ToString a user
func (auData *PlayerData) ToString() string {
	return fmt.Sprintf("{ Name: %s, Color: %s, Alive: %v }\n", auData.Name, GetColorStringForInt(auData.Color), auData.IsAlive)
}

func (auData *PlayerData) isDifferent(player Player) bool {
	return auData.IsAlive != !player.IsDead || auData.Color != player.Color || auData.Name != player.Name
}

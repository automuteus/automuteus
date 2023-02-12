package game

type PlayerAction int

const (
	JOINED PlayerAction = iota
	LEFT
	DIED
	CHANGECOLOR
	FORCEUPDATED
	DISCONNECTED
	EXILED
)

// Player struct
type Player struct {
	Action       PlayerAction `json:"Action"`
	Name         string       `json:"Name"`
	Color        int          `json:"Color"`
	IsDead       bool         `json:"IsDead"`
	Disconnected bool         `json:"Disconnected"`
}

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
	// FriendCode is the player's account Friend Code (like "coolbean#1234"). Unlike Name, it's unique and doesn't
	// change between games. Empty when the Capture doesn't report it, or the player has none.
	FriendCode string `json:"FriendCode,omitempty"`
}

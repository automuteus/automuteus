package game

// Phase type
type Phase int

// Phase constants
const (
	LOBBY   Phase = iota
	TASKS   Phase = iota
	DISCUSS Phase = iota
	MENU    Phase = iota
	//VOTING        Phase = iota
	//GAMEOVER      Phase = iota
	UNINITIALIZED Phase = iota
)

type PlayerAction int

const (
	JOINED       PlayerAction = iota
	LEFT         PlayerAction = iota
	DIED         PlayerAction = iota
	CHANGECOLOR  PlayerAction = iota
	FORCEUPDATED PlayerAction = iota
	DISCONNECTED PlayerAction = iota
	EXILED       PlayerAction = iota
)

type PhaseNameString string

// PhaseNames for lowercase, possibly for translation if needed
var PhaseNames = map[Phase]PhaseNameString{
	LOBBY:   "LOBBY",
	TASKS:   "TASKS",
	DISCUSS: "DISCUSSION",
	MENU:    "MENU",
}

// ToString for a phase
func (phase *Phase) ToString() PhaseNameString {
	return PhaseNames[*phase]
}

// Player struct
type Player struct {
	Action       PlayerAction `json:"Action"`
	Name         string       `json:"Name"`
	Color        int          `json:"Color"`
	IsDead       bool         `json:"IsDead"`
	Disconnected bool         `json:"Disconnected"`
}

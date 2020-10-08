package game

import "strings"

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

type Region int

const (
	NA Region = iota
	AS
	EU
)

type Lobby struct {
	LobbyCode string `json:"LobbyCode"`
	Region    Region `json:"Region"`
}

func (l *Lobby) ReduceLobbyCode() {
	l.LobbyCode = strings.Replace(l.LobbyCode, "Code\r\n", "", 1)
}

func (r Region) ToString() string {
	switch r {
	case NA:
		return "North America"
	case EU:
		return "Europe"
	case AS:
		return "Asia"
	}
	return "Unknown"
}

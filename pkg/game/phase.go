package game

// Phase type
type Phase int

// Phase constants
const (
	LOBBY Phase = iota
	TASKS
	DISCUSS
	MENU
	GAMEOVER
	UNINITIALIZED
)

type PhaseNameString string

// PhaseNames for lowercase, possibly for translation if needed
var PhaseNames = map[Phase]PhaseNameString{
	LOBBY:   "LOBBY",
	TASKS:   "TASKS",
	DISCUSS: "DISCUSSION",
	MENU:    "MENU",
}

// ToString for a Phase
func (phase *Phase) ToString() PhaseNameString {
	return PhaseNames[*phase]
}

package game

type GamePhase int

const (
	LOBBY         GamePhase = iota
	TASKS         GamePhase = iota
	DISCUSS       GamePhase = iota
	//VOTING        GamePhase = iota
	//GAMEOVER      GamePhase = iota
	//UNINITIALIZED GamePhase = iota
	//MENU          GamePhase = iota
)

//var PhaseStrings = []string{
//	"UNINITIALIZED",
//	"MENU",
//	"LOBBY",
//	"GAME",
//	"DISCUSS",
//	"VOTING",
//	"GAMEOVER",
//}

type Player struct {
	Action   int `json:"Action"`
	Name   string `json:"Name"`
	Color  int `json:"Color"`
	IsDead bool   `json:"IsDead"`
	Disconnected bool `json:"Disconnected"`
}

type PlayerUpdate struct {
	Player Player
	GuildID string
}

type GamePhaseUpdate struct {
	Phase GamePhase
	GuildID string
}

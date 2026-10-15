package game

import "encoding/json"

type GameResult int16

// Non-negative results are Among Us' own GameOverReason values, which capture clients pass through unchanged. They
// are named for the side that won; the game names 5 and 6 for the side that disconnected, so ours read swapped.
const (
	HumansByVote GameResult = iota
	HumansByTask
	ImpostorByVote
	ImpostorByKill
	ImpostorBySabotage
	HumansDisconnect   // the game's ImpostorDisconnect: the impostors left
	ImpostorDisconnect // the game's CrewmateDisconnect: too many crewmates left
	HideAndSeekHumansByTimer
	HideAndSeekImpostorByKills
)

// Negative results are AutoMuteUs' own and never collide with the game's.
const (
	// Aborted is recorded as the win type of a match that was ended before the game reported a result, e.g. by an
	// admin running /end or by the platform shutting down. Aborted matches are excluded from statistics.
	Aborted GameResult = -2
	// UnknownEnding is reported by a capture client for a game end it can't name, such as a role mod's custom win.
	UnknownEnding GameResult = -3
)

var (
	HumanWins    = []GameResult{HumansByVote, HumansByTask, HumansDisconnect, HideAndSeekHumansByTimer}
	ImpostorWins = []GameResult{ImpostorByVote, ImpostorByKill, ImpostorBySabotage, ImpostorDisconnect, HideAndSeekImpostorByKills}
)

// Winner reports whether the impostors won, and whether either side did. Only matches one side won count toward
// statistics; UnknownEnding and results this version doesn't know are recorded without a winner.
func (r GameResult) Winner() (impostors, decided bool) {
	for _, w := range ImpostorWins {
		if r == w {
			return true, true
		}
	}
	for _, w := range HumanWins {
		if r == w {
			return false, true
		}
	}
	return false, false
}

func (r *Gameover) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type Gameover struct {
	GameOverReason GameResult   `json:"GameOverReason"`
	PlayerInfos    []PlayerInfo `json:"PlayerInfos"`
}

type PlayerInfo struct {
	Name       string `json:"Name"`
	IsImpostor bool   `json:"IsImpostor"`
}

type GameRole int16

const (
	CrewmateRole GameRole = iota
	ImposterRole
)

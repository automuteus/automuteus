package game

import "encoding/json"

type GameResult int16

const (
	HumansByVote GameResult = iota
	HumansByTask
	ImpostorByVote
	ImpostorByKill
	ImpostorBySabotage
	ImpostorDisconnect
	HumansDisconnect
	Unknown
)

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

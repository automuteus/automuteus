package capture

type EventType int

const (
	Connection EventType = iota
	Lobby
	State
	Player
	GameOver
)

type Event struct {
	EventType EventType `json:"type"`
	Payload   []byte    `json:"payload"`
}

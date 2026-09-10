package capture

import (
	"time"
)

const DefaultCaptureBotTimeout = time.Second

type EventType int

const (
	Connection EventType = iota
	Lobby
	State
	Player
	GameOver
)

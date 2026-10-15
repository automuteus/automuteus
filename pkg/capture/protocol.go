package capture

// Socket.IO event names shared by Galactus and capture clients. Structured
// payloads are JSON-encoded strings; state is a decimal-encoded game.Phase.
const (
	ConnectCodeEvent  = "connectCode"
	BotIDEvent        = "botID"
	LobbyEvent        = "lobby"
	StateEvent        = "state"
	PlayerEvent       = "player"
	GameOverEvent     = "gameover"
	TaskCompleteEvent = "taskComplete"
	TaskFailedEvent   = "taskFailed"
	// KillSelfEvent tells a capture client it was replaced and must quit rather than reconnect.
	KillSelfEvent     = "killself"
	ConnectCodeLength = 8
)

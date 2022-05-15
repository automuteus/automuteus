package locales

import "github.com/automuteus/automuteus/discord/setting"

type LocalizedFields struct {
	// somewhat global fields
	User      string
	GameState string
	View      string
	Clear     string

	// command-specific fields here
	DebugViewDesc          string
	DebugViewUserDesc      string
	DebugViewUserCacheDesc string
	DebugViewGameStateDesc string
	DebugClearUserDesc     string
	DebugClearDesc         string
	DebugUnmuteAllName     string
	DebugUnmuteAllDesc     string
}

var DefaultFields = LocalizedFields{
	"user",
	"game-state",
	setting.View,
	setting.Clear,

	"View debug info",
	"User Cache",
	"User whose cache you want to view",
	"Game State",
	"User whose cache should be cleared",
	"Clear debug info",
	"unmute-all",
	"Unmute all players",
}

package discord

type GameStateRequest struct {
	GuildID      string
	TextChannel  string
	VoiceChannel string
	ConnectCode  string
}

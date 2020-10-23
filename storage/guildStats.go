package storage

type GuildStats struct {
	GuildID     string `json:"guildID"`
	GamesPlayed int    `json:"gamesPlayed"`
}

func MakeGuildStats(guildID string) *GuildStats {
	return &GuildStats{
		GuildID:     guildID,
		GamesPlayed: 0,
	}
}

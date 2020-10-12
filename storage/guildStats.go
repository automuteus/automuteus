package storage

type GuildStats struct {
	gamesPlayed int
}

func MakeGuildStats() GuildStats {
	return GuildStats{}
}

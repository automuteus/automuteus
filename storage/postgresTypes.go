package storage

type PostgresGuild struct {
	GuildID   string `db:"guild_id"`
	GuildName string `db:"guild_name"`
	Premium   string `db:"premium"`
}

type PostgresGame struct {
	GameID      int32  `db:"game_id"`
	ConnectCode string `db:"connect_code"`
	StartTime   int32  `db:"start_time"`
	WinType     string `db:"win_type"`
	EndTime     int32  `db:"end_time"`
}

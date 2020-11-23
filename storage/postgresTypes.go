package storage

type PostgresGuild struct {
	GuildID   string `db:"guild_id"`
	GuildName string `db:"guild_name"`
	Premium   string `db:"premium"`
}

type PostgresGame struct {
	GameID      int64  `db:"game_id"`
	ConnectCode string `db:"connect_code"`
	StartTime   int64  `db:"start_time"`
	WinType     int16  `db:"win_type"`
	EndTime     int64  `db:"end_time"`
}

type PostgresUser struct {
	UserID       string `db:"user_id"`
	HashedUserID string `db:"hashed_user_id"`
}

type PostgresGuildUser struct {
	GuildID      string `db:"guild_id"`
	HashedUserID string `db:"hashed_user_id"`
}

type PostgresGuildGame struct {
	GuildID string `db:"guild_id"`
	GameID  int64  `db:"game_id"`
}

type PostgresUserGame struct {
	HashedUserID string `db:"hashed_user_id"`
	GameID       int64  `db:"game_id"`
	PlayerName   string `db:"player_name"`
	PlayerColor  int16  `db:"player_color"`
	PlayerRole   string `db:"player_role"`
}

type PostgresGameEvent struct {
	//Note, we don't include eventID here because it gets decided/incremented by postgres
	GameID    int64  `db:"game_id"`
	EventTime int64  `db:"event_time"`
	EventType int16  `db:"event_type"`
	Payload   string `db:"payload"`
}

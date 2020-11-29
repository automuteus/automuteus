package storage

type PostgresGuild struct {
	GuildID   uint64 `db:"guild_id"`
	GuildName string `db:"guild_name"`
	Premium   int16  `db:"premium"`
}

type PostgresGame struct {
	GameID      int64  `db:"game_id"`
	GuildID     uint64 `db:"guild_id"`
	ConnectCode string `db:"connect_code"`
	StartTime   int32  `db:"start_time"`
	WinType     int16  `db:"win_type"`
	EndTime     int32  `db:"end_time"`
}

type PostgresUser struct {
	UserID uint64 `db:"user_id"`
	Opt    bool   `db:"opt"`
}

type PostgresGuildUser struct {
	GuildID      uint64 `db:"guild_id"`
	HashedUserID int64  `db:"hashed_user_id"`
}

type PostgresUserGame struct {
	UserID      uint64 `db:"user_id"`
	GuildID     uint64 `db:"guild_id"`
	GameID      int64  `db:"game_id"`
	PlayerName  string `db:"player_name"`
	PlayerColor int16  `db:"player_color"`
	PlayerRole  int16  `db:"player_role"`
	PlayerWon   bool   `db:"player_won"`
}

type PostgresGameEvent struct {
	//Note, we don't include eventID here because it gets decided/incremented by postgres
	UserID    uint64 `db:"hashed_user_id"`
	GameID    int64  `db:"game_id"`
	EventTime int32  `db:"event_time"`
	EventType int16  `db:"event_type"`
	Payload   string `db:"payload"`
}

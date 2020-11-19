package storage

type PostgresGuild struct {
	GuildID   string `db:"guild_id"`
	GuildName string `db:"guild_name"`
	Premium   string `db:"premium"`
}

type PostgresGame struct {
	GameID      int32  `db:"game_id"`
	ConnectCode string `db:"connect_code"`
	StartTime   int64  `db:"start_time"`
	WinType     string `db:"win_type"`
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
	GameID  int32  `db:"game_id"`
}

type PostgresUserGame struct {
	HashedUserID string `db:"hashed_user_id"`
	GameID       int32  `db:"game_id"`
	PlayerName   string `db:"player_name"`
	PlayerColor  int32  `db:"player_color"`
	PlayerRole   string `db:"player_role"`
	Winner       bool   `db:"winner"`
}

package storage

import (
	"bytes"
	"fmt"
)

type PostgresGuild struct {
	GuildID       uint64  `db:"guild_id"`
	GuildName     string  `db:"guild_name"`
	Premium       int16   `db:"premium"`
	TxTimeUnix    *int32  `db:"tx_time_unix"`
	TransferredTo *uint64 `db:"transferred_to"`
	InheritsFrom  *uint64 `db:"inherits_from"`
}

func nilToEmpty[T int32 | uint64](s *T) string {
	if s == nil {
		return ""
	} else {
		return fmt.Sprintf("%v", *s)
	}
}

func (g *PostgresGuild) ToCSV() string {
	return fmt.Sprintf("guild_id,guild_name,premium,tx_time_unix,transferred_to,inherits_from,\n"+
		"%d,%s,%d,%s,%s,%s\n", g.GuildID, g.GuildName, g.Premium,
		nilToEmpty(g.TxTimeUnix), nilToEmpty(g.TransferredTo), nilToEmpty(g.InheritsFrom))
}

type PostgresGame struct {
	GameID      int64  `db:"game_id"`
	GuildID     uint64 `db:"guild_id"`
	ConnectCode string `db:"connect_code"`
	StartTime   int32  `db:"start_time"`
	WinType     int16  `db:"win_type"`
	EndTime     int32  `db:"end_time"`
}

func GamesToCSV(g []*PostgresGame) string {
	s := bytes.NewBufferString("game_id,guild_id,connect_code,start_time,win_type,end_time,\n")
	for _, v := range g {
		if v != nil {
			s.WriteString(fmt.Sprintf("%d,%d,%s,%d,%d,%d,\n",
				v.GameID, v.GuildID, v.ConnectCode, v.StartTime, v.WinType, v.EndTime))
		}
	}
	return s.String()
}

type PostgresUser struct {
	UserID       uint64 `db:"user_id"`
	Opt          bool   `db:"opt"`
	VoteTimeUnix *int32 `db:"vote_time_unix"`
}

func UsersToCSV(u []*PostgresUser) string {
	s := bytes.NewBufferString("user_id,opt,vote_time_unix,\n")
	for _, v := range u {
		if v != nil {
			s.WriteString(fmt.Sprintf("%d,%t,%s,\n", v.UserID, v.Opt, nilToEmpty(v.VoteTimeUnix)))
		}
	}
	return s.String()
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

func UsersGamesToCSV(ug []*PostgresUserGame) string {
	s := bytes.NewBufferString("user_id,guild_id,game_id,player_name,player_color,player_role,player_won,\n")
	for _, v := range ug {
		if v != nil {
			s.WriteString(fmt.Sprintf("%d,%d,%d,%s,%d,%d,%t,\n",
				v.UserID, v.GuildID, v.GameID, v.PlayerName, v.PlayerColor, v.PlayerRole, v.PlayerWon))
		}
	}
	return s.String()
}

type PostgresGameEvent struct {
	EventID   uint64  `db:"event_id"`
	UserID    *uint64 `db:"user_id"`
	GameID    int64   `db:"game_id"`
	EventTime int32   `db:"event_time"`
	EventType int16   `db:"event_type"`
	Payload   string  `db:"payload"`
}

func EventsToCSV(e []*PostgresGameEvent) string {
	s := bytes.NewBufferString("event_id,user_id,game_id,event_time,event_type,payload,\n")
	for _, v := range e {
		if v != nil {
			s.WriteString(fmt.Sprintf("%d,%s,%d,%d,%d,%s,\n",
				v.EventID, nilToEmpty(v.UserID), v.GameID, v.EventTime, v.EventType, v.Payload))
		}
	}
	return s.String()
}

type PostgresOtherPlayerRanking struct {
	UserID  uint64  `db:"user_id"`
	Count   int64   `db:"count"`
	Percent float64 `db:"percent"`
}

type PostgresPlayerRanking struct {
	UserID   uint64  `db:"user_id"`
	WinCount int64   `db:"win"`
	Count    int64   `db:"total"`
	WinRate  float64 `db:"win_rate"`
}

type PostgresBestTeammatePlayerRanking struct {
	UserID     uint64  `db:"user_id"`
	TeammateID uint64  `db:"teammate_id"`
	WinCount   int64   `db:"win"`
	Count      int64   `db:"total"`
	WinRate    float64 `db:"win_rate"`
}

type PostgresWorstTeammatePlayerRanking struct {
	UserID     uint64  `db:"user_id"`
	TeammateID uint64  `db:"teammate_id"`
	LooseCount int64   `db:"loose"`
	Count      int64   `db:"total"`
	LooseRate  float64 `db:"loose_rate"`
}

type PostgresUserActionRanking struct {
	UserID      uint64  `db:"user_id"`
	TotalAction int64   `db:"total_action"`
	Count       int64   `db:"total"`
	WinRate     float64 `db:"win_rate"`
}

type PostgresUserMostFrequentFirstTargetRanking struct {
	UserID     uint64  `db:"user_id"`
	TotalDeath int64   `db:"total_death"`
	Count      int64   `db:"total"`
	DeathRate  float64 `db:"death_rate"`
}

type PostgresUserMostFrequentKilledByanking struct {
	UserID     uint64  `db:"user_id"`
	TeammateID uint64  `db:"teammate_id"`
	TotalDeath int64   `db:"total_death"`
	Encounter  int64   `db:"encounter"`
	DeathRate  float64 `db:"death_rate"`
}

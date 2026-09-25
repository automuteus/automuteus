package api

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/capture"
	"github.com/automuteus/automuteus/v8/pkg/game"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	pgstorage "github.com/automuteus/automuteus/v8/pkg/storage"
	"github.com/gin-gonic/gin"
	"github.com/pashagolub/pgxmock"
)

const testMatchNum = int64(42)

var gameColumns = []string{"game_id", "guild_id", "connect_code", "start_time", "win_type", "end_time", "play_map", "region"}

func matchRow(winType int16, end int32, playMap, region *int16) *pgxmock.Rows {
	return pgxmock.NewRows(gameColumns).AddRow(testMatchNum, testGuildNum, "ABCDEFGH", int32(1000), winType, end, playMap, region)
}

func int16p(v int16) *int16 { return &v }

func matchPlayerRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"user_id", "guild_id", "game_id", "player_name", "player_color", "player_role", "player_won"}).
		AddRow(uint64(22), testGuildNum, testMatchNum, "Imp", int16(game.Red), int16(game.ImposterRole), true).
		AddRow(uint64(11), testGuildNum, testMatchNum, "Crew", int16(game.Blue), int16(game.CrewmateRole), false)
}

var eventColumns = []string{"event_id", "user_id", "game_id", "event_time", "event_type", "payload"}

func expectMatchEvents(mock pgxmock.PgxPoolIface, rows *pgxmock.Rows) {
	mock.ExpectQuery(`FROM game_events WHERE game_id = \$1 AND event_type IN \(\$2, \$3, \$4\) ORDER BY event_time, event_id`).
		WithArgs(testMatchNum, int16(capture.State), int16(capture.Player), int16(capture.GameOver)).
		WillReturnRows(rows)
}

func playerPayload(action game.PlayerAction, name string, color int) string {
	b, _ := json.Marshal(game.Player{Action: action, Name: name, Color: color})
	return string(b)
}

func TestBuildMatchSummary_FreeGuildGetsHeaderAndRoster(t *testing.T) {
	mock := newStatsMock(t)
	mock.ExpectQuery(`FROM games WHERE game_id = \$1 AND guild_id = \$2`).
		WithArgs(testMatchNum, testGuildNum).
		WillReturnRows(matchRow(int16(game.ImpostorByKill), 1300, int16p(int16(game.POLUS)), int16p(int16(game.EU))))
	mock.ExpectQuery(`FROM users_games WHERE game_id = \$1`).WithArgs(testMatchNum).WillReturnRows(matchPlayerRows())
	expectMatchEvents(mock, pgxmock.NewRows(eventColumns).
		AddRow(uint64(1), (*uint64)(nil), testMatchNum, int32(1005), int16(capture.State), "1").
		AddRow(uint64(2), (*uint64)(nil), testMatchNum, int32(1300), int16(capture.GameOver),
			`{"GameOverReason":3,"PlayerInfos":[{"Name":"imp","IsImpostor":true},{"Name":"Crew","IsImpostor":false},{"Name":"Guest","IsImpostor":false}]}`))

	m, err := buildMatchSummary(context.Background(), mock, nil, nil, testGuildID, "42", premium.PremiumRecord{Tier: premium.FreeTier})
	if err != nil {
		t.Fatal(err)
	}
	if m.Status != "finished" || m.StartTime != 1000 || m.EndTime != 1300 || m.Result != "impostorKill" || m.Winner != "impostor" {
		t.Fatalf("header = %+v", m)
	}
	if m.Map != "polus" || m.Region != "eu" {
		t.Fatalf("map %q region %q", m.Map, m.Region)
	}
	// the game over report completes the roster for free guilds too; "imp" is Imp, matched regardless of case
	want := []MatchPlayer{
		{UserID: "22", Name: "Imp", Color: "red", Role: "impostor", Won: true},
		{UserID: "11", Name: "Crew", Color: "blue", Role: "crewmate", Won: false},
		{Name: "Guest", Role: "crewmate", Won: false},
	}
	if !reflect.DeepEqual(m.Roster, want) || !m.RosterComplete {
		t.Fatalf("roster = %+v complete=%v", m.Roster, m.RosterComplete)
	}
	if m.Timeline != nil {
		t.Fatal("free guild received a timeline")
	}
	body, _ := json.Marshal(m)
	if strings.Contains(string(body), "timeline") || strings.Contains(string(body), "ABCDEFGH") {
		t.Fatalf("free response names the timeline or the connect code: %s", body)
	}
}

func TestBuildMatchSummary_PremiumGetsTimeline(t *testing.T) {
	mock := newStatsMock(t)
	mock.ExpectQuery(`FROM games WHERE game_id`).WithArgs(testMatchNum, testGuildNum).
		WillReturnRows(matchRow(int16(game.HumansByVote), 1300, nil, nil))
	mock.ExpectQuery(`FROM users_games WHERE game_id`).WithArgs(testMatchNum).WillReturnRows(matchPlayerRows())
	expectMatchEvents(mock, pgxmock.NewRows(eventColumns).
		AddRow(uint64(1), (*uint64)(nil), testMatchNum, int32(1005), int16(capture.State), "1"))

	m, err := buildMatchSummary(context.Background(), mock, nil, nil, testGuildID, "42", premium.PremiumRecord{Tier: premium.GoldTier, Days: 5})
	if err != nil {
		t.Fatal(err)
	}
	if m.Result != "crewmateVote" || m.Winner != "crewmate" || m.Map != "" || m.Region != "" {
		t.Fatalf("header = %+v", m)
	}
	if m.Timeline == nil || len(m.Timeline.Events) != 1 || m.Timeline.Events[0] != (MatchEvent{Offset: 5, Type: "tasks"}) {
		t.Fatalf("timeline = %+v", m.Timeline)
	}
	// no game over report was kept, so only the linked players are known
	if len(m.Roster) != 2 || m.RosterComplete {
		t.Fatalf("roster = %+v complete=%v", m.Roster, m.RosterComplete)
	}
}

func TestBuildMatchSummary_Status(t *testing.T) {
	for _, tc := range []struct {
		name           string
		winType        int16
		end            int32
		status, result string
		winner         string
		endTime        int64
	}{
		{"in progress", -1, -1, "inProgress", "", "", 0},
		{"aborted", int16(game.Aborted), 1200, "aborted", "", "", 1200},
		{"unknown result", int16(game.Unknown), 1200, "finished", "unknown", "", 1200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock := newStatsMock(t)
			mock.ExpectQuery(`FROM games WHERE game_id`).WithArgs(testMatchNum, testGuildNum).WillReturnRows(matchRow(tc.winType, tc.end, nil, nil))
			mock.ExpectQuery(`FROM users_games WHERE game_id`).WithArgs(testMatchNum).
				WillReturnRows(pgxmock.NewRows([]string{"user_id", "guild_id", "game_id", "player_name", "player_color", "player_role", "player_won"}))
			expectMatchEvents(mock, pgxmock.NewRows(eventColumns))
			m, err := buildMatchSummary(context.Background(), mock, nil, nil, testGuildID, "42", premium.PremiumRecord{Tier: premium.FreeTier})
			if err != nil {
				t.Fatal(err)
			}
			if m.Status != tc.status || m.Result != tc.result || m.Winner != tc.winner || m.EndTime != tc.endTime {
				t.Fatalf("got status %q result %q winner %q end %d", m.Status, m.Result, m.Winner, m.EndTime)
			}
			body, _ := json.Marshal(m)
			if !strings.Contains(string(body), `"roster":[]`) {
				t.Fatalf("roster should be an empty array: %s", body)
			}
		})
	}
}

func TestBuildMatchSummary_OtherGuildsMatchIsNotFound(t *testing.T) {
	mock := newStatsMock(t)
	mock.ExpectQuery(`FROM games WHERE game_id`).WithArgs(testMatchNum, testGuildNum).WillReturnRows(pgxmock.NewRows(gameColumns))
	_, err := buildMatchSummary(context.Background(), mock, nil, nil, testGuildID, "42", premium.PremiumRecord{Tier: premium.GoldTier, Days: 5})
	if !errors.Is(err, errMatchNotFound) {
		t.Fatalf("err = %v, want errMatchNotFound", err)
	}
}

func TestBuildMatchRoster(t *testing.T) {
	linked := []*pgstorage.PostgresUserGame{
		{UserID: 11, PlayerName: "Crew", PlayerColor: int16(game.Blue), PlayerRole: int16(game.CrewmateRole), PlayerWon: true},
	}
	report := `{"GameOverReason":1,"PlayerInfos":[` +
		`{"Name":"Zed","IsImpostor":false},{"Name":"Sneak","IsImpostor":true},{"Name":"CREW","IsImpostor":false},` +
		`{"Name":"Amy","IsImpostor":false},{"Name":"","IsImpostor":false},{"Name":"Sneak","IsImpostor":true}]}`
	events := []*pgstorage.PostgresGameEvent{
		{EventType: int16(capture.Player), Payload: playerPayload(game.DIED, "Zed", game.Lime)},
		{EventType: int16(capture.Player), Payload: playerPayload(game.EXILED, "sneak", game.Black)},
		{EventType: int16(capture.GameOver), Payload: report},
	}
	roster, complete := buildMatchRoster(linked, events, "crewmate")
	want := []MatchPlayer{
		// colors come from timeline events where there are any, matched by name regardless of case
		{Name: "Sneak", Color: "black", Role: "impostor", Won: false},
		{UserID: "11", Name: "Crew", Color: "blue", Role: "crewmate", Won: true},
		{Name: "Zed", Color: "lime", Role: "crewmate", Won: true},
		// no event named Amy, so her color is unknown and she sorts last
		{Name: "Amy", Role: "crewmate", Won: true},
	}
	if !complete || !reflect.DeepEqual(roster, want) {
		t.Fatalf("roster = %+v complete=%v", roster, complete)
	}

	t.Run("unknown winner", func(t *testing.T) {
		roster, _ := buildMatchRoster(nil, events[2:], "")
		for _, p := range roster {
			if p.Won {
				t.Fatalf("%s won a match with no known winner", p.Name)
			}
		}
	})

	t.Run("malformed report leaves the linked roster", func(t *testing.T) {
		bad := []*pgstorage.PostgresGameEvent{{EventType: int16(capture.GameOver), Payload: `"nonsense"`}}
		roster, complete := buildMatchRoster(linked, bad, "crewmate")
		if complete || len(roster) != 1 {
			t.Fatalf("roster = %+v complete=%v", roster, complete)
		}
	})
}

func TestBuildMatchTimeline(t *testing.T) {
	linked, unlinked := uint64(11), uint64(99)
	ev := func(at int32, kind capture.EventType, payload string, user *uint64) *pgstorage.PostgresGameEvent {
		return &pgstorage.PostgresGameEvent{EventTime: at, EventType: int16(kind), Payload: payload, UserID: user}
	}
	events := []*pgstorage.PostgresGameEvent{
		ev(995, capture.State, "1", nil), // recorded a moment before the start time: clamped to zero
		ev(1010, capture.Player, playerPayload(game.DIED, "Crew", game.Blue), &linked),
		ev(1011, capture.Player, playerPayload(game.DIED, "Crew", game.Blue), &linked), // resent: not a second death
		ev(1012, capture.State, "2", nil),
		ev(1013, capture.State, "2", nil), // resent phase: listed once
		ev(1020, capture.Player, playerPayload(game.FORCEUPDATED, "Other", game.Green), nil),
		ev(1030, capture.Player, playerPayload(game.EXILED, "Other", game.Green), &unlinked),
		ev(1031, capture.State, "1", nil),
		ev(1040, capture.Player, playerPayload(game.DISCONNECTED, "Gone", game.Lime), nil),
		ev(1041, capture.Player, "not json", nil),
		ev(1042, capture.State, `"garbage"`, nil),
		ev(1050, capture.GameOver, `{"GameOverReason":1,"PlayerInfos":[]}`, nil), // roster data, not a timeline entry
	}
	got := buildMatchTimeline(1000, events, map[string]bool{"11": true})
	want := &MatchTimeline{Meetings: 1, Deaths: 1, Exiles: 1, Disconnects: 1, Events: []MatchEvent{
		{Offset: 0, Type: "tasks"},
		{Offset: 10, Type: "death", Name: "Crew", Color: "blue", UserID: "11"},
		{Offset: 12, Type: "discussion"},
		// user 99 is not on the roster (opted out or reset), so the link is withheld
		{Offset: 30, Type: "exile", Name: "Other", Color: "green"},
		{Offset: 31, Type: "tasks"},
		{Offset: 40, Type: "disconnect", Name: "Gone", Color: "lime"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("timeline =\n%+v\nwant\n%+v", got, want)
	}
}

func TestGetMatchSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const guild = "123456789012345678"
	member := verifierFunc(func(_ context.Context, _, target string) (VerifiedGuildAccess, error) {
		return VerifiedGuildAccess{UserID: "user", GuildID: target, Member: true}, nil
	})

	t.Run("invalid IDs", func(t *testing.T) {
		s := &fakeStore{}
		r := NewRouter(Config{GuildVerifier: member}, s)
		for _, path := range []string{
			"/guild/match?guildID=abc&matchID=42",
			"/guild/match?guildID=" + guild,
			"/guild/match?guildID=" + guild + "&matchID=ABCDEFGH:42",
			"/guild/match?guildID=" + guild + "&matchID=0",
			"/guild/match?guildID=" + guild + "&matchID=-3",
			"/guild/match?guildID=" + guild + "&matchID=042",
		} {
			if w := bearerRequest(r, path, "token"); w.Code != 400 {
				t.Fatalf("%s: %d %s", path, w.Code, w.Body)
			}
		}
		if s.matchCalls != 0 {
			t.Fatal("invalid request reached the store")
		}
	})

	t.Run("serves the store's document", func(t *testing.T) {
		s := &fakeStore{match: &MatchSummary{GuildID: guild, MatchID: "42", Status: "finished", Result: "crewmateTasks",
			Roster:   []MatchPlayer{{UserID: "11", Name: "Crew", Color: "blue", Role: "crewmate", Won: true}},
			Timeline: &MatchTimeline{Events: []MatchEvent{{Offset: 0, Type: "tasks"}}},
			Players:  map[string]StatsPlayer{"11": {Username: "alice"}},
		}}
		r := NewRouter(Config{GuildVerifier: member}, s)
		w := bearerRequest(r, "/guild/match?guildID="+guild+"&matchID=42", "token")
		if w.Code != 200 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
		var got MatchSummary
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Result != "crewmateTasks" || got.Roster[0].UserID != "11" || got.Timeline == nil || got.Players["11"].Username != "alice" {
			t.Fatalf("unexpected body: %s", w.Body)
		}
	})

	t.Run("missing match is 404 and not cached", func(t *testing.T) {
		s := &fakeStore{matchErr: errMatchNotFound}
		r := NewRouter(Config{GuildVerifier: member}, s)
		if w := bearerRequest(r, "/guild/match?guildID="+guild+"&matchID=42", "token"); w.Code != 404 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
		s.matchErr = nil
		if w := bearerRequest(r, "/guild/match?guildID="+guild+"&matchID=42", "token"); w.Code != 200 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
	})

	t.Run("store failure is 500 without detail", func(t *testing.T) {
		s := &fakeStore{matchErr: errors.New("postgres: connection refused")}
		r := NewRouter(Config{GuildVerifier: member}, s)
		w := bearerRequest(r, "/guild/match?guildID="+guild+"&matchID=42", "token")
		if w.Code != 500 || strings.Contains(w.Body.String(), "postgres") {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
	})

	t.Run("cached per guild and match", func(t *testing.T) {
		s := &fakeStore{}
		r := NewRouter(Config{GuildVerifier: member}, s)
		for _, path := range []string{
			"/guild/match?guildID=" + guild + "&matchID=42",
			"/guild/match?guildID=" + guild + "&matchID=42",
			"/guild/match?guildID=" + guild + "&matchID=43",
			"/guild/match?guildID=223456789012345678&matchID=42",
		} {
			w := bearerRequest(r, path, "token")
			if w.Code != 200 {
				t.Fatalf("%s: %d %s", path, w.Code, w.Body)
			}
			var got MatchSummary
			_ = json.Unmarshal(w.Body.Bytes(), &got)
			if !strings.Contains(path, "guildID="+got.GuildID+"&matchID="+got.MatchID) {
				t.Fatalf("%s answered with guild %s match %s", path, got.GuildID, got.MatchID)
			}
		}
		if s.matchCalls != 3 {
			t.Fatalf("store built %d summaries, want 3", s.matchCalls)
		}
	})

	t.Run("nonmembers are refused", func(t *testing.T) {
		s := &fakeStore{}
		outsider := verifierFunc(func(_ context.Context, _, target string) (VerifiedGuildAccess, error) {
			return VerifiedGuildAccess{UserID: "user", GuildID: target, Member: false}, nil
		})
		r := NewRouter(Config{GuildVerifier: outsider}, s)
		if w := bearerRequest(r, "/guild/match?guildID="+guild+"&matchID=42", "token"); w.Code != 403 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
		if s.matchCalls != 0 {
			t.Fatal("nonmember reached the store")
		}
	})
}

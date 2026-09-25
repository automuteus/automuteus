package api

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/game"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/gin-gonic/gin"
	"github.com/pashagolub/pgxmock"
)

const testUserNum = uint64(223456789012345678)

func TestBuildUserStats_FreeGuildGetsSummaryAndRecentMatchesOnly(t *testing.T) {
	mock := newStatsMock(t)
	mock.ExpectQuery(`AS crewmate_games, .* MIN\(g.start_time\)::bigint AS first_game, .* WHERE ug.guild_id = \$1 AND ug.user_id = \$2`).
		WithArgs(testGuildNum, testUserNum).
		WillReturnRows(pgxmock.NewRows([]string{"games", "wins", "crewmate_games", "crewmate_wins", "impostor_games", "impostor_wins", "first_game", "last_game"}).
			AddRow(int64(8), int64(5), int64(6), int64(4), int64(2), int64(1), int64p(1000), int64p(9000)))
	mock.ExpectQuery(`ORDER BY g.start_time DESC, g.game_id DESC LIMIT \$3`).
		WithArgs(testGuildNum, testUserNum, 10).
		WillReturnRows(pgxmock.NewRows([]string{"game_id", "start_time", "end_time", "win_type", "play_map", "player_name", "player_color", "player_role", "player_won"}).
			AddRow(int64(42), int32(9000), int32(9600), int16(game.ImpostorBySabotage), int16p(int16(game.AIRSHIP)), "Bo", int16(game.Cyan), int16(game.ImposterRole), true).
			AddRow(int64(41), int32(8000), int32(8500), int16(game.Unknown), (*int16)(nil), "Bo", int16(game.Lime), int16(game.CrewmateRole), false))

	stats, err := buildUserStats(context.Background(), mock, nil, nil, testGuildID, "223456789012345678", premium.PremiumRecord{Tier: premium.FreeTier}, settings.MakeGuildSettings(), time.Unix(10000, 0))
	if err != nil {
		t.Fatal(err)
	}
	want := UserStatsSummary{Games: 8, Wins: 5, Winrate: 62.5,
		Crewmate: RoleRecord{Games: 6, Wins: 4, Winrate: 66.7}, Impostor: RoleRecord{Games: 2, Wins: 1, Winrate: 50},
		FirstGame: 1000, LastGame: 9000}
	if stats.Summary != want {
		t.Fatalf("summary = %+v, want %+v", stats.Summary, want)
	}
	wantMatches := []UserMatch{
		{MatchID: "42", StartTime: 9000, EndTime: 9600, Result: "impostorSabotage", Map: "airship", Name: "Bo", Color: "cyan", Role: "impostor", Won: true},
		{MatchID: "41", StartTime: 8000, EndTime: 8500, Result: "unknown", Name: "Bo", Color: "lime", Role: "crewmate", Won: false},
	}
	if !reflect.DeepEqual(stats.RecentMatches, wantMatches) {
		t.Fatalf("recent = %+v", stats.RecentMatches)
	}
	if stats.Details != nil {
		t.Fatal("free guild received details")
	}
	body, _ := json.Marshal(stats)
	if strings.Contains(string(body), "details") {
		t.Fatalf("free response names details: %s", body)
	}
}

func TestBuildUserStats_NoGamesIsAnEmptyDocument(t *testing.T) {
	mock := newStatsMock(t)
	mock.ExpectQuery(`AS crewmate_games`).WithArgs(testGuildNum, testUserNum).
		WillReturnRows(pgxmock.NewRows([]string{"games", "wins", "crewmate_games", "crewmate_wins", "impostor_games", "impostor_wins", "first_game", "last_game"}).
			AddRow(int64(0), int64(0), int64(0), int64(0), int64(0), int64(0), (*int64)(nil), (*int64)(nil)))
	mock.ExpectQuery(`LIMIT \$3`).WithArgs(testGuildNum, testUserNum, 10).
		WillReturnRows(pgxmock.NewRows([]string{"game_id", "start_time", "end_time", "win_type", "play_map", "player_name", "player_color", "player_role", "player_won"}))
	stats, err := buildUserStats(context.Background(), mock, nil, nil, testGuildID, "223456789012345678", premium.PremiumRecord{Tier: premium.GoldTier, Days: 0}, settings.MakeGuildSettings(), time.Unix(10000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if stats.Summary != (UserStatsSummary{}) || stats.Details != nil {
		t.Fatalf("stats = %+v", stats)
	}
	body, _ := json.Marshal(stats)
	if !strings.Contains(string(body), `"recentMatches":[]`) || strings.Contains(string(body), "firstGame") {
		t.Fatalf("body = %s", body)
	}
}

func TestStreaks(t *testing.T) {
	for _, c := range []struct {
		results []bool
		want    Streaks
	}{
		{nil, Streaks{}},
		{[]bool{true, true, false, true, true, true}, Streaks{Current: 3, BestWin: 3, BestLoss: 1}},
		{[]bool{true, true, true, false, false}, Streaks{Current: -2, BestWin: 3, BestLoss: 2}},
		{[]bool{false, false, false, true}, Streaks{Current: 1, BestWin: 1, BestLoss: 3}},
	} {
		if got := streaks(c.results); got != c.want {
			t.Errorf("streaks(%v) = %+v, want %+v", c.results, got, c.want)
		}
	}
}

func TestUserDetailsUserIDs(t *testing.T) {
	d := &UserStatsDetails{
		PlayedWith:            []PlayedWith{{UserID: "3"}, {UserID: "2"}},
		BestImpostorTeammates: []Teammate{{UserID: "2"}},
		KilledBy:              []DiedWith{{ImpostorID: "4"}, {ImpostorID: "1"}},
	}
	if got := d.userIDs("1"); !reflect.DeepEqual(got, []string{"1", "2", "3", "4"}) {
		t.Fatalf("ids = %v", got)
	}
}

func TestGetUserStats(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const guild, user = "123456789012345678", "223456789012345678"
	member := verifierFunc(func(_ context.Context, _, target string) (VerifiedGuildAccess, error) {
		return VerifiedGuildAccess{UserID: "user", GuildID: target, Member: true}, nil
	})

	t.Run("invalid IDs", func(t *testing.T) {
		s := &fakeStore{}
		r := NewRouter(Config{GuildVerifier: member}, s)
		for _, path := range []string{
			"/guild/user?guildID=abc&userID=" + user,
			"/guild/user?guildID=" + guild,
			"/guild/user?guildID=" + guild + "&userID=42",
			"/guild/user?guildID=" + guild + "&userID=" + user + "x",
		} {
			if w := bearerRequest(r, path, "token"); w.Code != 400 {
				t.Fatalf("%s: %d %s", path, w.Code, w.Body)
			}
		}
		if s.userCalls != 0 {
			t.Fatal("invalid request reached the store")
		}
	})

	t.Run("serves the store's document, cached per guild and player", func(t *testing.T) {
		s := &fakeStore{}
		r := NewRouter(Config{GuildVerifier: member}, s)
		for _, path := range []string{
			"/guild/user?guildID=" + guild + "&userID=" + user,
			"/guild/user?guildID=" + guild + "&userID=" + user,
			"/guild/user?guildID=" + guild + "&userID=323456789012345678",
			"/guild/user?guildID=223456789012345678&userID=" + user,
		} {
			w := bearerRequest(r, path, "token")
			if w.Code != 200 {
				t.Fatalf("%s: %d %s", path, w.Code, w.Body)
			}
			var got UserStats
			_ = json.Unmarshal(w.Body.Bytes(), &got)
			if !strings.Contains(path, "guildID="+got.GuildID+"&userID="+got.UserID) {
				t.Fatalf("%s answered with guild %s user %s", path, got.GuildID, got.UserID)
			}
		}
		if s.userCalls != 3 {
			t.Fatalf("store built %d documents, want 3", s.userCalls)
		}
	})

	t.Run("store failure is 500 without detail", func(t *testing.T) {
		s := &fakeStore{userErr: errors.New("postgres: connection refused")}
		r := NewRouter(Config{GuildVerifier: member}, s)
		w := bearerRequest(r, "/guild/user?guildID="+guild+"&userID="+user, "token")
		if w.Code != 500 || strings.Contains(w.Body.String(), "postgres") {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
	})

	t.Run("nonmembers are refused", func(t *testing.T) {
		s := &fakeStore{}
		outsider := verifierFunc(func(_ context.Context, _, target string) (VerifiedGuildAccess, error) {
			return VerifiedGuildAccess{UserID: "user", GuildID: target, Member: false}, nil
		})
		r := NewRouter(Config{GuildVerifier: outsider}, s)
		if w := bearerRequest(r, "/guild/user?guildID="+guild+"&userID="+user, "token"); w.Code != 403 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
		if s.userCalls != 0 {
			t.Fatal("nonmember reached the store")
		}
	})
}

func int64p(v int64) *int64 { return &v }

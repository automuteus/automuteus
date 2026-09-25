package api

import (
	"net/http"
	"net/http/httptest"

	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/gin-gonic/gin"
	"github.com/pashagolub/pgxmock"
)

const testGuildNum = uint64(123456789012345678)

func newStatsMock(t *testing.T) pgxmock.PgxPoolIface {
	t.Helper()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
		mock.Close()
	})
	return mock
}

func summaryRows(games, crew, imp int64) *pgxmock.Rows {
	return pgxmock.NewRows([]string{"games", "crewmate_wins", "impostor_wins"}).AddRow(games, crew, imp)
}

func TestBuildGuildStats_FreeGuildGetsSummaryOnly(t *testing.T) {
	mock := newStatsMock(t)
	mock.ExpectQuery(`SELECT COUNT\(\*\) AS games, .* FROM games WHERE guild_id = \$1 AND end_time <> -1 AND win_type <> \$2`).
		WithArgs(testGuildNum, int16(-2)).
		WillReturnRows(summaryRows(8, 5, 2))

	stats, err := buildGuildStats(context.Background(), mock, nil, nil, testGuildID, premium.PremiumRecord{Tier: premium.FreeTier}, settings.MakeGuildSettings(), false)
	if err != nil {
		t.Fatal(err)
	}
	want := GuildStatsSummary{GamesPlayed: 8, CrewmateWins: 5, ImpostorWins: 2, CrewmateWinrate: 62.5, ImpostorWinrate: 25}
	if stats.Summary != want {
		t.Fatalf("summary = %+v, want %+v", stats.Summary, want)
	}
	if stats.Leaderboards != nil {
		t.Fatal("free guild received leaderboards")
	}
	body, _ := json.Marshal(stats)
	if strings.Contains(string(body), "leaderboards") {
		t.Fatalf("free response names leaderboards: %s", body)
	}
	if !strings.Contains(string(body), `"players":{}`) {
		t.Fatalf("players should be an empty object: %s", body)
	}
}

func TestBuildGuildStats_ExpiredPremiumIsFree(t *testing.T) {
	mock := newStatsMock(t)
	mock.ExpectQuery(`FROM games WHERE guild_id`).WithArgs(testGuildNum, int16(-2)).WillReturnRows(summaryRows(0, 0, 0))
	stats, err := buildGuildStats(context.Background(), mock, nil, nil, testGuildID, premium.PremiumRecord{Tier: premium.GoldTier, Days: 0}, settings.MakeGuildSettings(), false)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Leaderboards != nil {
		t.Fatal("expired premium received leaderboards")
	}
	if stats.Summary.CrewmateWinrate != 0 || stats.Summary.ImpostorWinrate != 0 {
		t.Fatal("winrates of a guild with no games must be zero, not NaN")
	}
}

func TestBuildGuildStats_PremiumRunsEveryBoardWithGuildSettings(t *testing.T) {
	mock := newStatsMock(t)
	// The boards run concurrently, so their order is not fixed.
	mock.MatchExpectationsInOrder(false)
	sett := settings.MakeGuildSettings()
	// The size setting is ignored: every board holds five entries regardless.
	sett.SetLeaderboardSize(9)
	sett.SetLeaderboardMin(4)

	mock.ExpectQuery(`FROM games WHERE guild_id`).WithArgs(testGuildNum, int16(-2)).WillReturnRows(summaryRows(20, 12, 8))
	mock.ExpectQuery(`SELECT user_id, COUNT\(\*\) AS total FROM users_games WHERE guild_id = \$1 GROUP BY user_id ORDER BY total DESC, user_id LIMIT \$2`).
		WithArgs(testGuildNum, 5).
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "total"}).AddRow(uint64(11), int64(9)).AddRow(uint64(22), int64(4)))
	winQuery := `FROM users_games WHERE guild_id = \$1 AND \(\$2 < 0 OR player_role = \$2\) GROUP BY user_id HAVING COUNT\(\*\) >= \$3 ORDER BY win_rate DESC, win DESC, total DESC, user_id LIMIT \$4`
	winRows := func() *pgxmock.Rows {
		return pgxmock.NewRows([]string{"user_id", "win", "total", "win_rate"})
	}
	mock.ExpectQuery(winQuery).WithArgs(testGuildNum, int16(-1), 4, 5).
		WillReturnRows(winRows().AddRow(uint64(11), int64(6), int64(9), 66.6667))
	mock.ExpectQuery(winQuery).WithArgs(testGuildNum, int16(0), 4, 5).WillReturnRows(winRows())
	mock.ExpectQuery(winQuery).WithArgs(testGuildNum, int16(1), 4, 5).WillReturnRows(winRows())
	duoRows := func() *pgxmock.Rows {
		return pgxmock.NewRows([]string{"user_id", "teammate_id", "total", "win", "win_rate"})
	}
	bestDuo := `b.user_id > a.user_id AND b.player_role = \$2 WHERE a.guild_id = \$1 AND a.player_role = \$2 GROUP BY a.user_id, b.user_id HAVING COUNT\(\*\) >= \$3 ORDER BY win_rate DESC, win DESC, total DESC, a.user_id, b.user_id LIMIT \$4`
	worstDuo := `HAVING COUNT\(\*\) >= \$3 ORDER BY win_rate ASC, win ASC, total DESC, a.user_id, b.user_id LIMIT \$4`
	// Impostor duos use the fixed floor of two shared games; crewmate duos use the guild minimum.
	mock.ExpectQuery(bestDuo).WithArgs(testGuildNum, int16(1), 2, 5).
		WillReturnRows(duoRows().AddRow(uint64(11), uint64(33), int64(3), int64(2), 66.6667))
	mock.ExpectQuery(worstDuo).WithArgs(testGuildNum, int16(1), 2, 5).WillReturnRows(duoRows())
	mock.ExpectQuery(bestDuo).WithArgs(testGuildNum, int16(0), 4, 5).WillReturnRows(duoRows())
	mock.ExpectQuery(worstDuo).WithArgs(testGuildNum, int16(0), 4, 5).WillReturnRows(duoRows())
	mock.ExpectQuery(`WITH first_death AS \(SELECT DISTINCT ON \(e.game_id\) .* crew AS .* totals AS .* SELECT c.user_id, COUNT\(\*\) AS total_death, t.total, .* HAVING t.total >= \$2 ORDER BY death_rate DESC, total_death DESC, c.user_id LIMIT \$3`).
		WithArgs(testGuildNum, 4, 5).
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "total_death", "total", "death_rate"}).AddRow(uint64(44), int64(3), int64(6), 50.0))
	mock.ExpectQuery(`WITH crew AS .* imp AS .* died AS .* SELECT c.user_id, i.user_id AS teammate_id, .* FROM crew c INNER JOIN imp i .* LEFT JOIN died d .* HAVING COUNT\(\*\) >= \$2 ORDER BY death_rate DESC, total_death DESC, encounter DESC, c.user_id, i.user_id LIMIT \$3`).
		WithArgs(testGuildNum, 4, 5).
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "teammate_id", "total_death", "encounter", "death_rate"}).AddRow(uint64(44), uint64(11), int64(4), int64(5), 80.0))

	stats, err := buildGuildStats(context.Background(), mock, nil, nil, testGuildID, premium.PremiumRecord{Tier: premium.GoldTier, Days: 10}, sett, false)
	if err != nil {
		t.Fatal(err)
	}
	b := stats.Leaderboards
	if b == nil {
		t.Fatal("premium guild received no leaderboards")
	}
	if b.MinGames != 4 {
		t.Fatalf("board minimum = %d, want 4", b.MinGames)
	}
	if len(b.MostGames) != 2 || b.MostGames[0] != (PlayerGames{UserID: "11", Games: 9}) {
		t.Fatalf("most games = %+v", b.MostGames)
	}
	if len(b.Winrate) != 1 || b.Winrate[0] != (PlayerWinrate{UserID: "11", Wins: 6, Games: 9, Winrate: 66.7}) {
		t.Fatalf("winrate = %+v", b.Winrate)
	}
	if len(b.BestImpostorDuo) != 1 || b.BestImpostorDuo[0] != (DuoWinrate{UserID: "11", TeammateID: "33", Wins: 2, Games: 3, Winrate: 66.7}) {
		t.Fatalf("best impostor duo = %+v", b.BestImpostorDuo)
	}
	if len(b.FirstTarget) != 1 || b.FirstTarget[0] != (FirstTarget{UserID: "44", FirstDeaths: 3, CrewmateGames: 6, Rate: 50}) {
		t.Fatalf("first target = %+v", b.FirstTarget)
	}
	if len(b.KilledBy) != 1 || b.KilledBy[0] != (KilledBy{UserID: "44", ImpostorID: "11", Deaths: 4, Games: 5, Rate: 80}) {
		t.Fatalf("killed by = %+v", b.KilledBy)
	}
	if got := b.userIDs(); strings.Join(got, ",") != "11,22,33,44" {
		t.Fatalf("named users = %v", got)
	}
	// Empty boards must serialise as [] so the page can render them without null checks.
	body, err := json.Marshal(stats)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"crewmateWinrate":[]`, `"worstImpostorDuo":[]`, `"bestCrewmateDuo":[]`, `"worstCrewmateDuo":[]`, `"impostorWinrate":[]`} {
		if !strings.Contains(string(body), key) {
			t.Errorf("missing %s in %s", key, body)
		}
	}
}

func TestBuildGuildStats_BoardFailureFailsTheRollup(t *testing.T) {
	// No ExpectationsWereMet check here: the failure cancels the other boards, so which of them still run is
	// not fixed, and the ones that do run are given an empty answer.
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	mock.MatchExpectationsInOrder(false)
	boom := errors.New("boom")
	mock.ExpectQuery(`FROM games WHERE guild_id`).WithArgs(testGuildNum, int16(-2)).WillReturnRows(summaryRows(1, 1, 0))
	mock.ExpectQuery(`SELECT user_id, COUNT\(\*\) AS total FROM users_games`).WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnError(boom)
	for i := 0; i < 9; i++ {
		mock.ExpectQuery(`FROM users_games|FROM games g`).WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
			WillReturnRows(pgxmock.NewRows([]string{"user_id"}))
	}
	_, err = buildGuildStats(context.Background(), mock, nil, nil, testGuildID, premium.PremiumRecord{Tier: premium.SelfHostTier, Days: premium.NoExpiryCode}, settings.MakeGuildSettings(), false)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
}

func TestBuildGuildStats_RejectsMalformedGuild(t *testing.T) {
	mock := newStatsMock(t)
	if _, err := buildGuildStats(context.Background(), mock, nil, nil, "not-a-snowflake", premium.PremiumRecord{}, settings.MakeGuildSettings(), false); err == nil {
		t.Fatal("malformed guild ID accepted")
	}
}

func TestGetGuildStats(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const guild = "123456789012345678"
	member := verifierFunc(func(_ context.Context, _, target string) (VerifiedGuildAccess, error) {
		return VerifiedGuildAccess{UserID: "user", GuildID: target, Member: true}, nil
	})

	t.Run("invalid guild", func(t *testing.T) {
		s := &fakeStore{}
		r := NewRouter(Config{GuildVerifier: member}, s)
		if w := bearerRequest(r, "/guild/stats?guildID=abc", "token"); w.Code != 400 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
		if s.statsCalls != 0 {
			t.Fatal("invalid guild reached the store")
		}
	})

	t.Run("serves the store's document", func(t *testing.T) {
		s := &fakeStore{stats: &GuildStats{
			GuildID: guild,
			Premium: premium.PremiumRecord{Tier: premium.GoldTier, Days: 3},
			Summary: GuildStatsSummary{GamesPlayed: 4, CrewmateWins: 3, ImpostorWins: 1, CrewmateWinrate: 75, ImpostorWinrate: 25},
			Leaderboards: &GuildLeaderboards{MinGames: 3,
				MostGames: []PlayerGames{{UserID: "11", Games: 4}},
			},
			Players: map[string]StatsPlayer{"11": {Username: "alice", Nickname: "Al"}},
		}}
		r := NewRouter(Config{GuildVerifier: member}, s)
		w := bearerRequest(r, "/guild/stats?guildID="+guild, "token")
		if w.Code != 200 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
		var got GuildStats
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Summary.GamesPlayed != 4 || got.Leaderboards == nil || got.Leaderboards.MostGames[0].UserID != "11" || got.Players["11"].Nickname != "Al" {
			t.Fatalf("unexpected body: %s", w.Body)
		}
	})

	t.Run("store failure is 500 without detail", func(t *testing.T) {
		s := &fakeStore{statsErr: errors.New("postgres: connection refused")}
		r := NewRouter(Config{GuildVerifier: member}, s)
		w := bearerRequest(r, "/guild/stats?guildID="+guild, "token")
		if w.Code != 500 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
		if strings.Contains(w.Body.String(), "postgres") {
			t.Fatalf("error detail leaked: %s", w.Body)
		}
	})

	t.Run("rollup is cached per guild", func(t *testing.T) {
		s := &fakeStore{}
		r := NewRouter(Config{GuildVerifier: member}, s)
		for i := 0; i < 3; i++ {
			if w := bearerRequest(r, "/guild/stats?guildID="+guild, "token"); w.Code != 200 {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
		}
		if w := bearerRequest(r, "/guild/stats?guildID=223456789012345678", "token"); w.Code != 200 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
		if s.statsCalls != 2 {
			t.Fatalf("store built the rollup %d times, want 2 (one per guild)", s.statsCalls)
		}
	})

	t.Run("caching can be disabled", func(t *testing.T) {
		s := &fakeStore{}
		r := NewRouter(Config{GuildVerifier: member, StatsCacheTTL: -1}, s)
		for i := 0; i < 2; i++ {
			if w := bearerRequest(r, "/guild/stats?guildID="+guild, "token"); w.Code != 200 {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
		}
		if s.statsCalls != 2 {
			t.Fatalf("store built the rollup %d times, want 2", s.statsCalls)
		}
	})

	t.Run("errors are not cached", func(t *testing.T) {
		s := &fakeStore{statsErr: errors.New("boom")}
		r := NewRouter(Config{GuildVerifier: member}, s)
		if w := bearerRequest(r, "/guild/stats?guildID="+guild, "token"); w.Code != 500 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
		s.statsErr = nil
		if w := bearerRequest(r, "/guild/stats?guildID="+guild, "token"); w.Code != 200 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
	})
}

// TestGetGuildStatsFull checks that full=1 reaches the operators' build only under Basic auth, so a member can
// never get the leaderboards by asking for them.
func TestGetGuildStatsFull(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const guild = "123456789012345678"
	member := verifierFunc(func(_ context.Context, _, target string) (VerifiedGuildAccess, error) {
		return VerifiedGuildAccess{UserID: "user", GuildID: target, Member: true}, nil
	})
	for _, tc := range []struct {
		name        string
		path        string
		basic       bool
		wantAdmin   int
		wantRegular int
	}{
		{"basic auth with full", "/guild/stats?guildID=" + guild + "&full=1", true, 1, 0},
		{"basic auth without full", "/guild/stats?guildID=" + guild, true, 0, 1},
		{"member asking for full", "/guild/stats?guildID=" + guild + "&full=1", false, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &fakeStore{stats: &GuildStats{GuildID: guild}}
			r := NewRouter(Config{GuildVerifier: member, AdminPassword: "test-password"}, s)
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.basic {
				req.SetBasicAuth("admin", "test-password")
			} else {
				req.Header.Set("Authorization", "Bearer token")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != 200 {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
			if s.adminStatsCalls != tc.wantAdmin || s.statsCalls-s.adminStatsCalls != tc.wantRegular {
				t.Fatalf("admin builds %d, regular builds %d", s.adminStatsCalls, s.statsCalls-s.adminStatsCalls)
			}
		})
	}
}

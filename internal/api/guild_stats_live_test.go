package api

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/game"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	pgstorage "github.com/automuteus/automuteus/v8/pkg/storage"
	"github.com/automuteus/automuteus/v8/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestLiveGuildStats runs the statistics queries against a real Postgres, since the mocked tests can only check
// parameters and scanning, not that the SQL is valid or that the joins count what they claim to.
func TestLiveGuildStats(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("set disposable TEST_POSTGRES_URL")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	// Registered before the fixture cleanup so it runs after it: cleanups run last-in first-out, and a deferred
	// Close would run before any cleanup, leaving the fixtures behind.
	t.Cleanup(pool.Close)
	if err := storage.ApplySchemas(ctx, pool, false); err != nil {
		t.Fatal(err)
	}

	const guild = uint64(900000000000000001)
	const alice, bob, carol, dave = uint64(1001), uint64(1002), uint64(1003), uint64(1004)
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM guilds WHERE guild_id = $1", guild)
		pool.Exec(ctx, "DELETE FROM users WHERE user_id = ANY($1)", []uint64{alice, bob, carol, dave})
	})
	pool.Exec(ctx, "DELETE FROM guilds WHERE guild_id = $1", guild)
	exec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	exec("INSERT INTO guilds (guild_id, guild_name, premium) VALUES ($1, 'stats test', 0)", guild)
	for _, u := range []uint64{alice, bob, carol, dave} {
		exec("INSERT INTO users (user_id, opt) VALUES ($1, true) ON CONFLICT DO NOTHING", u)
	}

	// Each game lists its result and, per player, their role and whether they died first (nil for nobody).
	type player struct {
		id    uint64
		role  game.GameRole
		died  bool
		first bool
	}
	games := []struct {
		result  game.GameResult
		players []player
	}{
		// 1: crewmates win by task; bob (impostor) loses; alice died first.
		{game.HumansByTask, []player{{alice, game.CrewmateRole, true, true}, {carol, game.CrewmateRole, false, false}, {bob, game.ImposterRole, false, false}}},
		// 2: impostors (bob & dave) win by kill; alice died first, carol died too.
		{game.ImpostorByKill, []player{{alice, game.CrewmateRole, true, true}, {carol, game.CrewmateRole, true, false}, {bob, game.ImposterRole, false, false}, {dave, game.ImposterRole, false, false}}},
		// 3: crewmates win by vote; bob & dave impostors lose; carol died first.
		{game.HumansByVote, []player{{alice, game.CrewmateRole, false, false}, {carol, game.CrewmateRole, true, true}, {bob, game.ImposterRole, false, false}, {dave, game.ImposterRole, false, false}}},
		// 4: aborted game must be ignored by every board, even though alice's death in it was recorded before
		// the abort (the bot keeps the events and records no players).
		{game.Aborted, []player{{alice, game.CrewmateRole, true, true}, {bob, game.ImposterRole, false, false}}},
		// 5: carol (impostor) wins by kill; an unlinked player died first (a null user, added below), then dave.
		// Nobody linked is the first target of this game.
		{game.ImpostorByKill, []player{{dave, game.CrewmateRole, true, false}, {carol, game.ImposterRole, false, false}}},
		// 6: crewmates win by task. alice died first, but her player record for this game is gone (added
		// below, as after a stats reset), so the game has no first target and her retained games are unaffected.
		{game.HumansByTask, []player{{carol, game.CrewmateRole, false, false}, {dave, game.ImposterRole, false, false}}},
	}
	start := int32(1_700_000_000)
	for i, g := range games {
		var gameID int64
		endTime := start + int32(i*1000) + 600
		if err := pool.QueryRow(ctx, "INSERT INTO games (guild_id, connect_code, start_time, win_type, end_time) VALUES ($1, 'ABCDEFGH', $2, $3, $4) RETURNING game_id",
			guild, start+int32(i*1000), int16(g.result), endTime).Scan(&gameID); err != nil {
			t.Fatal(err)
		}
		if g.result == game.Aborted {
			// The bot records no players for an aborted game, but events up to the abort stay.
			payload, _ := json.Marshal(game.Player{Action: game.DIED, Name: "name", IsDead: true})
			exec("INSERT INTO game_events (user_id, game_id, event_time, event_type, payload) VALUES ($1, $2, $3, 3, $4)", alice, gameID, start+int32(i*1000)+60, string(payload))
			continue
		}
		if i == 4 {
			payload, _ := json.Marshal(game.Player{Action: game.DIED, Name: "unlinked", IsDead: true})
			exec("INSERT INTO game_events (user_id, game_id, event_time, event_type, payload) VALUES (NULL, $1, $2, 3, $3)", gameID, start+int32(i*1000)+30, string(payload))
		}
		if i == 5 {
			payload, _ := json.Marshal(game.Player{Action: game.DIED, Name: "name", IsDead: true})
			exec("INSERT INTO game_events (user_id, game_id, event_time, event_type, payload) VALUES ($1, $2, $3, 3, $4)", alice, gameID, start+int32(i*1000)+30, string(payload))
		}
		impostorWin := g.result == game.ImpostorByKill || g.result == game.ImpostorByVote || g.result == game.ImpostorBySabotage || g.result == game.ImpostorDisconnect
		for _, p := range g.players {
			won := (p.role == game.ImposterRole) == impostorWin
			exec("INSERT INTO users_games VALUES ($1, $2, $3, 'name', 0, $4, $5)", p.id, guild, gameID, int16(p.role), won)
			if p.died {
				offset := int32(120)
				if p.first {
					offset = 60
				}
				payload, _ := json.Marshal(game.Player{Action: game.DIED, Name: "name", IsDead: true})
				exec("INSERT INTO game_events (user_id, game_id, event_time, event_type, payload) VALUES ($1, $2, $3, 3, $4)", p.id, gameID, start+int32(i*1000)+offset, string(payload))
			}
		}
	}

	sett := settings.MakeGuildSettings()
	sett.SetLeaderboardMin(1)
	stats, err := buildGuildStats(ctx, pool, nil, nil, "900000000000000001", premium.PremiumRecord{Tier: premium.SelfHostTier, Days: premium.NoExpiryCode}, sett, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := (GuildStatsSummary{GamesPlayed: 5, CrewmateWins: 3, ImpostorWins: 2, CrewmateWinrate: 60, ImpostorWinrate: 40}); stats.Summary != want {
		t.Errorf("summary = %+v, want %+v", stats.Summary, want)
	}
	b := stats.Leaderboards
	if b == nil {
		t.Fatal("no leaderboards")
	}
	find := func(rows []PlayerWinrate, id string) *PlayerWinrate {
		for i := range rows {
			if rows[i].UserID == id {
				return &rows[i]
			}
		}
		return nil
	}
	if len(b.MostGames) != 4 || b.MostGames[0] != (PlayerGames{UserID: "1003", Games: 5}) {
		t.Errorf("most games = %+v", b.MostGames)
	}
	if r := find(b.Winrate, "1001"); r == nil || r.Wins != 2 || r.Games != 3 || r.Winrate != 66.7 {
		t.Errorf("alice overall = %+v", r)
	}
	if r := find(b.ImpostorWinrate, "1002"); r == nil || r.Wins != 1 || r.Games != 3 || r.Winrate != 33.3 {
		t.Errorf("bob as impostor = %+v", r)
	}
	if r := find(b.CrewmateWinrate, "1002"); r != nil {
		t.Errorf("bob never played crewmate, got %+v", r)
	}
	// bob & dave shared two impostor games and won one; the pair must appear once, lower ID first.
	if len(b.BestImpostorDuo) != 1 || b.BestImpostorDuo[0] != (DuoWinrate{UserID: "1002", TeammateID: "1004", Wins: 1, Games: 2, Winrate: 50}) {
		t.Errorf("best impostor duo = %+v", b.BestImpostorDuo)
	}
	if len(b.WorstImpostorDuo) != 1 || b.WorstImpostorDuo[0] != b.BestImpostorDuo[0] {
		t.Errorf("worst impostor duo = %+v", b.WorstImpostorDuo)
	}
	// alice died first in two of her three crewmate games, carol in one of four. alice's deaths in the aborted
	// game and in game 6 (no player record) do not count, and dave is not the first target of game 5 because an
	// unlinked player died before him.
	if len(b.FirstTarget) != 2 || b.FirstTarget[0] != (FirstTarget{UserID: "1001", FirstDeaths: 2, CrewmateGames: 3, Rate: 66.7}) || b.FirstTarget[1] != (FirstTarget{UserID: "1003", FirstDeaths: 1, CrewmateGames: 4, Rate: 25}) {
		t.Errorf("first target = %+v", b.FirstTarget)
	}
	// A guild with no games at all must still produce a valid summary from the real query.
	empty, err := buildGuildStats(ctx, pool, nil, nil, "900000000000000002", premium.PremiumRecord{Tier: premium.FreeTier}, sett, false)
	if err != nil {
		t.Fatal(err)
	}
	if empty.Summary != (GuildStatsSummary{}) || empty.Leaderboards != nil {
		t.Errorf("empty guild = %+v", empty)
	}
}

// TestLiveGuildDuoRanking checks the duo boards rank by the Wilson bounds rather than the raw winrate: a pair
// that won 7 of 8 is a better bet than one that won 2 of 2, and one that won 1 of 8 a worse one than 0 of 2.
func TestLiveGuildDuoRanking(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("set disposable TEST_POSTGRES_URL")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := storage.ApplySchemas(ctx, pool, false); err != nil {
		t.Fatal(err)
	}

	const guild = uint64(900000000000000041)
	users := []uint64{4101, 4102, 4103, 4104, 4105, 4106, 4107, 4108}
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM guilds WHERE guild_id = $1", guild)
		pool.Exec(ctx, "DELETE FROM users WHERE user_id = ANY($1)", users)
	})
	pool.Exec(ctx, "DELETE FROM guilds WHERE guild_id = $1", guild)
	exec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	exec("INSERT INTO guilds (guild_id, guild_name, premium) VALUES ($1, 'duo test', 0)", guild)
	for _, u := range users {
		exec("INSERT INTO users (user_id, opt) VALUES ($1, true) ON CONFLICT DO NOTHING", u)
	}
	pairs := []struct {
		a, b        uint64
		wins, games int
	}{
		{4101, 4102, 7, 8},
		{4103, 4104, 2, 2},
		{4105, 4106, 1, 8},
		{4107, 4108, 0, 2},
	}
	start := int32(1_700_000_000)
	for _, p := range pairs {
		for i := 0; i < p.games; i++ {
			won := i < p.wins
			result := game.HumansByTask
			if won {
				result = game.ImpostorByKill
			}
			var gameID int64
			if err := pool.QueryRow(ctx, "INSERT INTO games (guild_id, connect_code, start_time, win_type, end_time) VALUES ($1, 'ABCDEFGH', $2, $3, $4) RETURNING game_id",
				guild, start, int16(result), start+600).Scan(&gameID); err != nil {
				t.Fatal(err)
			}
			start += 1000
			for _, u := range []uint64{p.a, p.b} {
				exec("INSERT INTO users_games VALUES ($1, $2, $3, 'name', 0, $4, $5)", u, guild, gameID, int16(game.ImposterRole), won)
			}
		}
	}

	best, worst, err := pgstorage.GuildDuoRanking(ctx, pool, guild, game.ImposterRole, impostorDuoMinGames, leaderboardSize)
	if err != nil {
		t.Fatal(err)
	}
	firsts := func(rows []*pgstorage.PostgresBestTeammatePlayerRanking) []uint64 {
		ids := make([]uint64, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.UserID)
		}
		return ids
	}
	if got, want := firsts(best), []uint64{4101, 4103, 4105, 4107}; !reflect.DeepEqual(got, want) {
		t.Errorf("best = %v, want %v", got, want)
	}
	if got, want := firsts(worst), []uint64{4105, 4107, 4101, 4103}; !reflect.DeepEqual(got, want) {
		t.Errorf("worst = %v, want %v", got, want)
	}
	if r := best[0]; r.TeammateID != 4102 || r.WinCount != 7 || r.Count != 8 || r.WinRate != 87.5 {
		t.Errorf("best pair = %+v", r)
	}
}

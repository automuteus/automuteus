package api

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/capture"
	"github.com/automuteus/automuteus/v8/pkg/game"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/automuteus/automuteus/v8/storage"
	"github.com/jackc/pgx/v4/pgxpool"
)

// TestLiveUserStats runs the player statistics queries against a real Postgres. The fixture is small enough that
// every expected number below is worked out by hand in the comments.
func TestLiveUserStats(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("set disposable TEST_POSTGRES_URL")
	}
	ctx := context.Background()
	pool, err := pgxpool.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := storage.ApplySchemas(ctx, pool, false); err != nil {
		t.Fatal(err)
	}

	const guild, other = uint64(900000000000000003), uint64(900000000000000004)
	const alice, bob, carol, dave = uint64(2001), uint64(2002), uint64(2003), uint64(2004)
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM guilds WHERE guild_id = ANY($1)", []uint64{guild, other})
		pool.Exec(ctx, "DELETE FROM users WHERE user_id = ANY($1)", []uint64{alice, bob, carol, dave})
	})
	pool.Exec(ctx, "DELETE FROM guilds WHERE guild_id = ANY($1)", []uint64{guild, other})
	exec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	for _, g := range []uint64{guild, other} {
		exec("INSERT INTO guilds (guild_id, guild_name, premium) VALUES ($1, 'user stats test', 0)", g)
	}
	for _, u := range []uint64{alice, bob, carol, dave} {
		exec("INSERT INTO users (user_id, opt) VALUES ($1, true) ON CONFLICT DO NOTHING", u)
	}

	type player struct {
		id    uint64
		role  game.GameRole
		name  string
		color int
	}
	type event struct {
		id     uint64
		offset int32
		action game.PlayerAction
	}
	const day = int32(86400)
	start := int32(1_700_000_000)
	skeld, polus := int16(game.SKELD), int16(game.POLUS)
	games := []struct {
		guild   uint64
		day     int32
		result  game.GameResult
		playMap *int16
		players []player
		events  []event
	}{
		// 1: crewmates win; alice (crew) died first.
		{guild, 5, game.HumansByTask, &skeld, []player{{alice, game.CrewmateRole, "alice", game.Red}, {carol, game.CrewmateRole, "carol", 1}, {bob, game.ImposterRole, "bob", 2}},
			[]event{{alice, 60, game.DIED}}},
		// 2: impostors win; carol died first, then alice, whose death the capture sent twice.
		{guild, 10, game.ImpostorByKill, &skeld, []player{{alice, game.CrewmateRole, "alice", game.Red}, {carol, game.CrewmateRole, "carol", 1}, {bob, game.ImposterRole, "bob", 2}, {dave, game.ImposterRole, "dave", 3}},
			[]event{{carol, 60, game.DIED}, {alice, 120, game.DIED}, {alice, 125, game.DIED}}},
		// 3: crewmates win by voting out alice (impostor, playing as "ali").
		{guild, 15, game.HumansByVote, &polus, []player{{alice, game.ImposterRole, "ali", game.Blue}, {bob, game.ImposterRole, "bob", 2}, {carol, game.CrewmateRole, "carol", 1}, {dave, game.CrewmateRole, "dave", 3}},
			[]event{{alice, 200, game.EXILED}}},
		// 4: alice and bob win as impostors; no map recorded.
		{guild, 20, game.ImpostorByVote, nil, []player{{alice, game.ImposterRole, "alice", game.Red}, {bob, game.ImposterRole, "bob", 2}, {carol, game.CrewmateRole, "carol", 1}, {dave, game.CrewmateRole, "dave", 3}}, nil},
		// 5: no result reported, so nobody won; counts as a game but not toward streaks.
		{guild, 25, game.Unknown, nil, []player{{alice, game.CrewmateRole, "alice", game.Red}, {carol, game.CrewmateRole, "carol", 1}, {dave, game.ImposterRole, "dave", 3}}, nil},
		// 6: aborted, so no players are recorded; alice's death in it counts nowhere.
		{guild, 30, game.Aborted, nil, nil, []event{{alice, 60, game.DIED}}},
		// 7: crewmates win even though alice (crew) was voted out.
		{guild, 35, game.HumansByTask, &polus, []player{{alice, game.CrewmateRole, "alice", game.Blue}, {dave, game.CrewmateRole, "dave", 3}, {bob, game.ImposterRole, "bob", 2}},
			[]event{{alice, 90, game.EXILED}}},
		// 8: another guild; must not count anywhere.
		{other, 36, game.HumansByTask, &skeld, []player{{alice, game.CrewmateRole, "alice", game.Red}, {bob, game.ImposterRole, "bob", 2}},
			[]event{{alice, 30, game.DIED}}},
	}
	ids := map[int]int64{}
	for i, g := range games {
		begin := start + g.day*day
		var gameID int64
		if err := pool.QueryRow(ctx, "INSERT INTO games (guild_id, connect_code, start_time, win_type, end_time, play_map) VALUES ($1, 'ABCDEFGH', $2, $3, $4, $5) RETURNING game_id",
			g.guild, begin, int16(g.result), begin+600, g.playMap).Scan(&gameID); err != nil {
			t.Fatal(err)
		}
		ids[i+1] = gameID
		impostorWin := g.result == game.ImpostorByKill || g.result == game.ImpostorByVote
		known := g.result != game.Unknown
		for _, p := range g.players {
			won := known && (p.role == game.ImposterRole) == impostorWin
			exec("INSERT INTO users_games VALUES ($1, $2, $3, $4, $5, $6, $7)", p.id, g.guild, gameID, p.name, p.color, int16(p.role), won)
		}
		for _, e := range g.events {
			payload, _ := json.Marshal(game.Player{Action: e.action, Name: "name", IsDead: true})
			exec("INSERT INTO game_events (user_id, game_id, event_time, event_type, payload) VALUES ($1, $2, $3, $4, $5)", e.id, gameID, begin+e.offset, int16(capture.Player), string(payload))
		}
	}

	sett := settings.MakeGuildSettings()
	sett.SetLeaderboardMin(1)
	now := time.Unix(int64(start+40*day), 0)
	stats, err := buildUserStats(ctx, pool, nil, nil, "900000000000000003", "2001", premium.PremiumRecord{Tier: premium.SelfHostTier, Days: premium.NoExpiryCode}, sett, now)
	if err != nil {
		t.Fatal(err)
	}

	// Games 1-5 and 7: wins in 1, 4, and 7. As crewmate 1, 2, 5, 7 (won 1 and 7); as impostor 3 and 4 (won 4).
	wantSummary := UserStatsSummary{Games: 6, Wins: 3, Winrate: 50,
		Crewmate: RoleRecord{Games: 4, Wins: 2, Winrate: 50}, Impostor: RoleRecord{Games: 2, Wins: 1, Winrate: 50},
		FirstGame: int64(start + 5*day), LastGame: int64(start + 35*day)}
	if stats.Summary != wantSummary {
		t.Errorf("summary = %+v, want %+v", stats.Summary, wantSummary)
	}
	var recent []string
	for _, m := range stats.RecentMatches {
		recent = append(recent, m.MatchID)
	}
	if want := []string{gameIDString(ids[7]), gameIDString(ids[5]), gameIDString(ids[4]), gameIDString(ids[3]), gameIDString(ids[2]), gameIDString(ids[1])}; !reflect.DeepEqual(recent, want) {
		t.Errorf("recent = %v, want %v", recent, want)
	}
	if m := stats.RecentMatches[3]; m != (UserMatch{MatchID: gameIDString(ids[3]), StartTime: int64(start + 15*day), EndTime: int64(start+15*day) + 600, Result: "crewmateVote", Map: "polus", Name: "ali", Color: "blue", Role: "impostor"}) {
		t.Errorf("match 3 = %+v", m)
	}

	d := stats.Details
	if d == nil {
		t.Fatal("no details")
	}
	// Games: alice 6, everyone else 5. Winrate: dave 3/5, alice 3/6, bob and carol 2/5. Crewmate: dave 2/3, alice
	// 2/4, carol 2/5. Impostor: alice and dave 1/2 (alice first by user ID), bob 2/5.
	wantRanks := UserRanks{Games: &BoardRank{1, 4}, Winrate: &BoardRank{2, 4}, CrewmateWinrate: &BoardRank{2, 3}, ImpostorWinrate: &BoardRank{1, 3}}
	if !reflect.DeepEqual(d.Ranks, wantRanks) {
		t.Errorf("ranks = %+v %+v %+v %+v", *d.Ranks.Games, *d.Ranks.Winrate, *d.Ranks.CrewmateWinrate, *d.Ranks.ImpostorWinrate)
	}
	// Known results in order: W L L W (5 skipped) W.
	if d.Streaks != (Streaks{Current: 2, BestWin: 2, BestLoss: 2}) {
		t.Errorf("streaks = %+v", d.Streaks)
	}
	// Crewmate games 1, 2, 5, 7: killed in 1 (won) and 2 (lost, sent twice), voted out in 7 (won), alive in 5.
	if d.Survival != (Survival{Games: 4, Survived: 1, Rate: 25}) {
		t.Errorf("survival = %+v", d.Survival)
	}
	wantFates := UserFates{
		KilledAsCrewmate:   Fate{Times: 2, Games: 4, Rate: 50, Wins: 1, Winrate: 50},
		VotedOutAsCrewmate: Fate{Times: 1, Games: 4, Rate: 25, Wins: 1, Winrate: 100},
		VotedOutAsImpostor: Fate{Times: 1, Games: 2, Rate: 50, Wins: 0, Winrate: 0},
	}
	if d.Fates != wantFates {
		t.Errorf("fates = %+v", d.Fates)
	}
	// First to die in game 1 only; carol died first in game 2.
	if d.FirstTarget != (UserFirstTarget{FirstDeaths: 1, CrewmateGames: 4, Rate: 25}) {
		t.Errorf("first target = %+v", d.FirstTarget)
	}
	if want := []ColorShare{{"red", 4, 66.7}, {"blue", 2, 33.3}}; !reflect.DeepEqual(d.Colors, want) {
		t.Errorf("colors = %+v", d.Colors)
	}
	if want := []NameShare{{"alice", 5, 83.3}, {"ali", 1, 16.7}}; !reflect.DeepEqual(d.Names, want) {
		t.Errorf("names = %+v", d.Names)
	}
	// bob, carol, and dave each shared five of alice's six games; ties go by user ID.
	if want := []PlayedWith{{"2002", 5, 83.3}, {"2003", 5, 83.3}, {"2004", 5, 83.3}}; !reflect.DeepEqual(d.PlayedWith, want) {
		t.Errorf("played with = %+v", d.PlayedWith)
	}
	// As crewmate: with carol in 1, 2, 5 (won 1), with dave in 7 (won). As impostor: with bob in 3 and 4 (won 4).
	if want := []Teammate{{"2004", 1, 1, 100}, {"2003", 1, 3, 33.3}}; !reflect.DeepEqual(d.BestCrewmateTeammates, want) {
		t.Errorf("best crewmate teammates = %+v", d.BestCrewmateTeammates)
	}
	if want := []Teammate{{"2003", 1, 3, 33.3}, {"2004", 1, 1, 100}}; !reflect.DeepEqual(d.WorstCrewmateTeammates, want) {
		t.Errorf("worst crewmate teammates = %+v", d.WorstCrewmateTeammates)
	}
	if want := []Teammate{{"2002", 1, 2, 50}}; !reflect.DeepEqual(d.BestImpostorTeammates, want) || !reflect.DeepEqual(d.WorstImpostorTeammates, want) {
		t.Errorf("impostor teammates = %+v / %+v", d.BestImpostorTeammates, d.WorstImpostorTeammates)
	}
	// bob was impostor in crewmate games 1, 2, 7 (alice died in 1 and 2); dave in 2 and 5 (died in 2).
	if want := []DiedWith{{"2002", 2, 3, 66.7}, {"2004", 1, 2, 50}}; !reflect.DeepEqual(d.KilledBy, want) {
		t.Errorf("killed by = %+v", d.KilledBy)
	}
	// Days 5, 10, 15, 20, 25, 35 before day 40 are 5, 4, 3, 2, 2, and 0 weeks ago.
	if want := (Activity{Until: now.Unix(), Weeks: []int64{0, 0, 0, 0, 0, 0, 1, 1, 1, 2, 0, 1}}); !reflect.DeepEqual(d.Activity, want) {
		t.Errorf("activity = %+v", d.Activity)
	}
	if want := []MapRecord{{"skeld", 2, 1, 50}, {"polus", 2, 1, 50}}; !reflect.DeepEqual(d.Maps, want) {
		t.Errorf("maps = %+v", d.Maps)
	}

	// A player with no games in the guild gets an empty document from the real queries.
	empty, err := buildUserStats(ctx, pool, nil, nil, "900000000000000003", "2999", premium.PremiumRecord{Tier: premium.SelfHostTier, Days: premium.NoExpiryCode}, sett, now)
	if err != nil {
		t.Fatal(err)
	}
	if empty.Summary != (UserStatsSummary{}) || len(empty.RecentMatches) != 0 || empty.Details == nil || empty.Details.Ranks != (UserRanks{}) || empty.Details.Streaks != (Streaks{}) || len(empty.Details.Maps) != 0 {
		t.Errorf("empty = %+v %+v", empty, empty.Details)
	}
}

func gameIDString(v int64) string {
	return strconv.FormatInt(v, 10)
}

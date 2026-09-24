// seed-stats writes fake match history for one guild so the stats page and /stats command have something to
// show on a development stack. It prints SQL (and, with -redis, redis-cli commands for the players' cached
// names) rather than connecting anywhere, so it works against containers with no published ports:
//
//	go run ./cmd/seed-stats -guild 123456789012345678 | docker exec -i deploy-postgres-1 psql -U postgres
//	go run ./cmd/seed-stats -guild 123456789012345678 -redis | docker exec -i deploy-redis-1 redis-cli
//
// Every seeded game uses a connect code starting with SEED, so -clean removes them again and nothing else.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/capture"
	"github.com/automuteus/automuteus/v8/pkg/game"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
)

type player struct {
	id       uint64
	username string
	nickname string
	// skill tilts a player's odds so the winrate boards are not all fifty percent.
	skill float64
}

// Fake roster with IDs no real Discord user has (Discord snowflakes started in 2015 and this range is far in
// the future). Nicknames are empty for a few so both name forms appear on the page.
var roster = []player{
	{900000000000000001, "red_sus", "Red", 0.7},
	{900000000000000002, "bluecrew", "Blue", 0.6},
	{900000000000000003, "greenbean", "", 0.55},
	{900000000000000004, "pinkpanther", "Pink", 0.5},
	{900000000000000005, "orangejuice", "OJ", 0.45},
	{900000000000000006, "yellowsub", "", 0.5},
	{900000000000000007, "blackcat", "Cat", 0.35},
	{900000000000000008, "whitewalker", "Walker", 0.4},
	{900000000000000009, "purplerain", "Prince", 0.65},
	{900000000000000010, "brownie", "", 0.3},
	{900000000000000011, "cyanide", "Cy", 0.5},
	{900000000000000012, "limelight", "Lime", 0.55},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	guild := flag.Uint64("guild", 0, "guild ID to seed (required)")
	games := flag.Int("games", 80, "number of finished games to create")
	players := flag.Int("players", 8, "size of the fake roster to draw from (max 12), plus any -include users")
	include := flag.String("include", "", "comma-separated real user IDs to put on the roster, optionally as id:nickname")
	seed := flag.Int64("seed", 1, "random seed; the same seed always produces the same players and outcomes")
	days := flag.Int("days", 60, "spread the games over this many days ending now")
	redis := flag.Bool("redis", false, "print redis-cli commands that cache the roster's names instead of SQL")
	clean := flag.Bool("clean", false, "print SQL that removes every seeded game for the guild")
	flag.Parse()
	if *guild == 0 {
		return fmt.Errorf("-guild is required")
	}
	if *clean {
		fmt.Printf("DELETE FROM games WHERE guild_id = %d AND connect_code LIKE 'SEED%%';\n", *guild)
		return nil
	}
	if *players < 2 || *players > len(roster) {
		return fmt.Errorf("-players must be between 2 and %d", len(roster))
	}
	team := append([]player{}, roster[:*players]...)
	for spec := range strings.SplitSeq(*include, ",") {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		id, nick, _ := strings.Cut(spec, ":")
		var uid uint64
		if _, err := fmt.Sscanf(id, "%d", &uid); err != nil || uid == 0 {
			return fmt.Errorf("-include: %q is not a user ID", id)
		}
		team = append(team, player{id: uid, username: "", nickname: nick, skill: 0.6})
	}
	if *redis {
		for _, p := range team {
			if p.username == "" && p.nickname == "" {
				continue // a real user with no name given; the bot caches theirs when they next use it
			}
			username := p.username
			if username == "" {
				username = p.nickname
			}
			// The bot stores "username:nickname:discriminator"; see bot.CheckOrFetchCachedUserData.
			fmt.Printf("SET %s %q EX %d\n", rediskey.CachedUserInfoOnGuild(fmt.Sprint(p.id), fmt.Sprint(*guild)),
				fmt.Sprintf("%s:%s:0", username, p.nickname), int(rediskey.CachedUserDataExpiration.Seconds()))
			// The API's profile record, which it would otherwise try to fetch from Discord for these fake IDs.
			profile, _ := json.Marshal(map[string]string{"username": username, "nickname": p.nickname})
			fmt.Printf("SET %s %s EX %d\n", rediskey.CachedPlayerProfile(fmt.Sprint(p.id), fmt.Sprint(*guild)),
				strconv.Quote(string(profile)), int(rediskey.CachedUserDataExpiration.Seconds()))
		}
		return nil
	}
	return writeSQL(os.Stdout, *guild, team, *games, *days, rand.New(rand.NewSource(*seed)))
}

func writeSQL(w *os.File, guild uint64, team []player, games, days int, rng *rand.Rand) error {
	fmt.Fprintln(w, "BEGIN;")
	fmt.Fprintf(w, "INSERT INTO guilds (guild_id, guild_name, premium) VALUES (%d, 'Seeded guild', 0) ON CONFLICT DO NOTHING;\n", guild)
	for _, p := range team {
		fmt.Fprintf(w, "INSERT INTO users (user_id, opt) VALUES (%d, true) ON CONFLICT DO NOTHING;\n", p.id)
	}
	// Anchored to the hour so two runs in the same hour print identical SQL.
	now := time.Now().Truncate(time.Hour).Unix()
	first := now - int64(days)*86400
	for i := range games {
		start := first + int64(float64(now-first)*float64(i)/float64(games)) + rng.Int63n(3600)
		if err := writeGame(w, guild, team, i, start, rng); err != nil {
			return err
		}
	}
	fmt.Fprintln(w, "COMMIT;")
	return nil
}

func writeGame(w *os.File, guild uint64, team []player, index int, start int64, rng *rand.Rand) error {
	// 6 to 10 players, or everyone if the roster is smaller; one impostor under 8 players, else two.
	n := min(6+rng.Intn(5), len(team))
	lobby := append([]player{}, team...)
	rng.Shuffle(len(lobby), func(a, b int) { lobby[a], lobby[b] = lobby[b], lobby[a] })
	lobby = lobby[:n]
	impostors := 1
	if n >= 8 {
		impostors = 2
	}
	// The impostor side's chance follows the average skill of its players against the crew's.
	impostorSkill, crewSkill := 0.0, 0.0
	for i, p := range lobby {
		if i < impostors {
			impostorSkill += p.skill
		} else {
			crewSkill += p.skill
		}
	}
	impostorSkill /= float64(impostors)
	crewSkill /= float64(n - impostors)
	impostorWin := rng.Float64() < 0.35+0.4*(impostorSkill-crewSkill)

	// One in twelve games is ended early and must not count anywhere.
	aborted := rng.Intn(12) == 0
	duration := int64(300 + rng.Intn(600))
	var result game.GameResult
	switch {
	case aborted:
		result = game.Aborted
	case impostorWin:
		result = []game.GameResult{game.ImpostorByKill, game.ImpostorByKill, game.ImpostorByVote, game.ImpostorBySabotage}[rng.Intn(4)]
	default:
		result = []game.GameResult{game.HumansByTask, game.HumansByVote, game.HumansByVote}[rng.Intn(3)]
	}
	code := fmt.Sprintf("SEED%04d", index)
	fmt.Fprintf(w, "WITH g AS (INSERT INTO games (guild_id, connect_code, start_time, win_type, end_time) VALUES (%d, '%s', %d, %d, %d) RETURNING game_id)",
		guild, code, start, int16(result), start+duration)
	if aborted {
		// The bot records no players or result for an aborted game, but it has usually logged some events.
		fmt.Fprintf(w, "\nINSERT INTO game_events (user_id, game_id, event_time, event_type, payload) SELECT NULL::numeric, g.game_id, %d, %d, '%d'::jsonb FROM g;\n",
			start+30, capture.State, game.TASKS)
		return nil
	}

	colors := rng.Perm(18)
	rows := make([]string, 0, n)
	for i, p := range lobby {
		role := game.CrewmateRole
		won := !impostorWin
		if i < impostors {
			role = game.ImposterRole
			won = impostorWin
		}
		name := p.username
		if name == "" {
			name = p.nickname
		}
		if name == "" {
			name = "player"
		}
		if len(name) > 10 {
			name = name[:10]
		}
		rows = append(rows, fmt.Sprintf("(%d::numeric, %d::numeric, '%s', %d::smallint, %d::smallint, %t)", p.id, guild, strings.ReplaceAll(name, "'", "''"), colors[i], int16(role), won))
	}
	fmt.Fprintf(w, ",\np AS (INSERT INTO users_games (user_id, guild_id, game_id, player_name, player_color, player_role, player_won) SELECT v.user_id, v.guild_id, g.game_id, v.player_name, v.player_color, v.player_role, v.player_won FROM g, (VALUES %s) AS v(user_id, guild_id, player_name, player_color, player_role, player_won))",
		strings.Join(rows, ", "))

	// Events: a task phase, then a few rounds of kill and meeting. Crewmates die in a random order, the first
	// death being what the first-target board counts; a vote-off is recorded as an exile.
	type event struct {
		user    string
		at      int64
		kind    capture.EventType
		payload string
	}
	events := []event{{"NULL", start + 5, capture.State, fmt.Sprint(game.TASKS)}}
	crew := append([]player{}, lobby[impostors:]...)
	rng.Shuffle(len(crew), func(a, b int) { crew[a], crew[b] = crew[b], crew[a] })
	deaths := rng.Intn(len(crew))
	if impostorWin && deaths < len(crew)/2 {
		deaths = len(crew) / 2
	}
	t := start + 60
	for i := 0; i < deaths && t < start+duration-30; i++ {
		p := crew[i]
		payload, _ := json.Marshal(game.Player{Action: game.DIED, Name: p.username, Color: colors[impostors+i], IsDead: true})
		events = append(events, event{fmt.Sprint(p.id), t, capture.Player, string(payload)})
		t += int64(20 + rng.Intn(60))
		if rng.Intn(2) == 0 {
			events = append(events, event{"NULL", t, capture.State, fmt.Sprint(game.DISCUSS)})
			t += 45
			if rng.Intn(3) == 0 && i+1 < len(crew) {
				// The meeting votes out an innocent crewmate.
				i++
				v := crew[i]
				payload, _ := json.Marshal(game.Player{Action: game.EXILED, Name: v.username, Color: colors[impostors+i], IsDead: true})
				events = append(events, event{fmt.Sprint(v.id), t, capture.Player, string(payload)})
			}
			events = append(events, event{"NULL", t + 5, capture.State, fmt.Sprint(game.TASKS)})
			t += 30
		}
	}
	values := make([]string, 0, len(events))
	for _, e := range events {
		user := "NULL::numeric"
		if e.user != "NULL" {
			user = e.user + "::numeric"
		}
		values = append(values, fmt.Sprintf("(%s, %d, %d::smallint, '%s'::jsonb)", user, e.at, e.kind, strings.ReplaceAll(e.payload, "'", "''")))
	}
	fmt.Fprintf(w, "\nINSERT INTO game_events (user_id, game_id, event_time, event_type, payload) SELECT v.user_id, g.game_id, v.event_time, v.event_type, v.payload FROM g, (VALUES %s) AS v(user_id, event_time, event_type, payload);\n",
		strings.Join(values, ", "))
	return nil
}

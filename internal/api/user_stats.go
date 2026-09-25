package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/automuteus/automuteus/v8/pkg/game"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	pgstorage "github.com/automuteus/automuteus/v8/pkg/storage"
	"github.com/georgysavva/scany/pgxscan"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"golang.org/x/sync/errgroup"
)

// recentMatchesSize is how many of a player's latest matches the player stats document lists.
const recentMatchesSize = 10

// activityWeeks is how many weeks of games per week the player stats document reports.
const activityWeeks = 12

// UserStats is the GET /guild/user response: one player's statistics in one guild. Summary and RecentMatches
// are always present; Details is present only while the guild's premium is active, as with GET /guild/stats.
// A player with no recorded games gets a zero summary and no matches rather than an error.
type UserStats struct {
	GuildID string `json:"guildId"`
	UserID  string `json:"userId"`
	// Premium is the guild's premium status the document was built under.
	Premium premium.PremiumRecord `json:"premium"`
	// GeneratedAt is the Unix time the document was built; responses may be served from a short cache.
	GeneratedAt int64            `json:"generatedAt"`
	Summary     UserStatsSummary `json:"summary"`
	// RecentMatches are the player's latest finished matches, newest first, at most ten. Their match IDs work
	// with GET /guild/match.
	RecentMatches []UserMatch `json:"recentMatches"`
	// Details is omitted for guilds whose premium is free or expired.
	Details *UserStatsDetails `json:"details,omitempty"`
	// Players maps the player and every user ID named in Details to a name and picture, resolved as for
	// GET /guild/stats.
	Players map[string]StatsPlayer `json:"players"`
}

// UserStatsSummary is what every guild sees. Winrates are percentages of the player's games in that role,
// counted as the guild winrate boards count them.
type UserStatsSummary struct {
	Games    int64      `json:"games"`
	Wins     int64      `json:"wins"`
	Winrate  float64    `json:"winrate"`
	Crewmate RoleRecord `json:"crewmate"`
	Impostor RoleRecord `json:"impostor"`
	// FirstGame and LastGame are the start times of the player's first and latest games; absent with no games.
	FirstGame int64 `json:"firstGame,omitempty"`
	LastGame  int64 `json:"lastGame,omitempty"`
}

type RoleRecord struct {
	Games   int64   `json:"games"`
	Wins    int64   `json:"wins"`
	Winrate float64 `json:"winrate"`
}

// UserMatch is one of the player's matches and how they played it.
type UserMatch struct {
	MatchID   string `json:"matchId"`
	StartTime int64  `json:"startTime"`
	EndTime   int64  `json:"endTime"`
	// Result uses the keys of MatchSummary.Result.
	Result string `json:"result"`
	// Map is absent for matches recorded before maps were stored.
	Map   string `json:"map,omitempty"`
	Name  string `json:"name"`
	Color string `json:"color"`
	Role  string `json:"role"`
	Won   bool   `json:"won"`
}

// UserStatsDetails are the premium sections. Every list is present, possibly empty.
type UserStatsDetails struct {
	// MinGames is the guild's leaderboard minimum: the games the player needs to be ranked by winrate, and a
	// crewmate teammate or impostor needs to be listed. Impostor teammates need two shared games instead.
	MinGames int       `json:"minGames"`
	Ranks    UserRanks `json:"ranks"`
	Streaks  Streaks   `json:"streaks"`
	// Survival is how often the player was still alive at the end of their crewmate games.
	Survival Survival  `json:"survival"`
	Fates    UserFates `json:"fates"`
	// FirstTarget is how often the player was the first to die, of their crewmate games.
	FirstTarget UserFirstTarget `json:"firstTarget"`
	Colors      []ColorShare    `json:"colors"`
	Names       []NameShare     `json:"names"`
	// PlayedWith ranks the linked players the player shared the most games with.
	PlayedWith []PlayedWith `json:"playedWith"`
	// The teammate boards rank players who shared a role with the player. Best is highest winrate first;
	// worst is lowest first.
	BestCrewmateTeammates  []Teammate `json:"bestCrewmateTeammates"`
	WorstCrewmateTeammates []Teammate `json:"worstCrewmateTeammates"`
	BestImpostorTeammates  []Teammate `json:"bestImpostorTeammates"`
	WorstImpostorTeammates []Teammate `json:"worstImpostorTeammates"`
	// KilledBy ranks impostors by how often the player died, as a crewmate, with them in the game. The game
	// never reports who made a kill, so a death counts against every impostor of that game.
	KilledBy []DiedWith `json:"killedBy"`
	Activity Activity   `json:"activity"`
	// Maps is the player's record per map, most played first, over the games whose map was recorded.
	Maps []MapRecord `json:"maps"`
}

// UserRanks place the player on the guild boards; each is absent when the player is not ranked on it.
type UserRanks struct {
	Games           *BoardRank `json:"games,omitempty"`
	Winrate         *BoardRank `json:"winrate,omitempty"`
	CrewmateWinrate *BoardRank `json:"crewmateWinrate,omitempty"`
	ImpostorWinrate *BoardRank `json:"impostorWinrate,omitempty"`
}

// BoardRank is a place on a board (1 is first) and how many players the board ranks.
type BoardRank struct {
	Position int64 `json:"position"`
	Players  int64 `json:"players"`
}

// Streaks count consecutive games with a known winner. Current is the run the player is on now: positive for
// wins, negative for losses, zero with no games.
type Streaks struct {
	Current  int `json:"current"`
	BestWin  int `json:"bestWin"`
	BestLoss int `json:"bestLoss"`
}

type Survival struct {
	Games    int64   `json:"games"`
	Survived int64   `json:"survived"`
	Rate     float64 `json:"rate"`
}

// UserFates are how often the player was killed or voted out in a role, and how often their side won those
// games anyway.
type UserFates struct {
	KilledAsCrewmate   Fate `json:"killedAsCrewmate"`
	VotedOutAsCrewmate Fate `json:"votedOutAsCrewmate"`
	VotedOutAsImpostor Fate `json:"votedOutAsImpostor"`
}

// Fate is Times of the player's Games in a role; Wins is how many of those Times their side still won.
type Fate struct {
	Times   int64   `json:"times"`
	Games   int64   `json:"games"`
	Rate    float64 `json:"rate"`
	Wins    int64   `json:"wins"`
	Winrate float64 `json:"winrate"`
}

type UserFirstTarget struct {
	FirstDeaths   int64   `json:"firstDeaths"`
	CrewmateGames int64   `json:"crewmateGames"`
	Rate          float64 `json:"rate"`
}

// ColorShare and NameShare are how often the player used a color or name, as a percentage of their games.
type ColorShare struct {
	Color string  `json:"color"`
	Games int64   `json:"games"`
	Share float64 `json:"share"`
}

type NameShare struct {
	Name  string  `json:"name"`
	Games int64   `json:"games"`
	Share float64 `json:"share"`
}

// PlayedWith is a player the user shared Games with; Share is the percentage of the user's games.
type PlayedWith struct {
	UserID string  `json:"userId"`
	Games  int64   `json:"games"`
	Share  float64 `json:"share"`
}

type Teammate struct {
	UserID  string  `json:"userId"`
	Wins    int64   `json:"wins"`
	Games   int64   `json:"games"`
	Winrate float64 `json:"winrate"`
}

type DiedWith struct {
	ImpostorID string  `json:"impostorId"`
	Deaths     int64   `json:"deaths"`
	Games      int64   `json:"games"`
	Rate       float64 `json:"rate"`
}

// Activity counts the player's games per week. Weeks runs oldest first and ends with the week up to Until.
type Activity struct {
	Until int64   `json:"until"`
	Weeks []int64 `json:"weeks"`
}

type MapRecord struct {
	Map     string  `json:"map"`
	Games   int64   `json:"games"`
	Wins    int64   `json:"wins"`
	Winrate float64 `json:"winrate"`
}

// UserStats builds the player stats document for one player of a guild.
func (s *DataStore) UserStats(ctx context.Context, guildID, userID string) (UserStats, error) {
	record, err := s.Premium(ctx, guildID)
	if err != nil {
		return UserStats{}, fmt.Errorf("premium status: %w", err)
	}
	sett, _, err := s.Settings(ctx, guildID)
	if err != nil {
		return UserStats{}, fmt.Errorf("guild settings: %w", err)
	}
	return buildUserStats(ctx, s.stats, s.redis, s.profiles, guildID, userID, record, sett, time.Now())
}

func buildUserStats(ctx context.Context, db pgxscan.Querier, client *redis.Client, profiles ProfileFetcher, guildID, userID string, record premium.PremiumRecord, sett *settings.GuildSettings, now time.Time) (UserStats, error) {
	gid, err := strconv.ParseUint(guildID, 10, 64)
	if err != nil {
		return UserStats{}, fmt.Errorf("guild ID: %w", err)
	}
	uid, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		return UserStats{}, fmt.Errorf("user ID: %w", err)
	}
	stats := UserStats{
		GuildID:     guildID,
		UserID:      userID,
		Premium:     record,
		GeneratedAt: now.Unix(),
	}

	summary, err := pgstorage.UserSummaryStats(ctx, db, gid, uid)
	if err != nil {
		return UserStats{}, fmt.Errorf("summary: %w", err)
	}
	stats.Summary = UserStatsSummary{
		Games:    summary.Games,
		Wins:     summary.Wins,
		Winrate:  percent(summary.Wins, summary.Games),
		Crewmate: RoleRecord{Games: summary.CrewmateGames, Wins: summary.CrewmateWins, Winrate: percent(summary.CrewmateWins, summary.CrewmateGames)},
		Impostor: RoleRecord{Games: summary.ImpostorGames, Wins: summary.ImpostorWins, Winrate: percent(summary.ImpostorWins, summary.ImpostorGames)},
	}
	if summary.FirstGame != nil && summary.LastGame != nil {
		stats.Summary.FirstGame, stats.Summary.LastGame = *summary.FirstGame, *summary.LastGame
	}
	matches, err := pgstorage.UserRecentMatches(ctx, db, gid, uid, recentMatchesSize)
	if err != nil {
		return UserStats{}, fmt.Errorf("recent matches: %w", err)
	}
	stats.RecentMatches = make([]UserMatch, 0, len(matches))
	for _, m := range matches {
		stats.RecentMatches = append(stats.RecentMatches, userMatch(m))
	}

	ids := []string{userID}
	if !premium.IsExpired(record.Tier, record.Days) {
		details, err := buildUserDetails(ctx, db, gid, uid, summary, sett.GetLeaderboardMin(), now)
		if err != nil {
			return UserStats{}, err
		}
		stats.Details = details
		ids = details.userIDs(userID)
	}
	stats.Players = resolvePlayers(ctx, client, profiles, guildID, ids)
	return stats, nil
}

func userMatch(m *pgstorage.UserMatch) UserMatch {
	match := UserMatch{
		MatchID:   strconv.FormatInt(m.GameID, 10),
		StartTime: int64(m.StartTime),
		EndTime:   int64(m.EndTime),
		Result:    "unknown",
		Name:      m.PlayerName,
		Color:     game.GetColorStringForInt(int(m.PlayerColor)),
		Role:      "crewmate",
		Won:       m.PlayerWon,
	}
	if key, ok := matchResults[game.GameResult(m.WinType)]; ok {
		match.Result = key
	}
	if m.PlayMap != nil {
		match.Map = matchMaps[game.PlayMap(*m.PlayMap)]
	}
	if game.GameRole(m.PlayerRole) == game.ImposterRole {
		match.Role = "impostor"
	}
	return match
}

// buildUserDetails runs the premium queries a few at a time; the first failure cancels the rest.
func buildUserDetails(ctx context.Context, db pgxscan.Querier, gid, uid uint64, summary pgstorage.UserSummary, minGames int, now time.Time) (*UserStatsDetails, error) {
	const size = leaderboardSize
	d := &UserStatsDetails{MinGames: minGames}
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(statsQueryParallelism)

	g.Go(func() error {
		var err error
		if d.Ranks.Games, err = boardRank(pgstorage.UserGamesRank(ctx, db, gid, uid)); err != nil {
			return fmt.Errorf("games rank: %w", err)
		}
		return nil
	})
	winRanks := []struct {
		name string
		role int16
		dest **BoardRank
	}{
		{"winrate rank", pgstorage.AnyRole, &d.Ranks.Winrate},
		{"crewmate winrate rank", int16(game.CrewmateRole), &d.Ranks.CrewmateWinrate},
		{"impostor winrate rank", int16(game.ImposterRole), &d.Ranks.ImpostorWinrate},
	}
	for _, r := range winRanks {
		r := r
		g.Go(func() error {
			rank, err := boardRank(pgstorage.UserWinRank(ctx, db, gid, uid, r.role, minGames))
			if err != nil {
				return fmt.Errorf("%s: %w", r.name, err)
			}
			*r.dest = rank
			return nil
		})
	}
	g.Go(func() error {
		results, err := pgstorage.UserResults(ctx, db, gid, uid)
		if err != nil {
			return fmt.Errorf("results: %w", err)
		}
		d.Streaks = streaks(results)
		return nil
	})
	g.Go(func() error {
		rows, err := pgstorage.UserFatesByRole(ctx, db, gid, uid)
		if err != nil {
			return fmt.Errorf("fates: %w", err)
		}
		for _, r := range rows {
			switch game.GameRole(r.Role) {
			case game.CrewmateRole:
				d.Survival = Survival{Games: r.Games, Survived: r.Games - r.Eliminated, Rate: percent(r.Games-r.Eliminated, r.Games)}
				d.Fates.KilledAsCrewmate = fate(r.Died, r.DiedWon, r.Games)
				d.Fates.VotedOutAsCrewmate = fate(r.Exiled, r.ExiledWon, r.Games)
				d.FirstTarget = UserFirstTarget{FirstDeaths: r.FirstDeaths, CrewmateGames: r.Games, Rate: percent(r.FirstDeaths, r.Games)}
			case game.ImposterRole:
				d.Fates.VotedOutAsImpostor = fate(r.Exiled, r.ExiledWon, r.Games)
			}
		}
		return nil
	})
	g.Go(func() error {
		rows, err := pgstorage.UserColors(ctx, db, gid, uid, size)
		if err != nil {
			return fmt.Errorf("colors: %w", err)
		}
		d.Colors = make([]ColorShare, 0, len(rows))
		for _, r := range rows {
			d.Colors = append(d.Colors, ColorShare{Color: game.GetColorStringForInt(int(r.Mode)), Games: r.Count, Share: percent(r.Count, summary.Games)})
		}
		return nil
	})
	g.Go(func() error {
		rows, err := pgstorage.UserNames(ctx, db, gid, uid, size)
		if err != nil {
			return fmt.Errorf("names: %w", err)
		}
		d.Names = make([]NameShare, 0, len(rows))
		for _, r := range rows {
			d.Names = append(d.Names, NameShare{Name: r.Mode, Games: r.Count, Share: percent(r.Count, summary.Games)})
		}
		return nil
	})
	g.Go(func() error {
		rows, err := pgstorage.UserPlayedWith(ctx, db, gid, uid, size)
		if err != nil {
			return fmt.Errorf("played with: %w", err)
		}
		d.PlayedWith = make([]PlayedWith, 0, len(rows))
		for _, r := range rows {
			d.PlayedWith = append(d.PlayedWith, PlayedWith{UserID: snowflake(r.UserID), Games: r.Count, Share: percent(r.Count, summary.Games)})
		}
		return nil
	})
	teammateBoards := []struct {
		name  string
		role  game.GameRole
		min   int
		worst bool
		dest  *[]Teammate
	}{
		{"best crewmate teammates", game.CrewmateRole, minGames, false, &d.BestCrewmateTeammates},
		{"worst crewmate teammates", game.CrewmateRole, minGames, true, &d.WorstCrewmateTeammates},
		{"best impostor teammates", game.ImposterRole, impostorDuoMinGames, false, &d.BestImpostorTeammates},
		{"worst impostor teammates", game.ImposterRole, impostorDuoMinGames, true, &d.WorstImpostorTeammates},
	}
	for _, b := range teammateBoards {
		b := b
		g.Go(func() error {
			rows, err := pgstorage.UserTeammates(ctx, db, gid, uid, b.role, b.min, size, b.worst)
			if err != nil {
				return fmt.Errorf("%s: %w", b.name, err)
			}
			out := make([]Teammate, 0, len(rows))
			for _, r := range rows {
				out = append(out, Teammate{UserID: snowflake(r.TeammateID), Wins: r.WinCount, Games: r.Count, Winrate: round1(r.WinRate)})
			}
			*b.dest = out
			return nil
		})
	}
	g.Go(func() error {
		rows, err := pgstorage.UserKilledBy(ctx, db, gid, uid, minGames, size)
		if err != nil {
			return fmt.Errorf("killed by: %w", err)
		}
		d.KilledBy = make([]DiedWith, 0, len(rows))
		for _, r := range rows {
			d.KilledBy = append(d.KilledBy, DiedWith{ImpostorID: snowflake(r.TeammateID), Deaths: r.TotalDeath, Games: r.Encounter, Rate: round1(r.DeathRate)})
		}
		return nil
	})
	g.Go(func() error {
		rows, err := pgstorage.UserActivity(ctx, db, gid, uid, now.Unix(), activityWeeks)
		if err != nil {
			return fmt.Errorf("activity: %w", err)
		}
		d.Activity = Activity{Until: now.Unix(), Weeks: make([]int64, activityWeeks)}
		for _, r := range rows {
			if r.WeeksAgo >= 0 && r.WeeksAgo < activityWeeks {
				d.Activity.Weeks[activityWeeks-1-r.WeeksAgo] = r.Games
			}
		}
		return nil
	})
	g.Go(func() error {
		rows, err := pgstorage.UserMaps(ctx, db, gid, uid)
		if err != nil {
			return fmt.Errorf("maps: %w", err)
		}
		d.Maps = make([]MapRecord, 0, len(rows))
		for _, r := range rows {
			// A map the API has no key for yet is left out rather than sent unnamed.
			if key, ok := matchMaps[game.PlayMap(r.PlayMap)]; ok {
				d.Maps = append(d.Maps, MapRecord{Map: key, Games: r.Games, Wins: r.Wins, Winrate: percent(r.Wins, r.Games)})
			}
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return d, nil
}

func boardRank(rank *pgstorage.UserRank, err error) (*BoardRank, error) {
	if err != nil || rank == nil {
		return nil, err
	}
	return &BoardRank{Position: rank.Position, Players: rank.Players}, nil
}

func fate(times, wins, games int64) Fate {
	return Fate{Times: times, Games: games, Rate: percent(times, games), Wins: wins, Winrate: percent(wins, times)}
}

// streaks walks a player's results, oldest first.
func streaks(results []bool) Streaks {
	var s Streaks
	for _, won := range results {
		switch {
		case won && s.Current > 0:
			s.Current++
		case won:
			s.Current = 1
		case s.Current < 0:
			s.Current--
		default:
			s.Current = -1
		}
		if s.Current > s.BestWin {
			s.BestWin = s.Current
		}
		if -s.Current > s.BestLoss {
			s.BestLoss = -s.Current
		}
	}
	return s
}

// userIDs lists the player and everyone named in the details, once each, the player first.
func (d *UserStatsDetails) userIDs(userID string) []string {
	seen := map[string]bool{userID: true}
	var others []string
	add := func(id string) {
		if !seen[id] {
			seen[id] = true
			others = append(others, id)
		}
	}
	for _, r := range d.PlayedWith {
		add(r.UserID)
	}
	for _, board := range [][]Teammate{d.BestCrewmateTeammates, d.WorstCrewmateTeammates, d.BestImpostorTeammates, d.WorstImpostorTeammates} {
		for _, r := range board {
			add(r.UserID)
		}
	}
	for _, r := range d.KilledBy {
		add(r.ImpostorID)
	}
	sort.Strings(others)
	return append([]string{userID}, others...)
}

// UserStats godoc
// @Summary Get Player Stats
// @Description One player's statistics in the guild. Every guild gets the player's games, wins, and winrates overall
// @Description and per role, and their ten latest matches; guilds with active premium also get the player's ranks
// @Description on the guild boards, streaks, survival, how often they were killed or voted out, first-target
// @Description rate, favorite colors and names, who they played with and won or lost with, the impostors they
// @Description died with, games per week, and per-map records. Any member may read any player.
// @Description Responses may be up to a minute old.
// @Security BasicAuth
// @Security DiscordBearer
// @Tags guild
// @Accept json
// @Produce json
// @Param guildID query string true "Guild ID"
// @Param userID query string true "User ID"
// @Success 200 {object} UserStats
// @Failure 400 {object} HttpError
// @Failure 401 {object} HttpError
// @Failure 403 {object} HttpError
// @Failure 500 {object} HttpError
// @Router /guild/user [get]
func handleGetUserStats(users *listCache[UserStats]) func(c *gin.Context) {
	return func(c *gin.Context) {
		guildID := c.Query("guildID")
		if discord.ValidateSnowflake(guildID) != nil {
			c.JSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "invalid guild ID",
			})
			return
		}
		userID := c.Query("userID")
		if discord.ValidateSnowflake(userID) != nil {
			c.JSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "invalid user ID",
			})
			return
		}
		result, err := users.get(c.Request.Context(), guildID+"/"+userID)
		if err != nil {
			log.Printf("Guild %s user %s stats: %v\n", guildID, userID, err)
			c.JSON(http.StatusInternalServerError, HttpError{
				StatusCode: http.StatusInternalServerError,
				Error:      "Unable to load player statistics",
			})
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// Player documents share the per-guild cache type, keyed by guild and user together.
func newUserStatsCache(ttl time.Duration, build func(ctx context.Context, guildID, userID string) (UserStats, error)) *listCache[UserStats] {
	return newListCache(ttl, nil, func(ctx context.Context, key string) (UserStats, error) {
		guildID, userID, _ := strings.Cut(key, "/")
		return build(ctx, guildID, userID)
	})
}

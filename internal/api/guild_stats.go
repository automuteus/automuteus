package api

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
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

// DefaultStatsCacheTTL bounds how long GET /guild/stats reuses a guild's rollup. Statistics only change when a
// game ends, and the premium boards are several self-joins over the guild's whole history, so a page held on
// refresh or a crowd opening the same guild should cost Postgres one build a minute, not one per view.
const DefaultStatsCacheTTL = time.Minute

// leaderboardSize is how many entries every board holds. The bot's leaderboard size setting is not consulted: on
// a page a fixed count keeps the boards the same height, and the setting is being retired.
const leaderboardSize = 5

// impostorDuoMinGames is the fixed floor for the impostor duo boards. Two players are impostors together far
// less often than crewmates, so the guild's leaderboard minimum would leave those boards empty for most guilds.
const impostorDuoMinGames = 2

// statsQueryParallelism bounds how many of a document's queries run at once. Running all of them together made a
// large guild's build starve every other request on the shared database, including the readiness probe.
const statsQueryParallelism = 3

// GuildStats is the GET /guild/stats response: everything the stats page shows for one guild in one document.
// Summary is always present. Leaderboards is present only while the guild's premium is active, as with the
// retired /stats guild slash command; a free guild sees no key at all rather than empty boards.
type GuildStats struct {
	GuildID string `json:"guildId"`
	// Premium is the guild's premium status the rollup was built under, so the page can explain a missing
	// leaderboards section without a second request.
	Premium premium.PremiumRecord `json:"premium"`
	// GeneratedAt is the Unix time the rollup was built; responses may be served from a short cache.
	GeneratedAt int64             `json:"generatedAt"`
	Summary     GuildStatsSummary `json:"summary"`
	// Leaderboards is omitted for guilds whose premium is free or expired.
	Leaderboards *GuildLeaderboards `json:"leaderboards,omitempty"`
	// Players maps every user ID named in the leaderboards to a name and picture, resolved through Discord with
	// the bot's credentials. IDs nothing knows are absent and the page shows
	// the ID itself.
	Players map[string]StatsPlayer `json:"players"`
}

// GuildStatsSummary is what every guild sees. Winrates are percentages of finished games; a game whose result
// was unknown counts toward neither side, so the two need not sum to one hundred.
type GuildStatsSummary struct {
	GamesPlayed     int64   `json:"gamesPlayed"`
	CrewmateWins    int64   `json:"crewmateWins"`
	ImpostorWins    int64   `json:"impostorWins"`
	CrewmateWinrate float64 `json:"crewmateWinrate"`
	ImpostorWinrate float64 `json:"impostorWinrate"`
}

// GuildLeaderboards are the premium boards, each trimmed to five entries and, where a minimum applies, to players
// with at least the guild's leaderboard minimum of games. Every board is present, possibly empty.
type GuildLeaderboards struct {
	// MinGames is the guild's leaderboard minimum: the games a player or crewmate duo needs to be ranked by
	// rate. The impostor duo boards use a fixed floor of two shared games instead.
	MinGames int `json:"minGames"`
	// MostGames ranks players by games recorded, with no minimum.
	MostGames []PlayerGames `json:"mostGames"`
	// Winrate ranks players by winrate across both roles.
	Winrate         []PlayerWinrate `json:"winrate"`
	CrewmateWinrate []PlayerWinrate `json:"crewmateWinrate"`
	ImpostorWinrate []PlayerWinrate `json:"impostorWinrate"`
	// The duo boards rank pairs of players who shared a role in a game. Each pair appears once, lower user ID
	// first. Best is highest winrate first; worst is lowest first.
	BestImpostorDuo  []DuoWinrate `json:"bestImpostorDuo"`
	WorstImpostorDuo []DuoWinrate `json:"worstImpostorDuo"`
	BestCrewmateDuo  []DuoWinrate `json:"bestCrewmateDuo"`
	WorstCrewmateDuo []DuoWinrate `json:"worstCrewmateDuo"`
	// FirstTarget ranks players by how often they were the first to die.
	FirstTarget []FirstTarget `json:"firstTarget"`
	// KilledBy ranks crewmate and impostor pairs by how often the crewmate died with that impostor in the game.
	// The game never reports who made a kill, so a death counts against every impostor of that game.
	KilledBy []KilledBy `json:"killedBy"`
}

type PlayerGames struct {
	UserID string `json:"userId"`
	Games  int64  `json:"games"`
}

type PlayerWinrate struct {
	UserID  string  `json:"userId"`
	Wins    int64   `json:"wins"`
	Games   int64   `json:"games"`
	Winrate float64 `json:"winrate"`
}

type DuoWinrate struct {
	UserID     string  `json:"userId"`
	TeammateID string  `json:"teammateId"`
	Wins       int64   `json:"wins"`
	Games      int64   `json:"games"`
	Winrate    float64 `json:"winrate"`
}

type FirstTarget struct {
	UserID string `json:"userId"`
	// FirstDeaths is how many games this player was the first to die in.
	FirstDeaths int64 `json:"firstDeaths"`
	// CrewmateGames is how many games the player was a crewmate in, the denominator of Rate.
	CrewmateGames int64   `json:"crewmateGames"`
	Rate          float64 `json:"rate"`
}

type KilledBy struct {
	UserID     string `json:"userId"`
	ImpostorID string `json:"impostorId"`
	// Deaths is how many of the shared games the crewmate died in.
	Deaths int64 `json:"deaths"`
	// Games is how many games the two shared as crewmate and impostor, the denominator of Rate.
	Games int64   `json:"games"`
	Rate  float64 `json:"rate"`
}

// GuildStats builds the stats page document for a guild. The summary is one query; the premium boards run
// concurrently and are skipped entirely for a guild whose premium is free or expired, so a free guild costs
// one count query.
func (s *DataStore) GuildStats(ctx context.Context, guildID string) (GuildStats, error) {
	return s.guildStats(ctx, guildID, false)
}

// AdminGuildStats is GuildStats with the leaderboards built whatever the guild's premium, for operators looking at
// a guild from outside. The premium record in the document is still the guild's real one.
func (s *DataStore) AdminGuildStats(ctx context.Context, guildID string) (GuildStats, error) {
	return s.guildStats(ctx, guildID, true)
}

func (s *DataStore) guildStats(ctx context.Context, guildID string, full bool) (GuildStats, error) {
	record, err := s.Premium(ctx, guildID)
	if err != nil {
		return GuildStats{}, fmt.Errorf("premium status: %w", err)
	}
	sett, _, err := s.Settings(ctx, guildID)
	if err != nil {
		return GuildStats{}, fmt.Errorf("guild settings: %w", err)
	}
	return buildGuildStats(ctx, s.stats, s.redis, s.profiles, guildID, record, sett, full)
}

// buildGuildStats assembles the document; full includes the leaderboards even when the premium record says the
// guild should not see them.
func buildGuildStats(ctx context.Context, db pgxscan.Querier, client *redis.Client, profiles ProfileFetcher, guildID string, record premium.PremiumRecord, sett *settings.GuildSettings, full bool) (GuildStats, error) {
	gid, err := strconv.ParseUint(guildID, 10, 64)
	if err != nil {
		return GuildStats{}, fmt.Errorf("guild ID: %w", err)
	}
	stats := GuildStats{
		GuildID:     guildID,
		Premium:     record,
		GeneratedAt: time.Now().Unix(),
		Players:     map[string]StatsPlayer{},
	}

	summary, err := pgstorage.GuildSummaryStats(ctx, db, gid)
	if err != nil {
		return GuildStats{}, fmt.Errorf("summary: %w", err)
	}
	stats.Summary = GuildStatsSummary{
		GamesPlayed:     summary.GamesPlayed,
		CrewmateWins:    summary.CrewmateWins,
		ImpostorWins:    summary.ImpostorWins,
		CrewmateWinrate: percent(summary.CrewmateWins, summary.GamesPlayed),
		ImpostorWinrate: percent(summary.ImpostorWins, summary.GamesPlayed),
	}
	if !full && premium.IsExpired(record.Tier, record.Days) {
		return stats, nil
	}

	boards, err := buildGuildLeaderboards(ctx, db, gid, sett.GetLeaderboardMin())
	if err != nil {
		return GuildStats{}, err
	}
	stats.Leaderboards = boards
	stats.Players = resolvePlayers(ctx, client, profiles, guildID, boards.userIDs())
	return stats, nil
}

// buildGuildLeaderboards runs every premium board concurrently. The first failure cancels the rest; a partial
// set of boards would be indistinguishable from empty ones on the page.
func buildGuildLeaderboards(ctx context.Context, db pgxscan.Querier, gid uint64, minGames int) (*GuildLeaderboards, error) {
	const size = leaderboardSize
	boards := &GuildLeaderboards{MinGames: minGames}
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(statsQueryParallelism)
	g.Go(func() error {
		rows, err := pgstorage.GuildMostGames(ctx, db, gid, size)
		if err != nil {
			return fmt.Errorf("most games: %w", err)
		}
		boards.MostGames = make([]PlayerGames, 0, len(rows))
		for _, r := range rows {
			boards.MostGames = append(boards.MostGames, PlayerGames{UserID: snowflake(r.UserID), Games: r.Count})
		}
		return nil
	})
	winBoards := []struct {
		name string
		role int16
		dest *[]PlayerWinrate
	}{
		{"winrate", pgstorage.AnyRole, &boards.Winrate},
		{"crewmate winrate", int16(game.CrewmateRole), &boards.CrewmateWinrate},
		{"impostor winrate", int16(game.ImposterRole), &boards.ImpostorWinrate},
	}
	for _, b := range winBoards {
		b := b
		g.Go(func() error {
			rows, err := pgstorage.GuildWinRanking(ctx, db, gid, b.role, minGames, size)
			if err != nil {
				return fmt.Errorf("%s: %w", b.name, err)
			}
			out := make([]PlayerWinrate, 0, len(rows))
			for _, r := range rows {
				out = append(out, PlayerWinrate{UserID: snowflake(r.UserID), Wins: r.WinCount, Games: r.Count, Winrate: round1(r.WinRate)})
			}
			*b.dest = out
			return nil
		})
	}
	duoBoards := []struct {
		name  string
		role  game.GameRole
		min   int
		worst bool
		dest  *[]DuoWinrate
	}{
		{"best impostor duo", game.ImposterRole, impostorDuoMinGames, false, &boards.BestImpostorDuo},
		{"worst impostor duo", game.ImposterRole, impostorDuoMinGames, true, &boards.WorstImpostorDuo},
		{"best crewmate duo", game.CrewmateRole, minGames, false, &boards.BestCrewmateDuo},
		{"worst crewmate duo", game.CrewmateRole, minGames, true, &boards.WorstCrewmateDuo},
	}
	for _, b := range duoBoards {
		b := b
		g.Go(func() error {
			rows, err := pgstorage.GuildDuoRanking(ctx, db, gid, b.role, b.min, size, b.worst)
			if err != nil {
				return fmt.Errorf("%s: %w", b.name, err)
			}
			out := make([]DuoWinrate, 0, len(rows))
			for _, r := range rows {
				out = append(out, DuoWinrate{UserID: snowflake(r.UserID), TeammateID: snowflake(r.TeammateID), Wins: r.WinCount, Games: r.Count, Winrate: round1(r.WinRate)})
			}
			*b.dest = out
			return nil
		})
	}
	g.Go(func() error {
		rows, err := pgstorage.GuildFirstTargetRanking(ctx, db, gid, minGames, size)
		if err != nil {
			return fmt.Errorf("first target: %w", err)
		}
		boards.FirstTarget = make([]FirstTarget, 0, len(rows))
		for _, r := range rows {
			boards.FirstTarget = append(boards.FirstTarget, FirstTarget{UserID: snowflake(r.UserID), FirstDeaths: r.TotalDeath, CrewmateGames: r.Count, Rate: round1(r.DeathRate)})
		}
		return nil
	})
	g.Go(func() error {
		rows, err := pgstorage.GuildKilledByRanking(ctx, db, gid, minGames, size)
		if err != nil {
			return fmt.Errorf("killed by: %w", err)
		}
		boards.KilledBy = make([]KilledBy, 0, len(rows))
		for _, r := range rows {
			boards.KilledBy = append(boards.KilledBy, KilledBy{UserID: snowflake(r.UserID), ImpostorID: snowflake(r.TeammateID), Deaths: r.TotalDeath, Games: r.Encounter, Rate: round1(r.DeathRate)})
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return boards, nil
}

// userIDs lists every user named on any board, once each, in a stable order.
func (b *GuildLeaderboards) userIDs() []string {
	seen := map[string]bool{}
	add := func(ids ...string) {
		for _, id := range ids {
			seen[id] = true
		}
	}
	for _, r := range b.MostGames {
		add(r.UserID)
	}
	for _, board := range [][]PlayerWinrate{b.Winrate, b.CrewmateWinrate, b.ImpostorWinrate} {
		for _, r := range board {
			add(r.UserID)
		}
	}
	for _, board := range [][]DuoWinrate{b.BestImpostorDuo, b.WorstImpostorDuo, b.BestCrewmateDuo, b.WorstCrewmateDuo} {
		for _, r := range board {
			add(r.UserID, r.TeammateID)
		}
	}
	for _, r := range b.FirstTarget {
		add(r.UserID)
	}
	for _, r := range b.KilledBy {
		add(r.UserID, r.ImpostorID)
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func snowflake(id uint64) string {
	return strconv.FormatUint(id, 10)
}

func percent(part, whole int64) float64 {
	if whole == 0 {
		return 0
	}
	return round1(100 * float64(part) / float64(whole))
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// GuildStats godoc
// @Summary Get Guild Stats
// @Description The guild statistics page in one document. Every guild gets the summary (games played and each
// @Description side's wins); guilds with active premium also get the leaderboards,
// @Description five entries per board, honouring the guild's leaderboard minimum. User IDs are resolved to
// @Description names and avatars through Discord where possible. Responses are built at most once a minute per guild.
// @Security BasicAuth
// @Security DiscordBearer
// @Tags guild
// @Accept json
// @Produce json
// @Param guildID query string true "Guild ID"
// @Param full query string false "With Basic auth, 1 includes the leaderboards whatever the guild's premium. Ignored for members."
// @Success 200 {object} GuildStats
// @Failure 400 {object} HttpError
// @Failure 401 {object} HttpError
// @Failure 403 {object} HttpError
// @Failure 500 {object} HttpError
// @Router /guild/stats [get]
func handleGetGuildStats(stats, full *listCache[GuildStats]) func(c *gin.Context) {
	return func(c *gin.Context) {
		guildID := c.Query("guildID")
		if discord.ValidateSnowflake(guildID) != nil {
			c.JSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "invalid guild ID",
			})
			return
		}
		cache := stats
		// Full documents are for operators with the admin password, never for a member, and have their own
		// cache so one can never be served to the other.
		if c.Query("full") == "1" && !c.GetBool(memberRequestKey) {
			cache = full
		}
		result, err := cache.get(c.Request.Context(), guildID)
		if err != nil {
			log.Printf("Guild %s stats: %v\n", guildID, err)
			c.JSON(http.StatusInternalServerError, HttpError{
				StatusCode: http.StatusInternalServerError,
				Error:      "Unable to load guild statistics",
			})
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/capture"
	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/automuteus/automuteus/v8/pkg/game"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	pgstorage "github.com/automuteus/automuteus/v8/pkg/storage"
	"github.com/georgysavva/scany/pgxscan"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// errMatchNotFound means the guild has no match with the requested ID. It is not cached, so a match that starts
// recording moments later is found on the next request.
var errMatchNotFound = errors.New("match not found")

// MatchSummary is the GET /guild/match response: one match of a guild. The header and roster are always present;
// Timeline is present only while the guild's premium is active, as the /stats match slash command did
// before it was retired. Enumerations are stable lowercase keys so the page can translate them.
type MatchSummary struct {
	GuildID string `json:"guildId"`
	MatchID string `json:"matchId"`
	// Premium is the guild's premium status the summary was built under, so the page can explain a missing
	// timeline without a second request.
	Premium premium.PremiumRecord `json:"premium"`
	// Status is "finished", "inProgress", or "aborted" (ended by /end or the platform before the game reported a
	// result). Only finished matches count toward statistics.
	Status string `json:"status"`
	// StartTime and EndTime are Unix times. EndTime is absent while the match is in progress.
	StartTime int64 `json:"startTime"`
	EndTime   int64 `json:"endTime,omitempty"`
	// Result is how a finished match ended: crewmateVote, crewmateTasks, crewmateDisconnect, impostorVote,
	// impostorKill, impostorSabotage, impostorDisconnect, or unknown. Absent unless finished.
	Result string `json:"result,omitempty"`
	// Winner is "crewmate" or "impostor"; absent when there is no known winner.
	Winner string `json:"winner,omitempty"`
	// Map is skeld, mira, polus, dleks, airship, or fungle, and Region is na, eu, or as. Each is absent for
	// matches recorded before they were stored, or when the capture never reported a lobby.
	Map    string `json:"map,omitempty"`
	Region string `json:"region,omitempty"`
	// Roster lists the players of the match, impostors first. Linked players carry a user ID. When the capture's
	// game over report was kept (matches since it started being recorded), everyone else in the lobby is listed
	// too, by in-game name; otherwise only linked players are, and nobody is until the match ends.
	Roster []MatchPlayer `json:"roster"`
	// RosterComplete reports whether Roster came with the game over report and so names every player. When
	// false, unlinked and opted-out players are missing and the page should say so.
	RosterComplete bool `json:"rosterComplete"`
	// Timeline is omitted for guilds whose premium is free or expired.
	Timeline *MatchTimeline `json:"timeline,omitempty"`
	// Players maps every user ID in the roster to a name and picture, resolved as for GET /guild/stats.
	Players map[string]StatsPlayer `json:"players"`
}

type MatchPlayer struct {
	// UserID is set for players linked to a Discord user; absent for everyone else.
	UserID string `json:"userId,omitempty"`
	// Name is the in-game name the player used in this match.
	Name string `json:"name"`
	// Color is the in-game color key (red, blue, ... coral). The game over report carries no colors, so an
	// unlinked player has one only if a timeline event named them; otherwise it is empty.
	Color string `json:"color"`
	// Role is "crewmate" or "impostor".
	Role string `json:"role"`
	// Won is false for everyone when the result is unknown.
	Won bool `json:"won"`
}

// MatchTimeline is what happened during a match, from the capture's events. Counts are of the events listed.
type MatchTimeline struct {
	Meetings    int          `json:"meetings"`
	Deaths      int          `json:"deaths"`
	Exiles      int          `json:"exiles"`
	Disconnects int          `json:"disconnects"`
	Events      []MatchEvent `json:"events"`
}

type MatchEvent struct {
	// Offset is seconds since the match started.
	Offset int64 `json:"offset"`
	// Type is "tasks" or "discussion" for a phase beginning, or "death", "exile", or "disconnect" for a player.
	Type string `json:"type"`
	// Name and Color identify the player of a player event, as they appeared in game.
	Name  string `json:"name,omitempty"`
	Color string `json:"color,omitempty"`
	// UserID is set on a player event only when that player is on the roster, so an opted-out or reset player's
	// link is never revealed.
	UserID string `json:"userId,omitempty"`
}

var matchResults = map[game.GameResult]string{
	game.HumansByVote:       "crewmateVote",
	game.HumansByTask:       "crewmateTasks",
	game.HumansDisconnect:   "crewmateDisconnect",
	game.ImpostorByVote:     "impostorVote",
	game.ImpostorByKill:     "impostorKill",
	game.ImpostorBySabotage: "impostorSabotage",
	game.ImpostorDisconnect: "impostorDisconnect",
}

var matchMaps = map[game.PlayMap]string{
	game.SKELD:   "skeld",
	game.MIRA:    "mira",
	game.POLUS:   "polus",
	game.DLEKS:   "dleks",
	game.AIRSHIP: "airship",
	game.FUNGLE:  "fungle",
}

var matchRegions = map[game.Region]string{
	game.NA: "na",
	game.EU: "eu",
	game.AS: "as",
}

// MatchSummary builds the match summary document for one match of a guild, or returns errMatchNotFound.
func (s *DataStore) MatchSummary(ctx context.Context, guildID, matchID string) (MatchSummary, error) {
	record, err := s.Premium(ctx, guildID)
	if err != nil {
		return MatchSummary{}, fmt.Errorf("premium status: %w", err)
	}
	return buildMatchSummary(ctx, s.stats, s.redis, s.profiles, guildID, matchID, record)
}

func buildMatchSummary(ctx context.Context, db pgxscan.Querier, client *redis.Client, profiles ProfileFetcher, guildID, matchID string, record premium.PremiumRecord) (MatchSummary, error) {
	gid, err := strconv.ParseUint(guildID, 10, 64)
	if err != nil {
		return MatchSummary{}, fmt.Errorf("guild ID: %w", err)
	}
	mid, err := strconv.ParseInt(matchID, 10, 64)
	if err != nil {
		return MatchSummary{}, fmt.Errorf("match ID: %w", err)
	}
	g, err := pgstorage.GuildMatch(ctx, db, gid, mid)
	if err != nil {
		return MatchSummary{}, fmt.Errorf("match: %w", err)
	}
	if g == nil {
		return MatchSummary{}, errMatchNotFound
	}
	summary := MatchSummary{
		GuildID:   guildID,
		MatchID:   matchID,
		Premium:   record,
		StartTime: int64(g.StartTime),
		Roster:    []MatchPlayer{},
		Players:   map[string]StatsPlayer{},
	}
	result := game.GameResult(g.WinType)
	switch {
	case result == game.Aborted:
		summary.Status = "aborted"
		summary.EndTime = int64(g.EndTime)
	case g.EndTime < 0:
		summary.Status = "inProgress"
	default:
		summary.Status = "finished"
		summary.EndTime = int64(g.EndTime)
		summary.Result = "unknown"
		if key, ok := matchResults[result]; ok {
			summary.Result = key
			summary.Winner = "crewmate"
			if strings.HasPrefix(key, "impostor") {
				summary.Winner = "impostor"
			}
		}
	}
	if g.PlayMap != nil {
		summary.Map = matchMaps[game.PlayMap(*g.PlayMap)]
	}
	if g.Region != nil {
		summary.Region = matchRegions[game.Region(*g.Region)]
	}

	players, err := pgstorage.MatchPlayers(ctx, db, mid)
	if err != nil {
		return MatchSummary{}, fmt.Errorf("players: %w", err)
	}
	// Every guild reads the events: the game over report among them completes the roster, which is not premium.
	events, err := pgstorage.MatchEvents(ctx, db, mid)
	if err != nil {
		return MatchSummary{}, fmt.Errorf("events: %w", err)
	}
	summary.Roster, summary.RosterComplete = buildMatchRoster(players, events, summary.Winner)
	onRoster := make(map[string]bool, len(players))
	userIDs := make([]string, 0, len(players))
	for _, p := range summary.Roster {
		if p.UserID != "" {
			onRoster[p.UserID] = true
			userIDs = append(userIDs, p.UserID)
		}
	}
	if !premium.IsExpired(record.Tier, record.Days) {
		summary.Timeline = buildMatchTimeline(int64(g.StartTime), events, onRoster)
	}

	sort.Strings(userIDs)
	summary.Players = resolvePlayers(ctx, client, profiles, guildID, userIDs)
	return summary, nil
}

// buildMatchRoster lists the linked players recorded for the match and, when the capture's game over report was
// kept, adds everyone else it names. The recorded rows win for linked players: they were matched to the report
// by name when the match ended, so a report entry with a recorded player's name (in any case) is that player.
// A player who opted out or reset their statistics has no row and so appears unlinked, by in-game name only.
func buildMatchRoster(players []*pgstorage.PostgresUserGame, events []*pgstorage.PostgresGameEvent, winner string) ([]MatchPlayer, bool) {
	type entry struct {
		MatchPlayer
		color int
	}
	entries := make([]entry, 0, len(players))
	recorded := make(map[string]bool, len(players))
	for _, p := range players {
		role := "crewmate"
		if game.GameRole(p.PlayerRole) == game.ImposterRole {
			role = "impostor"
		}
		entries = append(entries, entry{MatchPlayer{
			UserID: snowflake(p.UserID),
			Name:   p.PlayerName,
			Color:  game.GetColorStringForInt(int(p.PlayerColor)),
			Role:   role,
			Won:    p.PlayerWon,
		}, int(p.PlayerColor)})
		recorded[strings.ToLower(p.PlayerName)] = true
	}

	var report *game.Gameover
	colors := map[string]int{}
	for _, e := range events {
		switch capture.EventType(e.EventType) {
		case capture.GameOver:
			var g game.Gameover
			if err := json.Unmarshal([]byte(e.Payload), &g); err == nil {
				report = &g
			}
		case capture.Player:
			var p game.Player
			if err := json.Unmarshal([]byte(e.Payload), &p); err == nil && p.Name != "" {
				colors[strings.ToLower(p.Name)] = p.Color
			}
		}
	}
	if report != nil {
		for _, info := range report.PlayerInfos {
			key := strings.ToLower(info.Name)
			if info.Name == "" || recorded[key] {
				continue
			}
			recorded[key] = true
			p := entry{MatchPlayer{Name: info.Name, Role: "crewmate"}, len(game.ColorStrings)}
			if info.IsImpostor {
				p.Role = "impostor"
			}
			p.Won = winner != "" && p.Role == winner
			if c, ok := colors[key]; ok {
				p.color = c
				p.Color = game.GetColorStringForInt(c)
			}
			entries = append(entries, p)
		}
	}

	// Impostors first, then in color order as the game lists players, with unknown colors last.
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.Role != b.Role {
			return a.Role == "impostor"
		}
		if a.color != b.color {
			return a.color < b.color
		}
		return a.Name < b.Name
	})
	roster := make([]MatchPlayer, 0, len(entries))
	for _, e := range entries {
		roster = append(roster, e.MatchPlayer)
	}
	return roster, report != nil
}

// buildMatchTimeline turns the capture's events into the page's timeline. A phase the capture reports again
// without changing is listed once, and a player who already died or was exiled is not listed dying again, so a
// capture that reconnects and resends its state does not inflate the counts. Player updates other than deaths,
// exiles, and disconnects (joins, color changes, forced refreshes) are left out.
func buildMatchTimeline(start int64, events []*pgstorage.PostgresGameEvent, onRoster map[string]bool) *MatchTimeline {
	timeline := &MatchTimeline{Events: []MatchEvent{}}
	lastPhase := game.UNINITIALIZED
	dead := map[int]bool{}
	for _, e := range events {
		offset := int64(e.EventTime) - start
		if offset < 0 {
			offset = 0
		}
		switch capture.EventType(e.EventType) {
		case capture.State:
			var phase game.Phase
			if err := json.Unmarshal([]byte(e.Payload), &phase); err != nil {
				continue
			}
			if phase == lastPhase {
				continue
			}
			lastPhase = phase
			switch phase {
			case game.TASKS:
				timeline.Events = append(timeline.Events, MatchEvent{Offset: offset, Type: "tasks"})
			case game.DISCUSS:
				timeline.Meetings++
				timeline.Events = append(timeline.Events, MatchEvent{Offset: offset, Type: "discussion"})
			}
		case capture.Player:
			var player game.Player
			if err := json.Unmarshal([]byte(e.Payload), &player); err != nil {
				continue
			}
			event := MatchEvent{Offset: offset, Name: player.Name, Color: game.GetColorStringForInt(player.Color)}
			if e.UserID != nil && onRoster[snowflake(*e.UserID)] {
				event.UserID = snowflake(*e.UserID)
			}
			switch player.Action {
			case game.DIED, game.EXILED:
				// Colors are unique within a lobby, so they identify the player even if two share a name.
				if dead[player.Color] {
					continue
				}
				dead[player.Color] = true
				if player.Action == game.DIED {
					event.Type = "death"
					timeline.Deaths++
				} else {
					event.Type = "exile"
					timeline.Exiles++
				}
			case game.DISCONNECTED:
				event.Type = "disconnect"
				timeline.Disconnects++
			default:
				continue
			}
			timeline.Events = append(timeline.Events, event)
		}
	}
	return timeline
}

// MatchSummary godoc
// @Summary Get Match Summary
// @Description One match of the guild: when it ran, how it ended, the map and region, and the linked players with
// @Description their roles and results; for matches whose game over report was kept, unlinked players too. Guilds with active premium also get the timeline
// @Description (phases, deaths, exiles, and disconnects, with seconds since the start). matchID is the
// @Description number after the colon in the Match ID the bot posts when a game ends. Responses may be up to a
// @Description minute old.
// @Security BasicAuth
// @Security DiscordBearer
// @Tags guild
// @Accept json
// @Produce json
// @Param guildID query string true "Guild ID"
// @Param matchID query string true "Match ID"
// @Success 200 {object} MatchSummary
// @Failure 400 {object} HttpError
// @Failure 401 {object} HttpError
// @Failure 403 {object} HttpError
// @Failure 404 {object} HttpError
// @Failure 500 {object} HttpError
// @Failure 503 {object} HttpError "The summary is still being built; retry after the Retry-After header"
// @Header 503 {string} Retry-After "Seconds to wait before retrying"
// @Router /guild/match [get]
func handleGetMatchSummary(matches *listCache[MatchSummary]) func(c *gin.Context) {
	return func(c *gin.Context) {
		guildID := c.Query("guildID")
		if discord.ValidateSnowflake(guildID) != nil {
			c.JSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "invalid guild ID",
			})
			return
		}
		matchID := c.Query("matchID")
		if id, err := strconv.ParseInt(matchID, 10, 64); err != nil || id <= 0 || strconv.FormatInt(id, 10) != matchID {
			c.JSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "invalid match ID",
			})
			return
		}
		result, err := matches.get(c.Request.Context(), matchCacheKey(guildID, matchID))
		if errors.Is(err, errMatchNotFound) {
			c.JSON(http.StatusNotFound, HttpError{
				StatusCode: http.StatusNotFound,
				Error:      "match not found",
			})
			return
		}
		if errors.Is(err, errStillBuilding) {
			respondStillBuilding(c, "match summaries")
			return
		}
		if err != nil {
			log.Printf("Guild %s match %s summary: %v\n", guildID, matchID, err)
			c.JSON(http.StatusInternalServerError, HttpError{
				StatusCode: http.StatusInternalServerError,
				Error:      "Unable to load match summary",
			})
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// Match summaries share the per-guild cache type, keyed by guild and match together.
func matchCacheKey(guildID, matchID string) string {
	return guildID + "/" + matchID
}

func newMatchCache(ttl time.Duration, build func(ctx context.Context, guildID, matchID string) (MatchSummary, error)) *listCache[MatchSummary] {
	return newListCache(ttl, nil, func(ctx context.Context, key string) (MatchSummary, error) {
		guildID, matchID, _ := strings.Cut(key, "/")
		return build(ctx, guildID, matchID)
	})
}

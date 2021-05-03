package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/automuteus/galactus/broker"
	"github.com/automuteus/utils/pkg/game"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

var DiscussCode = fmt.Sprintf("%d", game.DISCUSS)
var TasksCode = fmt.Sprintf("%d", game.TASKS)

type SimpleEventType int

const (
	Tasks SimpleEventType = iota
	Discuss
	PlayerDeath
	PlayerDisconnect
	PlayerExiled
)

type SimpleEvent struct {
	EventType       SimpleEventType
	EventTimeOffset time.Duration
	Data            string
}

type GameStatistics struct {
	GameStartTime   time.Time
	GameEndTime     time.Time
	GameDuration    time.Duration
	WinType         game.GameResult
	WinRole         game.GameRole
	WinPlayerNames  []string
	LosePlayerNames []string

	NumMeetings    int
	NumDeaths      int
	NumVotedOff    int
	NumDisconnects int
	Events         []SimpleEvent
}

func StatsFromGameAndEvents(pgame *PostgresGame, events []*PostgresGameEvent, users []*PostgresUserGame) GameStatistics {
	stats := GameStatistics{
		GameStartTime:   time.Time{},
		GameEndTime:     time.Time{},
		GameDuration:    0,
		WinType:         game.Unknown,
		WinRole:         game.CrewmateRole,
		WinPlayerNames:  []string{},
		LosePlayerNames: []string{},
		NumMeetings:     0,
		NumDeaths:       0,
		Events:          []SimpleEvent{},
	}

	if pgame != nil {
		stats.GameStartTime = time.Unix(int64(pgame.StartTime), 0)
		stats.GameEndTime = time.Unix(int64(pgame.EndTime), 0)
		stats.GameDuration = time.Second * time.Duration(pgame.EndTime-pgame.StartTime)
		stats.WinType = game.GameResult(pgame.WinType)
	}

	if stats.WinType == game.ImpostorDisconnect || stats.WinType == game.ImpostorBySabotage || stats.WinType == game.ImpostorByVote || stats.WinType == game.ImpostorByKill {
		stats.WinRole = game.ImposterRole
	}

	for _, v := range users {
		if v.PlayerWon {
			stats.WinPlayerNames = append(stats.WinPlayerNames, v.PlayerName)
		} else {
			stats.LosePlayerNames = append(stats.LosePlayerNames, v.PlayerName)
		}
	}

	if len(events) < 2 {
		return stats
	}
	exiledPlayerNames := []string{}
	for _, v := range events {
		if v.EventType == int16(broker.State) {
			if v.Payload == DiscussCode {
				stats.NumMeetings++
				stats.Events = append(stats.Events, SimpleEvent{
					EventType:       Discuss,
					EventTimeOffset: time.Second * time.Duration(v.EventTime-pgame.StartTime),
					Data:            "",
				})
			} else if v.Payload == TasksCode {
				stats.Events = append(stats.Events, SimpleEvent{
					EventType:       Tasks,
					EventTimeOffset: time.Second * time.Duration(v.EventTime-pgame.StartTime),
					Data:            "",
				})
			}
		} else if v.EventType == int16(broker.Player) {
			player := game.Player{}
			err := json.Unmarshal([]byte(v.Payload), &player)
			if err != nil {
				log.Println(err)
			} else {
				switch {
				case player.Action == game.DIED:
					stats.NumDeaths++
					isExiled := false
					for _, exiledPlayerName := range exiledPlayerNames {
						if exiledPlayerName == player.Name {
							isExiled = true
						}
					}
					if !isExiled {
						stats.Events = append(stats.Events, SimpleEvent{
							EventType:       PlayerDeath,
							EventTimeOffset: time.Second * time.Duration(v.EventTime-pgame.StartTime),
							Data:            v.Payload,
						})
					}
				case player.Action == game.EXILED:
					stats.NumVotedOff++
					exiledPlayerNames = append(exiledPlayerNames, player.Name)
					stats.Events = append(stats.Events, SimpleEvent{
						EventType:       PlayerExiled,
						EventTimeOffset: time.Second * time.Duration(v.EventTime-pgame.StartTime),
						Data:            v.Payload,
					})
				case player.Action == game.DISCONNECTED:
					stats.NumDisconnects++
					stats.Events = append(stats.Events, SimpleEvent{
						EventType:       PlayerDisconnect,
						EventTimeOffset: time.Second * time.Duration(v.EventTime-pgame.StartTime),
						Data:            v.Payload,
					})
				}
			}
		}
	}

	return stats
}

func (stats *GameStatistics) ToDiscordEmbed(combinedID string, sett *GuildSettings) *discordgo.MessageEmbed {

	title := sett.LocalizeMessage(&i18n.Message{
		ID:    "responses.matchStatsEmbed.Title",
		Other: "Game `{{.MatchID}}`",
	}, map[string]interface{}{
		"MatchID": combinedID,
	})

	fields := make([]*discordgo.MessageEmbedField, 0)

	winRoleStr, loseRoleStr := "", ""
	switch stats.WinRole {
	case game.CrewmateRole:
		winRoleStr = sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.matchStatsEmbed.Crewmates",
			Other: "Crewmates",
		})
		loseRoleStr = sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.matchStatsEmbed.Imposters",
			Other: "Imposters",
		})
	case game.ImposterRole:
		winRoleStr = sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.matchStatsEmbed.Imposters",
			Other: "Imposters",
		})
		loseRoleStr = sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.matchStatsEmbed.Crewmates",
			Other: "Crewmates",
		})
	}

	if len(stats.WinPlayerNames) > 0 {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("🏆 %s (%d)", winRoleStr, len(stats.WinPlayerNames)),
			Value:  fmt.Sprintf("%s", strings.Join(stats.WinPlayerNames, ", ")),
			Inline: false,
		})
	}
	if len(stats.LosePlayerNames) > 0 {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("🤢 %s (%d)", loseRoleStr, len(stats.LosePlayerNames)),
			Value:  fmt.Sprintf("%s", strings.Join(stats.LosePlayerNames, ", ")),
			Inline: false,
		})
	}

	fields = append(fields, &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.matchStatsEmbed.Start",
			Other: "🕑 Start",
		}),
		Value:  stats.GameStartTime.Add(time.Duration(sett.GetTimeOffset()*60) * time.Minute).Format("Jan 2, 3:04 PM"),
		Inline: true,
	})
	fields = append(fields, &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.matchStatsEmbed.End",
			Other: "🕑 End",
		}),
		Value:  stats.GameEndTime.Add(time.Duration(sett.GetTimeOffset()*60) * time.Minute).Format("3:04 PM"),
		Inline: true,
	})
	fields = append(fields, &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.matchStatsEmbed.Duration",
			Other: "⏲️ Duration",
		}),
		Value:  stats.GameDuration.String(),
		Inline: true,
	})
	fields = append(fields, &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.matchStatsEmbed.Players",
			Other: "🎮 Players",
		}),
		Value:  fmt.Sprintf("%d", len(stats.WinPlayerNames)+len(stats.LosePlayerNames)),
		Inline: true,
	})
	fields = append(fields, &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.matchStatsEmbed.Meetings",
			Other: "💬 Meetings",
		}),
		Value:  fmt.Sprintf("%d", stats.NumMeetings),
		Inline: true,
	})
	fields = append(fields, &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.matchStatsEmbed.Death",
			Other: "☠️ Death",
		}),
		Value: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.matchStatsEmbed.DeathDesc",
			Other: "{{.Death}} ({{.Killed}} killed, {{.VotedOff}} exiled)",
		}, map[string]interface{}{
			"Death":    stats.NumDeaths,
			"Killed":   stats.NumDeaths - stats.NumVotedOff,
			"VotedOff": stats.NumVotedOff,
		}),
		Inline: true,
	})

	buf := bytes.NewBuffer([]byte{})
	for _, v := range stats.Events {
		switch {
		case v.EventType == Tasks:
			buf.WriteString(fmt.Sprintf("`%s` %s\n", formatTimeDuration(v.EventTimeOffset), sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.matchStatsEmbed.TaskEvent",
				Other: "🔨 Task",
			})))
		case v.EventType == Discuss:
			buf.WriteString(fmt.Sprintf("`%s` %s\n", formatTimeDuration(v.EventTimeOffset), sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.matchStatsEmbed.DiscussionEvent",
				Other: "💬 Discussion",
			})))
		case v.EventType == PlayerDeath:
			player := game.Player{}
			err := json.Unmarshal([]byte(v.Data), &player)
			if err != nil {
				log.Println(err)
			} else {
				buf.WriteString(fmt.Sprintf("`%s` %s\n", formatTimeDuration(v.EventTimeOffset), sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.matchStatsEmbed.KilledEvent",
					Other: "🔪 **{{.PlayerName}}** Killed",
				}, map[string]interface{}{
					"PlayerName": player.Name,
				})))
			}
		case v.EventType == PlayerDisconnect:
			player := game.Player{}
			err := json.Unmarshal([]byte(v.Data), &player)
			if err != nil {
				log.Println(err)
			} else {
				buf.WriteString(fmt.Sprintf("`%s` %s\n", formatTimeDuration(v.EventTimeOffset), sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.matchStatsEmbed.DisconnectedEvent",
					Other: "🔌 **{{.PlayerName}}** Disconnected",
				}, map[string]interface{}{
					"PlayerName": player.Name,
				})))
			}
		case v.EventType == PlayerExiled:
			player := game.Player{}
			err := json.Unmarshal([]byte(v.Data), &player)
			if err != nil {
				log.Println(err)
			} else {
				buf.WriteString(fmt.Sprintf("`%s` %s\n", formatTimeDuration(v.EventTimeOffset), sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.matchStatsEmbed.ExiledEvent",
					Other: "⛔ **{{.PlayerName}}** Exiled",
				}, map[string]interface{}{
					"PlayerName": player.Name,
				})))
			}
		}
	}
	if len(stats.Events) > 0 {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name: sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.matchStatsEmbed.GameEvents",
				Other: "📋 Game Events",
			}),
			Value:  buf.String(),
			Inline: false,
		})
	}

	msg := discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       title,
		Description: stats.FormatGameStatsDescription(sett),
		Timestamp:   "",
		Color:       10181046, // PURPLE
		Footer:      nil,
		Image:       nil,
		Thumbnail:   nil,
		Video:       nil,
		Provider:    nil,
		Author:      nil,
		Fields:      fields,
	}
	return &msg
}

func (stats *GameStatistics) FormatGameStatsDescription(sett *GuildSettings) string {
	buf := bytes.NewBuffer([]byte{})
	switch stats.WinType {
	case game.HumansByTask:
		buf.WriteString(sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.matchStatsEmbed.HumansByTask",
			Other: "**Crewmates** won by **completing tasks** !",
		}))
	case game.HumansByVote:
		buf.WriteString(sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.matchStatsEmbed.HumansByVote",
			Other: "**Crewmates** won by **voting off the last Imposter** !",
		}))
	case game.HumansDisconnect:
		buf.WriteString(sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.matchStatsEmbed.HumansDisconnect",
			Other: "**Crewmates** won because **the last Imposter disconnected** !",
		}))
	case game.ImpostorDisconnect:
		buf.WriteString(sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.matchStatsEmbed.ImpostorDisconnect",
			Other: "**Imposters** won because **the last Crewmate disconnected** !",
		}))
	case game.ImpostorBySabotage:
		buf.WriteString(sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.matchStatsEmbed.ImpostorBySabotage",
			Other: "**Imposters** won by **sabotage** !",
		}))
	case game.ImpostorByVote:
		buf.WriteString(sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.matchStatsEmbed.ImpostorByVote",
			Other: "**Imposters** won by **voting off the last Crewmate** !",
		}))
	case game.ImpostorByKill:
		buf.WriteString(sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.matchStatsEmbed.ImpostorByKill",
			Other: "**Imposters** won by **killing the last Crewmate** !",
		}))
	}
	return buf.String()
}

func (stats *GameStatistics) ToString() string {
	buf := bytes.NewBuffer([]byte{})
	buf.WriteString(stats.FormatGameStatsDescription(MakeGuildSettings()))

	for _, v := range stats.Events {
		switch {
		case v.EventType == Tasks:
			buf.WriteString(fmt.Sprintf("%s into the game, Tasks phase resumed", v.EventTimeOffset.String()))
		case v.EventType == Discuss:
			buf.WriteString(fmt.Sprintf("%s into the game, Discussion was called", v.EventTimeOffset.String()))
		case v.EventType == PlayerDeath:
			player := game.Player{}
			err := json.Unmarshal([]byte(v.Data), &player)
			if err != nil {
				log.Println(err)
			} else {
				buf.WriteString(fmt.Sprintf("%s into the game, %s died", v.EventTimeOffset.String(), player.Name))
			}
		}
		buf.WriteRune('\n')
	}

	return buf.String()
}

func formatTimeDuration(d time.Duration) string {
	minute := d / time.Minute
	second := (d - minute*time.Minute) / time.Second
	return fmt.Sprintf("%02d:%02d", minute, second)
}

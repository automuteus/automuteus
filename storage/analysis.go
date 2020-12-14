package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/automuteus/galactus/broker"
	"github.com/automuteus/utils/pkg/game"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"log"
	"time"
)

var DiscussCode = fmt.Sprintf("%d", game.DISCUSS)
var TasksCode = fmt.Sprintf("%d", game.TASKS)

type SimpleEventType int

const (
	Tasks SimpleEventType = iota
	Discuss
	PlayerDeath
	PlayerDisconnect
)

type SimpleEvent struct {
	EventType       SimpleEventType
	EventTimeOffset time.Duration
	Data            string
}

type GameStatistics struct {
	GameDuration time.Duration
	WinType      game.GameResult

	NumMeetings    int
	NumDeaths      int
	NumVotedOff    int
	NumDisconnects int
	Events         []SimpleEvent
}

func StatsFromGameAndEvents(pgame *PostgresGame, events []*PostgresGameEvent) GameStatistics {
	stats := GameStatistics{
		GameDuration: time.Second * time.Duration(pgame.EndTime-pgame.StartTime),
		WinType:      game.GameResult(pgame.WinType),
		NumMeetings:  0,
		NumDeaths:    0,
		Events:       []SimpleEvent{},
	}

	if len(events) < 2 {
		return stats
	}

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
					stats.Events = append(stats.Events, SimpleEvent{
						EventType:       PlayerDeath,
						EventTimeOffset: time.Second * time.Duration(v.EventTime-pgame.StartTime),
						Data:            v.Payload,
					})
				case player.Action == game.EXILED:
					stats.NumVotedOff++
				case player.Action == game.DISCONNECTED:
					stats.NumDisconnects++
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

	fieldsOnLine := 0
	// TODO collapse by meeting/tasks "blocks" of data
	// TODO localize
	for _, v := range stats.Events {
		switch {
		case v.EventType == Tasks:
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   v.EventTimeOffset.String(),
				Value:  "🔨 Task Phase Begins",
				Inline: true,
			})
			fieldsOnLine++
		case v.EventType == Discuss:
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   v.EventTimeOffset.String(),
				Value:  "💬 Discussion Begins",
				Inline: true,
			})
			fieldsOnLine++
		case v.EventType == PlayerDeath:
			player := game.Player{}
			err := json.Unmarshal([]byte(v.Data), &player)
			if err != nil {
				log.Println(err)
			} else {
				fields = append(fields, &discordgo.MessageEmbedField{
					Name:   v.EventTimeOffset.String(),
					Value:  fmt.Sprintf("☠️ \"%s\" Died", player.Name),
					Inline: false,
				})
			}
			fieldsOnLine = 0
		}
		if fieldsOnLine == 2 {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "\u200B",
				Value:  "\u200B",
				Inline: true,
			})
		}
	}

	msg := discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       title,
		Description: stats.FormatDurationAndWin(),
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

// TODO localize
func (stats *GameStatistics) FormatDurationAndWin() string {
	buf := bytes.NewBuffer([]byte{})
	winner := ""
	switch stats.WinType {
	case game.HumansByTask:
		winner = "Crewmates won by completing tasks"
	case game.HumansByVote:
		winner = "Crewmates won by voting off the last Imposter"
	case game.HumansDisconnect:
		winner = "Crewmates won because the last Imposter disconnected"
	case game.ImpostorDisconnect:
		winner = "Imposters won because the last Human disconnected"
	case game.ImpostorBySabotage:
		winner = "Imposters won by sabotage"
	case game.ImpostorByVote:
		winner = "Imposters won by voting off the last Human"
	case game.ImpostorByKill:
		winner = "Imposters won by killing the last Human"
	}
	buf.WriteString("This display is VERY UNFINISHED and will be refined as time goes on!\n\n")

	buf.WriteString(fmt.Sprintf("Game lasted %s and %s\n", stats.GameDuration.String(), winner))
	buf.WriteString(fmt.Sprintf("There were %d meetings, %d deaths, and of those deaths, %d were from being voted off\n",
		stats.NumMeetings, stats.NumDeaths, stats.NumVotedOff))
	buf.WriteString("Game Events:\n")
	return buf.String()
}

func (stats *GameStatistics) ToString() string {
	buf := bytes.NewBuffer([]byte{})
	buf.WriteString(stats.FormatDurationAndWin())

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

package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/automuteus/galactus/broker"
	"github.com/denverquane/amongusdiscord/game"
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
	//PlayerDisconnect
)

type SimpleEvent struct {
	EventType       SimpleEventType
	EventTimeOffset time.Duration
	Data            string
}

type GameStatistics struct {
	GameDuration time.Duration
	WinType      game.GameResult

	NumMeetings int
	NumDeaths   int
	NumVotedOff int
	Events      []SimpleEvent
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
				if player.Action == game.DIED {
					stats.NumDeaths++
					uid := fmt.Sprintf("%d", v.UserID)
					if v.UserID == nil {
						uid = "<Unlinked Player>"
					}
					stats.Events = append(stats.Events, SimpleEvent{
						EventType:       PlayerDeath,
						EventTimeOffset: time.Second * time.Duration(v.EventTime-pgame.StartTime),
						Data:            uid,
					})
				} else if player.Action == game.EXILED {
					stats.NumVotedOff++
				}
			}
		}
	}

	return stats
}

func (stats *GameStatistics) ToString() string {
	buf := bytes.NewBuffer([]byte{})
	winner := ""
	switch stats.WinType {
	case game.HumansByTask:
		winner = "Humans won by completing tasks"
	case game.HumansByVote:
		winner = "Humans won by voting off the last Imposter"
	case game.HumansDisconnect:
		winner = "Humans won because the last Imposter disconnected"
	case game.ImpostorDisconnect:
		winner = "Imposters won because the last Human disconnected"
	case game.ImpostorBySabotage:
		winner = "Imposters won by sabotage"
	case game.ImpostorByVote:
		winner = "Imposters won by voting off the last Human"
	case game.ImpostorByKill:
		winner = "Imposters won by killing the last Human"
	}

	buf.WriteString(fmt.Sprintf("Game lasted %s and %s\n", stats.GameDuration.String(), winner))
	buf.WriteString(fmt.Sprintf("There were %d meetings, %d deaths, and of those deaths, %d were from being voted off\n",
		stats.NumMeetings, stats.NumDeaths, stats.NumVotedOff))
	buf.WriteString("Game Events:\n")

	for _, v := range stats.Events {
		if v.EventType == Tasks {
			buf.WriteString(fmt.Sprintf("%s into the game, Tasks phase resumed", v.EventTimeOffset.String()))
		} else if v.EventType == Discuss {
			buf.WriteString(fmt.Sprintf("%s into the game, Discussion was called", v.EventTimeOffset.String()))
		} else if v.EventType == PlayerDeath {
			buf.WriteString(fmt.Sprintf("%s into the game, %s died", v.EventTimeOffset.String(), v.Data))
		}
		buf.WriteRune('\n')
	}

	return buf.String()
}

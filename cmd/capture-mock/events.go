package main

import (
	"cmp"
	"slices"

	"github.com/automuteus/automuteus/v8/pkg/game"
)

type entryKind int

const (
	entrySent entryKind = iota
	entryNote
	entryGalactus
	entryInfo
	entryPass
	entryFail
	entryError
)

// logEntry is one line of the event log.
type logEntry struct {
	kind entryKind
	text string
}

// option is one choice in a manual event form, listed in value order.
type option[T any] struct {
	label string
	value T
}

func options[K cmp.Ordered, V ~string](names map[K]V) []option[K] {
	opts := make([]option[K], 0, len(names))
	for value, label := range names {
		opts = append(opts, option[K]{string(label), value})
	}
	slices.SortFunc(opts, func(a, b option[K]) int { return cmp.Compare(a.value, b.value) })
	return opts
}

var actionNames = map[game.PlayerAction]string{
	game.JOINED: "JOINED", game.LEFT: "LEFT", game.DIED: "DIED", game.CHANGECOLOR: "CHANGECOLOR",
	game.FORCEUPDATED: "FORCEUPDATED", game.DISCONNECTED: "DISCONNECTED", game.EXILED: "EXILED",
}

var resultNames = map[game.GameResult]string{
	game.HumansByVote: "HumansByVote", game.HumansByTask: "HumansByTask",
	game.ImpostorByVote: "ImpostorByVote", game.ImpostorByKill: "ImpostorByKill",
	game.ImpostorBySabotage: "ImpostorBySabotage", game.ImpostorDisconnect: "ImpostorDisconnect",
	game.HumansDisconnect: "HumansDisconnect", game.Unknown: "Unknown",
}

var (
	regionOptions = options(map[game.Region]string{
		game.NA: game.NA.ToString(), game.AS: game.AS.ToString(), game.EU: game.EU.ToString(),
	})
	mapOptions    = options(game.MapNames)
	phaseOptions  = options(game.PhaseNames)
	actionOptions = options(actionNames)
	resultOptions = options(resultNames)
	colorOptions  = func() []option[int] {
		names := make(map[int]string, len(game.ColorStrings))
		for label, value := range game.ColorStrings {
			names[value] = label
		}
		return options(names)
	}()
)

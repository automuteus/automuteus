package game

type PlayMap int

const (
	SKELD PlayMap = iota
	MIRA
	POLUS
	DLEKS // Skeld backwards
	AIRSHIP
	EMPTYMAP PlayMap = 10
)

var MapNames = map[PlayMap]string{
	SKELD:   "Skeld",
	MIRA:    "Mira",
	POLUS:   "Polus",
	DLEKS:   "dlekS",
	AIRSHIP: "Airship",
}

var NameToPlayMap = map[string]int32{
	"the_skeld": (int32)(SKELD),
	"mira_hq":   (int32)(MIRA),
	"polus":     (int32)(POLUS),
	"dleks":     (int32)(DLEKS),
	"airship":   (int32)(AIRSHIP),
	"NoMap":     -1,
}

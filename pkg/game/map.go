package game

import "fmt"

type PlayMap int

const DefaultMapsUrl = "https://github.com/automuteus/automuteus/blob/master/assets/maps/"

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

var nameToPlayMap = map[string]int32{
	"the_skeld": (int32)(SKELD),
	"mira_hq":   (int32)(MIRA),
	"polus":     (int32)(POLUS),
	"dleks":     (int32)(DLEKS),
	"airship":   (int32)(AIRSHIP),
	"NoMap":     -1,
}

func FormMapUrl(baseUrl string, mapType PlayMap, detailed bool) string {
	if mapType == EMPTYMAP {
		return ""
	}
	if baseUrl == "" {
		baseUrl = DefaultMapsUrl
	}

	mapString := ""
	for i, v := range nameToPlayMap {
		if v == int32(mapType) {
			mapString = i
			break
		}
	}
	if mapString == "" {
		return ""
	}
	// only have the simple variant of dleks
	if detailed && mapType != DLEKS {
		return fmt.Sprintf("%s%s_detailed.png?raw=true", baseUrl, mapString)
	}
	return fmt.Sprintf("%s%s.png?raw=true", baseUrl, mapString)
}

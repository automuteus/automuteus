package discord

// Color : Int constant mapping
const (
	Red    = 0
	Blue   = 1
	Green  = 2
	Pink   = 3
	Orange = 4
	Yellow = 5
	Black  = 6
	White  = 7
	Purple = 8
	Brown  = 9
	Cyan   = 10
	Lime   = 11
)

// ColorStrings for lowercase, possibly for translation if needed
var ColorStrings = map[string]int{
	"red":    Red,
	"blue":   Blue,
	"green":  Green,
	"pink":   Pink,
	"orange": Orange,
	"yellow": Yellow,
	"black":  Black,
	"white":  White,
	"purple": Purple,
	"brown":  Brown,
	"cyan":   Cyan,
	"lime":   Lime,
}

// GetColorStringForInt does what it sounds like
func GetColorStringForInt(colorint int) string {
	for str, idx := range ColorStrings {
		if idx == colorint {
			return str
		}
	}
	return ""
}

// Emoji struct for discord
type Emoji struct {
	Name string
	ID   string
}

// AlivenessColoredEmojis keys are IsAlive, Color
var AlivenessColoredEmojis = map[bool]map[int]Emoji{
	true: map[int]Emoji{
		Red: {
			Name: "aured",
			ID:   "756202732301320325",
		},
		Blue: {
			Name: "aublue",
			ID:   "756201148154642642",
		},
		Green: {
			Name: "augreen",
			ID:   "756202732099993753",
		},
		Pink: {
			Name: "aupink",
			ID:   "756200620049956864",
		},
		Orange: {
			Name: "auorange",
			ID:   "756202732523618435",
		},
		Yellow: {
			Name: "auyellow",
			ID:   "756202732678938624",
		},
		Black: {
			Name: "aublack",
			ID:   "756202732758761522",
		},
		White: {
			Name: "auwhite",
			ID:   "756202732343394386",
		},
		Purple: {
			Name: "aupurple",
			ID:   "756202732624543770",
		},
		Brown: {
			Name: "aubrown",
			ID:   "756202732594921482",
		},
		Cyan: {
			Name: "aucyan",
			ID:   "756202732511297556",
		},
		Lime: {
			Name: "aulime",
			ID:   "756202732360040569",
		},
	},
	false: map[int]Emoji{
		Red: {
			Name: "audeadred",
			ID:   "756404218163888200",
		},
		Blue: {
			Name: "audeadblue",
			ID:   "756404218163888200",
		},
		Green: {
			Name: "audeadgreen",
			ID:   "756404218163888200",
		},
		Pink: {
			Name: "audeadpink",
			ID:   "756404218163888200",
		},
		Orange: {
			Name: "audeadorange",
			ID:   "756404218436517888",
		},
		Yellow: {
			Name: "audeadyellow",
			ID:   "756404218339786762",
		},
		Black: {
			Name: "audeadblack",
			ID:   "756404218339786762",
		},
		White: {
			Name: "audeadwhite",
			ID:   "756404218339786762",
		},
		Purple: {
			Name: "audeadpurple",
			ID:   "756404218339786762",
		},
		Brown: {
			Name: "audeadbrown",
			ID:   "756404218339786762",
		},
		Cyan: {
			Name: "audeadcyan",
			ID:   "756204054698262559",
		},
		Lime: {
			Name: "audeadlime",
			ID:   "756204054698262559",
		},
	},
}

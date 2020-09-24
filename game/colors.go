package game

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

// IsColorString determines if a string is actually one of our colors
func IsColorString(test string) bool {
	_, ok := ColorStrings[test]
	return ok
}

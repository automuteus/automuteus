package game

// Color : Int constant mapping
const (
	Red       = 0
	Blue      = 1
	Green     = 2
	Pink      = 3
	Orange    = 4
	Yellow    = 5
	Black     = 6
	White     = 7
	Purple    = 8
	Brown     = 9
	Cyan      = 10
	Lime      = 11
	Skincolor = 12
	Bordeaux  = 13
	Olive     = 14
	Turqoise  = 15
	Mint      = 16
	Lavender  = 17
	Nougat    = 18
	Peach     = 19
	Neongreen = 20
	Hotpink   = 21
	Gray      = 22
	Petrol    = 23
)

// ColorStrings for lowercase, possibly for translation if needed
var ColorStrings = map[string]int{
	"red":       Red,
	"blue":      Blue,
	"green":     Green,
	"pink":      Pink,
	"orange":    Orange,
	"yellow":    Yellow,
	"black":     Black,
	"white":     White,
	"purple":    Purple,
	"brown":     Brown,
	"cyan":      Cyan,
	"lime":      Lime,
	"skincolor": Skincolor,
	"bordeaux":  Bordeaux,
	"olive":     Olive,
	"turqoise":  Turqoise,
	"mint":      Mint,
	"lavender":  Lavender,
	"nougat":    Nougat,
	"peach":     Peach,
	"neongreen": Neongreen,
	"hotpink":   Hotpink,
	"gray":      Gray,
	"petrol":    Petrol,
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

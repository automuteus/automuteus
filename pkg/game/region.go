package game

type Region int

const (
	NA Region = iota
	AS
	EU
)

func (r Region) ToString() string {
	switch r {
	case NA:
		return "North America"
	case EU:
		return "Europe"
	case AS:
		return "Asia"
	}
	return "Unknown"
}

// RegionFromString reverses ToString. It reports false for "Unknown" and any other unrecognized name.
func RegionFromString(s string) (Region, bool) {
	for _, r := range []Region{NA, AS, EU} {
		if r.ToString() == s {
			return r, true
		}
	}
	return 0, false
}

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

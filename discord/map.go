package discord

import (
	"fmt"
	"strings"
	"errors"
)

type MapType struct {
    Name string
}

func (m *MapType) String() string {
    return m.Name
}

func NewMapFromName(name string) (*MapType, error) {
    switch strings.ToLower(name) {
	case "the_skeld", "skeld":
		name = "skeld"
	case "mira_hq", "mira hq", "mirahq":
		name = "mira_hq"
	case "polus":
		name = "polus"
	default:
		return nil, errors.New(fmt.Sprintf("Invalid map name: %s", name))
    }
	return &MapType{Name: name}, nil
}


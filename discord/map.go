package discord

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
)

type MapItem struct {
	Name     string
	MapImage MapImage
}

type MapImage struct {
	Simple   string
	Detailed string
}

func (m *MapItem) String() string {
	return m.Name
}

const BaseMapURL = "https://github.com/denverquane/automuteus/blob/master/assets/maps/"

func NewMapItem(name string) (*MapItem, error) {
	switch strings.ToLower(name) {
	case "the skeld", "the_skeld", "skeld":
		name = "the_skeld"
	case "mira", "mira_hq", "mira hq", "mirahq":
		name = "mira_hq"
	case "polus":
		name = "polus"
	default:
		return nil, errors.New(fmt.Sprintf("Invalid map name: %s", name))
	}

	base, err := url.Parse(BaseMapURL)
	if err != nil {
		log.Println(err)
	}

	simpleURL, err := base.Parse(name + ".png?raw=true")
	if err != nil {
		log.Println(err)
	}

	detailedURL, err := base.Parse(name + "_detailed.png?raw=true")
	if err != nil {
		log.Println(err)
	}

	mapImage := MapImage{
		Simple:   simpleURL.String(),
		Detailed: detailedURL.String(),
	}

	return &MapItem{Name: name, MapImage: mapImage}, nil
}

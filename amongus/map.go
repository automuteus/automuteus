package amongus

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
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

func NewMapItem(name string) (*MapItem, error) {
	switch strings.ToLower(name) {
	case "the skeld", "the_skeld", "skeld":
		name = "the_skeld"
	case "mira", "mira_hq", "mira hq", "mirahq":
		name = "mira_hq"
	case "polus":
		name = "polus"
	//case "dleks":
	//	name = "dleks"
	case "airship", "ship", "air":
		name = "airship"
	default:
		return nil, errors.New(fmt.Sprintf("Invalid map name: %s", name))
	}

	BaseMapURL := os.Getenv("BASE_MAP_URL")
	if BaseMapURL == "" {
		BaseMapURL = "https://github.com/denverquane/automuteus/blob/master/assets/maps/"
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

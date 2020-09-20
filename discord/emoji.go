package discord

import (
	"encoding/base64"
	"io/ioutil"
	"log"
	"net/http"
)

// Emoji struct for discord
type Emoji struct {
	Name string
	ID   string
}

func (e *Emoji) FormatForReaction() string {
	return "<:" + e.Name + ":" + e.ID
}

func (e *Emoji) FormatForInline() string {
	return "<:" + e.Name + ":" + e.ID + ">"
}

func (e *Emoji) GetDiscordCDNUrl() string {
	return "https://cdn.discordapp.com/emojis/" + e.ID + ".png"
}

func (e *Emoji) DownloadAndBase64Encode() string {
	url := e.GetDiscordCDNUrl()
	response, err := http.Get(url)
	if err != nil {
		log.Println(err)
	}
	defer response.Body.Close()
	bytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Println(err)
	}
	encodedStr := base64.StdEncoding.EncodeToString(bytes)
	return "data:image/png;base64," + encodedStr
}

var AlarmEmoji = Emoji{
	Name: "aualarm",
	ID:   "756595863048159323",
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
			Name: "aureddead",
			ID:   "756404218163888200",
		},
		Blue: {
			Name: "aubluedead",
			ID:   "756552864309969057",
		},
		Green: {
			Name: "augreendead",
			ID:   "756552867275604008",
		},
		Pink: {
			Name: "aupinkdead",
			ID:   "756552867413753906",
		},
		Orange: {
			Name: "auorangedead",
			ID:   "756404218436517888",
		},
		Yellow: {
			Name: "auyellowdead",
			ID:   "756404218339786762",
		},
		Black: {
			Name: "aublackdead",
			ID:   "756552864171557035",
		},
		White: {
			Name: "auwhitedead",
			ID:   "756552867200106596",
		},
		Purple: {
			Name: "aupurpledead",
			ID:   "756552866491138159",
		},
		Brown: {
			Name: "aubrowndead",
			ID:   "756552864620347422",
		},
		Cyan: {
			Name: "aucyandead",
			ID:   "756204054698262559",
		},
		Lime: {
			Name: "aulimedead",
			ID:   "756552864847102042",
		},
	},
}

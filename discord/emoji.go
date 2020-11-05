package discord

import (
	"encoding/base64"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/denverquane/amongusdiscord/game"

	"github.com/bwmarrin/discordgo"
)

// Emoji struct for discord
type Emoji struct {
	Name string
	ID   string
}

// FormatForReaction does what it sounds like
func (e *Emoji) FormatForReaction() string {
	return "<:" + e.Name + ":" + e.ID
}

// FormatForInline does what it sounds like
func (e *Emoji) FormatForInline() string {
	return "<:" + e.Name + ":" + e.ID + ">"
}

// GetDiscordCDNUrl does what it sounds like
func (e *Emoji) GetDiscordCDNUrl() string {
	return "https://cdn.discordapp.com/emojis/" + e.ID + ".png"
}

// DownloadAndBase64Encode does what it sounds like
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

func emptyStatusEmojis() AlivenessEmojis {
	topMap := make(AlivenessEmojis)
	topMap[true] = make([]Emoji, 12) //12 colors for alive/dead
	topMap[false] = make([]Emoji, 12)
	return topMap
}

func (bot *Bot) addAllMissingEmojis(s *discordgo.Session, guildID string, alive bool, serverEmojis []*discordgo.Emoji) {
	for i, emoji := range GlobalAlivenessEmojis[alive] {
		alreadyExists := false
		for _, v := range serverEmojis {
			if v.Name == emoji.Name {
				emoji.ID = v.ID
				bot.StatusEmojis[alive][i] = emoji
				alreadyExists = true
				break
			}
		}
		if !alreadyExists {
			b64 := emoji.DownloadAndBase64Encode()
			em, err := s.GuildEmojiCreate(guildID, emoji.Name, b64, nil)
			if err != nil {
				log.Println(err)
			} else {
				log.Printf("Added emoji %s successfully!\n", emoji.Name)
				emoji.ID = em.ID
				bot.StatusEmojis[alive][i] = emoji
			}
		}
	}
}

// AlivenessEmojis map
type AlivenessEmojis map[bool][]Emoji

// GlobalAlivenessEmojis keys are IsAlive, Color
var GlobalAlivenessEmojis = AlivenessEmojis{
	true: []Emoji{
		game.Red: {
			Name: "aured",
			ID:   "762392085768175646",
		},
		game.Blue: {
			Name: "aublue",
			ID:   "762392085629632512",
		},
		game.Green: {
			Name: "augreen",
			ID:   "762392085889417226",
		},
		game.Pink: {
			Name: "aupink",
			ID:   "762392085726363648",
		},
		game.Orange: {
			Name: "auorange",
			ID:   "762392085264728095",
		},
		game.Yellow: {
			Name: "auyellow",
			ID:   "762392085541158923",
		},
		game.Black: {
			Name: "aublack",
			ID:   "762392086493790249",
		},
		game.White: {
			Name: "auwhite",
			ID:   "762392085990866974",
		},
		game.Purple: {
			Name: "aupurple",
			ID:   "762392085973303376",
		},
		game.Brown: {
			Name: "aubrown",
			ID:   "762392086023634986",
		},
		game.Cyan: {
			Name: "aucyan",
			ID:   "762392087945281557",
		},
		game.Lime: {
			Name: "aulime",
			ID:   "762392088121442334",
		},
	},
	false: []Emoji{
		game.Red: {
			Name: "aureddead",
			ID:   "762397192362393640",
		},
		game.Blue: {
			Name: "aubluedead",
			ID:   "762397192349679616",
		},
		game.Green: {
			Name: "augreendead",
			ID:   "762397192060272724",
		},
		game.Pink: {
			Name: "aupinkdead",
			ID:   "762397192643805194",
		},
		game.Orange: {
			Name: "auorangedead",
			ID:   "762397192333819904",
		},
		game.Yellow: {
			Name: "auyellowdead",
			ID:   "762397192425046016",
		},
		game.Black: {
			Name: "aublackdead",
			ID:   "762397192291090462",
		},
		game.White: {
			Name: "auwhitedead",
			ID:   "762397192409186344",
		},
		game.Purple: {
			Name: "aupurpledead",
			ID:   "762397192404860958",
		},
		game.Brown: {
			Name: "aubrowndead",
			ID:   "762397192102739989",
		},
		game.Cyan: {
			Name: "aucyandead",
			ID:   "762397192307867698",
		},
		game.Lime: {
			Name: "aulimedead",
			ID:   "762397192366325793",
		},
	},
}

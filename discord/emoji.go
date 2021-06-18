package discord

import (
	"encoding/base64"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/automuteus/utils/pkg/game"

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
	topMap[true] = make([]Emoji, 18) // 18 colors for alive/dead
	topMap[false] = make([]Emoji, 18)
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
			ID:   "855577595138408458",
		},
		game.Blue: {
			Name: "aublue",
			ID:   "855577594698399744",
		},
		game.Green: {
			Name: "augreen",
			ID:   "855577594937081886",
		},
		game.Pink: {
			Name: "aupink",
			ID:   "855577594719895622",
		},
		game.Orange: {
			Name: "auorange",
			ID:   "855577595131068416",
		},
		game.Yellow: {
			Name: "auyellow",
			ID:   "855577594560905237",
		},
		game.Black: {
			Name: "aublack",
			ID:   "855577594803781632",
		},
		game.White: {
			Name: "auwhite",
			ID:   "855577594577420339",
		},
		game.Purple: {
			Name: "aupurple",
			ID:   "855577594937868289",
		},
		game.Brown: {
			Name: "aubrown",
			ID:   "855577594657243136",
		},
		game.Cyan: {
			Name: "aucyan",
			ID:   "855577594958446602",
		},
		game.Lime: {
			Name: "aulime",
			ID:   "855577594949926912",
		},
		game.Maroon: {
			Name: "aumaroon",
			ID:   "855577594966573067",
		},
		game.Rose: {
			Name: "aurose",
			ID:   "855577594739949579",
		},
		game.Banana: {
			Name: "aubanana",
			ID:   "855577594535477279",
		},
		game.Gray: {
			Name: "augray",
			ID:   "855577594706919485",
		},
		game.Tan: {
			Name: "autan",
			ID:   "855577594723565568",
		},
		game.Coral: {
			Name: "aucoral",
			ID:   "855577594597998633",
		},
	},
	false: []Emoji{
		game.Red: {
			Name: "aureddead",
			ID:   "855577594548191253",
		},
		game.Blue: {
			Name: "aubluedead",
			ID:   "855577595391115364",
		},
		game.Green: {
			Name: "augreendead",
			ID:   "855577594690535453",
		},
		game.Pink: {
			Name: "aupinkdead",
			ID:   "855577594757382194",
		},
		game.Orange: {
			Name: "auorangedead",
			ID:   "855577594879148032",
		},
		game.Yellow: {
			Name: "auyellowdead",
			ID:   "855577594971947008",
		},
		game.Black: {
			Name: "aublackdead",
			ID:   "855577595496497172",
		},
		game.White: {
			Name: "auwhitedead",
			ID:   "855577595218755614",
		},
		game.Purple: {
			Name: "aupurpledead",
			ID:   "855577594984005672",
		},
		game.Brown: {
			Name: "aubrowndead",
			ID:   "855577594609795082",
		},
		game.Cyan: {
			Name: "aucyandead",
			ID:   "855577594807713792",
		},
		game.Lime: {
			Name: "aulimedead",
			ID:   "855577595834925066",
		},
		game.Maroon: {
			Name: "aumaroondead",
			ID:   "855577594699448331",
		},
		game.Rose: {
			Name: "aurosedead",
			ID:   "855577595232256011",
		},
		game.Banana: {
			Name: "aubananadead",
			ID:   "855577594312130592",
		},
		game.Gray: {
			Name: "augraydead",
			ID:   "855577594903527424",
		},
		game.Tan: {
			Name: "autandead",
			ID:   "855577595222163476",
		},
		game.Coral: {
			Name: "aucoraldead",
			ID:   "855577595563212860",
		},
	},
}

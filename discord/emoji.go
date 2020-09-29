package discord

import (
	"encoding/base64"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
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
	return "https://cdn.discordapp.com/emojis/" + e.ID + ".gif"
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
	return "data:image/gif;base64," + encodedStr
}

func emptyStatusEmojis() AlivenessEmojis {
	topMap := make(AlivenessEmojis)
	topMap[true] = make([]Emoji, 12) //12 colors for alive/dead
	topMap[false] = make([]Emoji, 12)
	return topMap
}

func (guild *GuildState) addSpecialEmojis(s *discordgo.Session, guildID string, serverEmojis []*discordgo.Emoji) {
	for _, emoji := range GlobalSpecialEmojis {
		alreadyExists := false
		for _, v := range serverEmojis {
			if v.Name == emoji.Name {
				emoji.ID = v.ID
				guild.SpecialEmojis[v.Name] = emoji
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
				guild.SpecialEmojis[em.Name] = emoji
			}
		}
	}
}

func (guild *GuildState) addAllMissingEmojis(s *discordgo.Session, guildID string, alive bool, serverEmojis []*discordgo.Emoji) {
	for i, emoji := range GlobalAlivenessEmojis[alive] {
		alreadyExists := false
		for _, v := range serverEmojis {
			if v.Name == emoji.Name {
				emoji.ID = v.ID
				guild.StatusEmojis[alive][i] = emoji
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
				guild.StatusEmojis[alive][i] = emoji
			}
		}
	}
}

// GlobalSpecialEmojis map
var GlobalSpecialEmojis = map[string]Emoji{
	"alarm": {
		Name: "aualarm",
		ID:   "760601471221235722",
	},
}

// AlivenessEmojis map
type AlivenessEmojis map[bool][]Emoji

// GlobalAlivenessEmojis keys are IsAlive, Color
var GlobalAlivenessEmojis = AlivenessEmojis{
	true: []Emoji{
		game.Red: {
			Name: "aured",
			ID:   "760600880650649600",
		},
		game.Blue: {
			Name: "aublue",
			ID:   "760600903895089182",
		},
		game.Green: {
			Name: "augreen",
			ID:   "760600905128083496",
		},
		game.Pink: {
			Name: "aupink",
			ID:   "760600906126721045",
		},
		game.Orange: {
			Name: "auorange",
			ID:   "760600907040686100",
		},
		game.Yellow: {
			Name: "auyellow",
			ID:   "760600907913101333",
		},
		game.Black: {
			Name: "aublack",
			ID:   "760600908974260234",
		},
		game.White: {
			Name: "auwhite",
			ID:   "760600909427376159",
		},
		game.Purple: {
			Name: "aupurple",
			ID:   "760600910685929482",
		},
		game.Brown: {
			Name: "aubrown",
			ID:   "760600911675654184",
		},
		game.Cyan: {
			Name: "aucyan",
			ID:   "760600912514646026",
		},
	},
	false: []Emoji{
		game.Red: {
			Name: "aureddead",
			ID:   "760601090701393967",
		},
		game.Blue: {
			Name: "aubluedead",
			ID:   "760601091729129543",
		},
		game.Green: {
			Name: "augreendead",
			ID:   "760601093260050432",
		},
		game.Pink: {
			Name: "aupinkdead",
			ID:   "760601094630932520",
		},
		game.Orange: {
			Name: "auorangedead",
			ID:   "760601095507673108",
		},
		game.Yellow: {
			Name: "auyellowdead",
			ID:   "760601096271429632",
		},
		game.Black: {
			Name: "aublackdead",
			ID:   "760601097126543431",
		},
		game.White: {
			Name: "auwhitedead",
			ID:   "760601097546760224",
		},
		game.Purple: {
			Name: "aupurpledead",
			ID:   "760601098473701428",
		},
		game.Brown: {
			Name: "aubrowndead",
			ID:   "760601099660689428",
		},
		game.Cyan: {
			Name: "aucyandead",
			ID:   "760601100737839105",
		},
		game.Lime: {
			Name: "aulimedead",
			ID:   "760601101773963274",
		},
	},
}

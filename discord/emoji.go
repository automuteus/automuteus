package discord

import (
	"encoding/base64"
	"github.com/denverquane/amongusdiscord/game"
	"io/ioutil"
	"log"
	"net/http"

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
		ID:   "756595863048159323",
	},
}

// AlivenessEmojis map
type AlivenessEmojis map[bool][]Emoji

// GlobalAlivenessEmojis keys are IsAlive, Color
var GlobalAlivenessEmojis = AlivenessEmojis{
	true: []Emoji{
		game.Red: {
			Name: "aured",
			ID:   "756202732301320325",
		},
		game.Blue: {
			Name: "aublue",
			ID:   "756201148154642642",
		},
		game.Green: {
			Name: "augreen",
			ID:   "756202732099993753",
		},
		game.Pink: {
			Name: "aupink",
			ID:   "756200620049956864",
		},
		game.Orange: {
			Name: "auorange",
			ID:   "756202732523618435",
		},
		game.Yellow: {
			Name: "auyellow",
			ID:   "756202732678938624",
		},
		game.Black: {
			Name: "aublack",
			ID:   "756202732758761522",
		},
		game.White: {
			Name: "auwhite",
			ID:   "756202732343394386",
		},
		game.Purple: {
			Name: "aupurple",
			ID:   "756202732624543770",
		},
		game.Brown: {
			Name: "aubrown",
			ID:   "756202732594921482",
		},
		game.Cyan: {
			Name: "aucyan",
			ID:   "756202732511297556",
		},
		game.Lime: {
			Name: "aulime",
			ID:   "756202732360040569",
		},
	},
	false: []Emoji{
		game.Red: {
			Name: "aureddead",
			ID:   "756404218163888200",
		},
		game.Blue: {
			Name: "aubluedead",
			ID:   "756552864309969057",
		},
		game.Green: {
			Name: "augreendead",
			ID:   "756552867275604008",
		},
		game.Pink: {
			Name: "aupinkdead",
			ID:   "756552867413753906",
		},
		game.Orange: {
			Name: "auorangedead",
			ID:   "756404218436517888",
		},
		game.Yellow: {
			Name: "auyellowdead",
			ID:   "756404218339786762",
		},
		game.Black: {
			Name: "aublackdead",
			ID:   "756552864171557035",
		},
		game.White: {
			Name: "auwhitedead",
			ID:   "756552867200106596",
		},
		game.Purple: {
			Name: "aupurpledead",
			ID:   "756552866491138159",
		},
		game.Brown: {
			Name: "aubrowndead",
			ID:   "756552864620347422",
		},
		game.Cyan: {
			Name: "aucyandead",
			ID:   "756204054698262559",
		},
		game.Lime: {
			Name: "aulimedead",
			ID:   "756552864847102042",
		},
	},
}

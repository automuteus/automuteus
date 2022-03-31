package discord

import (
	"encoding/base64"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/automuteus/utils/pkg/game"

	"github.com/bwmarrin/discordgo"
)

const (
	UnlinkEmojiName = "auunlink"
	X               = "❌"
	ThumbsUp        = "👍"
)

// Emoji struct for discord
type Emoji struct {
	Name string
	ID   string
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

func EmojisToSelectMenuOptions(emojis []Emoji, unlinkEmoji string) (arr []discordgo.SelectMenuOption) {
	for i, v := range emojis {
		arr = append(arr, v.toSelectMenuOption(game.GetColorStringForInt(i)))
	}
	arr = append(arr, discordgo.SelectMenuOption{
		Label:   "unlink",
		Value:   UnlinkEmojiName,
		Emoji:   discordgo.ComponentEmoji{Name: unlinkEmoji},
		Default: false,
	})
	return arr
}

func (e Emoji) toSelectMenuOption(displayName string) discordgo.SelectMenuOption {
	return discordgo.SelectMenuOption{
		Label:   displayName,
		Value:   displayName, // use the Name for listen events later
		Emoji:   discordgo.ComponentEmoji{ID: e.ID},
		Default: false,
	}
}

// AlivenessEmojis map
type AlivenessEmojis map[bool][]Emoji

// GlobalAlivenessEmojis keys are IsAlive, Color
var GlobalAlivenessEmojis = AlivenessEmojis{
	true: []Emoji{
		game.Red: {
			Name: "aured",
			ID:   "866558066921177108",
		},
		game.Blue: {
			Name: "aublue",
			ID:   "866558066484183060",
		},
		game.Green: {
			Name: "augreen",
			ID:   "866558066568986664",
		},
		game.Pink: {
			Name: "aupink",
			ID:   "866558067004538891",
		},
		game.Orange: {
			Name: "auorange",
			ID:   "866558066902958090",
		},
		game.Yellow: {
			Name: "auyellow",
			ID:   "866558067243221002",
		},
		game.Black: {
			Name: "aublack",
			ID:   "866558066442895370",
		},
		game.White: {
			Name: "auwhite",
			ID:   "866558067026165770",
		},
		game.Purple: {
			Name: "aupurple",
			ID:   "866558066966396928",
		},
		game.Brown: {
			Name: "aubrown",
			ID:   "866558066564136970",
		},
		game.Cyan: {
			Name: "aucyan",
			ID:   "866558066525601853",
		},
		game.Lime: {
			Name: "aulime",
			ID:   "866558066963382282",
		},
		game.Maroon: {
			Name: "aumaroon",
			ID:   "866558066917113886",
		},
		game.Rose: {
			Name: "aurose",
			ID:   "866558066921439242",
		},
		game.Banana: {
			Name: "aubanana",
			ID:   "866558065917558797",
		},
		game.Gray: {
			Name: "augray",
			ID:   "866558066174459905",
		},
		game.Tan: {
			Name: "autan",
			ID:   "866558066820382721",
		},
		game.Coral: {
			Name: "aucoral",
			ID:   "866558066552209448",
		},
	},
	false: []Emoji{
		game.Red: {
			Name: "aureddead",
			ID:   "866558067255279636",
		},
		game.Blue: {
			Name: "aubluedead",
			ID:   "866558066660999218",
		},
		game.Green: {
			Name: "augreendead",
			ID:   "866558067088949258",
		},
		game.Pink: {
			Name: "aupinkdead",
			ID:   "866558066945556512",
		},
		game.Orange: {
			Name: "auorangedead",
			ID:   "866558067508510730",
		},
		game.Yellow: {
			Name: "auyellowdead",
			ID:   "866558067206520862",
		},
		game.Black: {
			Name: "aublackdead",
			ID:   "866558066668339250",
		},
		game.White: {
			Name: "auwhitedead",
			ID:   "866558067231293450",
		},
		game.Purple: {
			Name: "aupurpledead",
			ID:   "866558067223298048",
		},
		game.Brown: {
			Name: "aubrowndead",
			ID:   "866558066945163304",
		},
		game.Cyan: {
			Name: "aucyandead",
			ID:   "866558067051200512",
		},
		game.Lime: {
			Name: "aulimedead",
			ID:   "866558067344408596",
		},
		game.Maroon: {
			Name: "aumaroondead",
			ID:   "866558067238895626",
		},
		game.Rose: {
			Name: "aurosedead",
			ID:   "866558067083444225",
		},
		game.Banana: {
			Name: "aubananadead",
			ID:   "866558066342625350",
		},
		game.Gray: {
			Name: "augraydead",
			ID:   "866558067049758740",
		},
		game.Tan: {
			Name: "autandead",
			ID:   "866558067230638120",
		},
		game.Coral: {
			Name: "aucoraldead",
			ID:   "866558067024723978",
		},
	},
}

/*
Helpful for copy/paste into Discord to get new emoji IDs when they are re-uploaded...
\:aured:
\:aublue:
\:augreen:
\:aupink:
\:auorange:
\:auyellow:
\:aublack:
\:auwhite:
\:aupurple:
\:aubrown:
\:aucyan:
\:aulime:
\:aumaroon:
\:aurose:
\:aubanana:
\:augray:
\:autan:
\:aucoral:

\:aureddead:
\:aubluedead:
\:augreendead:
\:aupinkdead:
\:auorangedead:
\:auyellowdead:
\:aublackdead:
\:auwhitedead:
\:aupurpledead:
\:aubrowndead:
\:aucyandead:
\:aulimedead:
\:aumaroondead:
\:aurosedead:
\:aubananadead:
\:augraydead:
\:autandead:
\:aucoraldead:
*/

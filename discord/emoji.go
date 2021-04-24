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
	topMap[true] = make([]Emoji, 24) // 12 colors for alive/dead
	topMap[false] = make([]Emoji, 24)
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
		game.Skincolor: {
			Name: "auskincolor",
			ID:   "835189203381256193",
		},
		game.Bordeaux: {
			Name: "aubordeaux",
			ID:   "835222234318241842",
		},
		game.Olive: {
			Name: "auolive",
			ID:   "835248969777021030",
		},
		game.Turqoise: {
			Name: "auturqoise",
			ID:   "835249842577997834",
		},
		game.Mint: {
			Name: "aumint",
			ID:   "835253671276970024",
		},
		game.Lavender: {
			Name: "aulavender",
			ID:   "835254463094718474",
		},
		game.Nougat: {
			Name: "aunougat",
			ID:   "835284359694909540",
		},
		game.Peach: {
			Name: "aupeach",
			ID:   "835322590582538280",
		},
		game.Neongreen: {
			Name: "auneongreen",
			ID:   "835319035709227018",
		},
		game.Hotpink: {
			Name: "auhotpink",
			ID:   "835319857682710544",
		},
		game.Gray: {
			Name: "augray",
			ID:   "835320515660480563",
		},
		game.Petrol: {
			Name: "aupetrol",
			ID:   "835321101293846529",
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
		game.Skincolor: {
			Name: "auskincolordead",
			ID:   "835189220154015775",
		},
		game.Bordeaux: {
			Name: "aubordeauxdead",
			ID:   "835222251703369828",
		},
		game.Olive: {
			Name: "auolivedead",
			ID:   "835248989338075146",
		},
		game.Turqoise: {
			Name: "auturqoisedead",
			ID:   "835249860823220325",
		},
		game.Mint: {
			Name: "aumintdead",
			ID:   "835253689526255716",
		},
		game.Lavender: {
			Name: "aulavenderdead",
			ID:   "835254483842760775",
		},
		game.Nougat: {
			Name: "aunougatdead",
			ID:   "835284392812609556",
		},
		game.Peach: {
			Name: "aupeachdead",
			ID:   "835322622916689990",
		},
		game.Neongreen: {
			Name: "auneongreendead",
			ID:   "835319053458997249",
		},
		game.Hotpink: {
			Name: "auhotpinkdead",
			ID:   "835319875479404545",
		},
		game.Gray: {
			Name: "augraydead",
			ID:   "835320547948757023",
		},
		game.Petrol: {
			Name: "aupetroldead",
			ID:   "835321129232760853",
		},
	},
}

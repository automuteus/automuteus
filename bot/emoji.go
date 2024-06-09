package bot

import (
	"encoding/base64"
	"io"
	"log"
	"net/http"

	"github.com/j0nas500/automuteus/v8/pkg/game"

	"github.com/bwmarrin/discordgo"
)

const (
	UnlinkEmojiName = "auunlink"
	X               = "❌"
	ThumbsUp        = "👍"
	Hourglass       = "⌛"
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
	bytes, err := io.ReadAll(response.Body)
	if err != nil {
		log.Println(err)
	}
	encodedStr := base64.StdEncoding.EncodeToString(bytes)
	return "data:image/png;base64," + encodedStr
}

func (a AlivenessEmojis) isEmpty() bool {
	if v, ok := a[true]; ok {
		for _, vv := range v {
			if vv.Name == "" || vv.ID == "" {
				return true
			}
		}
	} else {
		return true
	}
	if v, ok := a[false]; ok {
		for _, vv := range v {
			if vv.Name == "" || vv.ID == "" {
				return true
			}
		}
	} else {
		return true
	}
	return false
}

func emptyStatusEmojis() AlivenessEmojis {
	topMap := make(AlivenessEmojis)
	topMap[true] = make([]Emoji, 35) // 35 colors for alive/dead
	topMap[false] = make([]Emoji, 35)
	return topMap
}

func (bot *Bot) verifyEmojis(s *discordgo.Session, guildID string, alive bool, serverEmojis []*discordgo.Emoji, add bool) {
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
		if add && !alreadyExists {
			b64 := emoji.DownloadAndBase64Encode()
			p := discordgo.EmojiParams{
				Name:  emoji.Name,
				Image: b64,
				Roles: nil,
			}
			em, err := s.GuildEmojiCreate(guildID, &p)
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

// TODO
func EmojisToSelectMenuOptions(emojis []Emoji, unlinkEmoji string, isVanilla bool) (arr []discordgo.SelectMenuOption) {
	if isVanilla {
		for i := 0; i < 18; i++ {
			arr = append(arr, emojis[i].toSelectMenuOption(game.GetColorStringForInt(i)))
		}
	} else {
		for i := 18; i < len(emojis); i++ {
			arr = append(arr, emojis[i].toSelectMenuOption(game.GetColorStringForInt(i)))
		}
	}
	/*for i, v := range emojis {
		arr = append(arr, v.toSelectMenuOption(game.GetColorStringForInt(i)))
	}*/
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
			ID:   "1046352430000001075",
		},
		game.Blue: {
			Name: "aublue",
			ID:   "1046352431254077502",
		},
		game.Green: {
			Name: "augreen",
			ID:   "1046352432369770566",
		},
		game.Pink: {
			Name: "aupink",
			ID:   "1046352433682579466",
		},
		game.Orange: {
			Name: "auorange",
			ID:   "1046352435272233010",
		},
		game.Yellow: {
			Name: "auyellow",
			ID:   "1046352437184823347",
		},
		game.Black: {
			Name: "aublack",
			ID:   "1046352438233411614",
		},
		game.White: {
			Name: "auwhite",
			ID:   "1046352439386832916",
		},
		game.Purple: {
			Name: "aupurple",
			ID:   "1046352440884211812",
		},
		game.Brown: {
			Name: "aubrown",
			ID:   "1046352442440306738",
		},
		game.Cyan: {
			Name: "aucyan",
			ID:   "1046352443774083092",
		},
		game.Lime: {
			Name: "aulime",
			ID:   "1046352445317578872",
		},
		game.Maroon: {
			Name: "aumaroon",
			ID:   "1046352446538141756",
		},
		game.Rose: {
			Name: "aurose",
			ID:   "1046352448031309884",
		},
		game.Banana: {
			Name: "aubanana",
			ID:   "1046352449310576761",
		},
		game.Gray: {
			Name: "augray",
			ID:   "1046352450866655323",
		},
		game.Tan: {
			Name: "autan",
			ID:   "1046352452586328064",
		},
		game.Coral: {
			Name: "aucoral",
			ID:   "1046352453899137155",
		},
		game.Salmon: {
			Name: "ausalmon",
			ID:   "1046396936162381885",
		},
		game.Bordeaux: {
			Name: "aubordeaux",
			ID:   "1046396801386811392",
		},
		game.Olive: {
			Name: "auolive",
			ID:   "1046396896366829658",
		},
		game.Turqoise: {
			Name: "auturqoise",
			ID:   "1046396990365372540",
		},
		game.Mint: {
			Name: "aumint",
			ID:   "1046396868470525962",
		},
		game.Lavender: {
			Name: "aulavender",
			ID:   "1046396841970892852",
		},
		game.Nougat: {
			Name: "aunougat",
			ID:   "1046396884794744962",
		},
		game.Peach: {
			Name: "aupeach",
			ID:   "1046396909662777405",
		},
		game.Wasabi: {
			Name: "auwasabi",
			ID:   "1046397002218479626",
		},
		game.HotPink: {
			Name: "auhotpink",
			ID:   "1046396815408377966",
		},
		game.Petrol: {
			Name: "aupetrol",
			ID:   "1046396922920964126",
		},
		game.Lemon: {
			Name: "aulemon",
			ID:   "1046396854293757962",
		},
		game.SignalOrange: {
			Name: "ausignalorange",
			ID:   "1046396948107755531",
		},
		game.Teal: {
			Name: "auteal",
			ID:   "1046396975064555611",
		},
		game.Blurple: {
			Name: "aublurple",
			ID:   "1046396788367708230",
		},
		game.Sunrise: {
			Name: "ausunrise",
			ID:   "1046396960279642122",
		},
		game.Ice: {
			Name: "auice",
			ID:   "1046396827739635772",
		},
	},
	false: []Emoji{
		game.Red: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Blue: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Green: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Pink: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Orange: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Yellow: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Black: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.White: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Purple: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Brown: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Cyan: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Lime: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Maroon: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Rose: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Banana: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Gray: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Tan: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Coral: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Salmon: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Bordeaux: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Olive: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Turqoise: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Mint: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Lavender: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Nougat: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Peach: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Wasabi: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.HotPink: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Petrol: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Lemon: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.SignalOrange: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Teal: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Blurple: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Sunrise: {
			Name: "aureddead",
			ID:   "1046402641435049984",
		},
		game.Ice: {
			Name: "aureddead",
			ID:   "11046402641435049984",
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
\:ausalmon:
\:aubordeaux:
\:auolive:
\:auturqoise:
\:aumint:
\:aulavender:
\:aunougat:
\:aupeach:
\:auwasabi:
\:auhotpink:
\:aupetrol:
\:aulemon:
\:ausignalorange:
\:auteal:
\:aublurple:
\:ausunrise:
\:auice:

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





\:ausalmondead:
\:aubordeauxdead:
\auolivedead:
\:auturqoisedead:
\:aumintdead:
\:aulavenderdead:
\:aunougatdead:
\:aupeachdead:
WASABI
\:auhotpinkdead:
\:aupetroldead:
LEMON
SIGNALORANGE
TEAL
BLURPLE
SUNRISE
ICE
*/

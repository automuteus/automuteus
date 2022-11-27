package discord

import (
	"encoding/base64"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/j0nas500/utils/pkg/game"

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
	bytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Println(err)
	}
	encodedStr := base64.StdEncoding.EncodeToString(bytes)
	return "data:image/png;base64," + encodedStr
}

func emptyStatusEmojis() AlivenessEmojis {
	topMap := make(AlivenessEmojis)
	topMap[true] = make([]Emoji, 35) // 35 colors for alive/dead
	topMap[false] = make([]Emoji, 35)
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

// TODO
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
			Name: "aured",
			ID:   "1046352430000001075",
		},
		game.Bordeaux: {
			Name: "aublue",
			ID:   "1046352431254077502",
		},
		game.Olive: {
			Name: "augreen",
			ID:   "1046352432369770566",
		},
		game.Turqoise: {
			Name: "aupink",
			ID:   "1046352433682579466",
		},
		game.Mint: {
			Name: "auorange",
			ID:   "1046352435272233010",
		},
		game.Lavender: {
			Name: "auyellow",
			ID:   "1046352437184823347",
		},
		game.Nougat: {
			Name: "aublack",
			ID:   "1046352438233411614",
		},
		game.Peach: {
			Name: "auwhite",
			ID:   "1046352439386832916",
		},
		game.Wasabi: {
			Name: "aupurple",
			ID:   "1046352440884211812",
		},
		game.HotPink: {
			Name: "aubrown",
			ID:   "1046352442440306738",
		},
		game.Petrol: {
			Name: "autan",
			ID:   "1046352452586328064",
		},
		game.Lemon: {
			Name: "aucyan",
			ID:   "1046352443774083092",
		},
		game.SignalOrange: {
			Name: "aulime",
			ID:   "1046352445317578872",
		},
		game.Teal: {
			Name: "aumaroon",
			ID:   "1046352446538141756",
		},
		game.Blurple: {
			Name: "aurose",
			ID:   "1046352448031309884",
		},
		game.Sunrise: {
			Name: "aubanana",
			ID:   "1046352449310576761",
		},
		game.Ice: {
			Name: "augray",
			ID:   "1046352450866655323",
		},
	},
	false: []Emoji{
		game.Red: {
			Name: "aureddead",
			ID:   "1046352479119482921",
		},
		game.Blue: {
			Name: "aubluedead",
			ID:   "1046352481019494502",
		},
		game.Green: {
			Name: "augreendead",
			ID:   "1046352482856607864",
		},
		game.Pink: {
			Name: "aupinkdead",
			ID:   "1046352484211380254",
		},
		game.Orange: {
			Name: "auorangedead",
			ID:   "1046352485717135431",
		},
		game.Yellow: {
			Name: "auyellowdead",
			ID:   "1046352487071895652",
		},
		game.Black: {
			Name: "aublackdead",
			ID:   "1046352488468582420",
		},
		game.White: {
			Name: "auwhitedead",
			ID:   "1046352489894662204",
		},
		game.Purple: {
			Name: "aupurpledead",
			ID:   "1046352491316514888",
		},
		game.Brown: {
			Name: "aubrowndead",
			ID:   "1046352492725801030",
		},
		game.Cyan: {
			Name: "aucyandead",
			ID:   "1046352496811061268",
		},
		game.Lime: {
			Name: "aulimedead",
			ID:   "1046352502427222056",
		},
		game.Maroon: {
			Name: "aumaroondead",
			ID:   "1046352503635202088",
		},
		game.Rose: {
			Name: "aurosedead",
			ID:   "1046355494341718027",
		},
		game.Banana: {
			Name: "aubananadead",
			ID:   "1046355495415464008",
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
		game.Salmon: {
			Name: "aureddead",
			ID:   "1046352479119482921",
		},
		game.Bordeaux: {
			Name: "aubluedead",
			ID:   "1046352481019494502",
		},
		game.Olive: {
			Name: "augreendead",
			ID:   "1046352482856607864",
		},
		game.Turqoise: {
			Name: "aupinkdead",
			ID:   "1046352484211380254",
		},
		game.Mint: {
			Name: "auorangedead",
			ID:   "1046352485717135431",
		},
		game.Lavender: {
			Name: "auyellowdead",
			ID:   "1046352487071895652",
		},
		game.Nougat: {
			Name: "aublackdead",
			ID:   "1046352488468582420",
		},
		game.Peach: {
			Name: "auwhitedead",
			ID:   "1046352489894662204",
		},
		game.Wasabi: {
			Name: "aupurpledead",
			ID:   "1046352491316514888",
		},
		game.HotPink: {
			Name: "aubrowndead",
			ID:   "1046352492725801030",
		},
		game.Petrol: {
			Name: "autandead",
			ID:   "1046367153047216188",
		},
		game.Lemon: {
			Name: "aucyandead",
			ID:   "1046352496811061268",
		},
		game.SignalOrange: {
			Name: "aulimedead",
			ID:   "1046352502427222056",
		},
		game.Teal: {
			Name: "aumaroondead",
			ID:   "1046352503635202088",
		},
		game.Blurple: {
			Name: "aurosedead",
			ID:   "1046355494341718027",
		},
		game.Sunrise: {
			Name: "aubananadead",
			ID:   "1046355495415464008",
		},
		game.Ice: {
			Name: "augraydead",
			ID:   "1046367151621144616",
		},
	},
}

var GlobalAlivenessVanillaEmojis = AlivenessEmojis{
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
	},
	false: []Emoji{
		game.Red: {
			Name: "aureddead",
			ID:   "1046352479119482921",
		},
		game.Blue: {
			Name: "aubluedead",
			ID:   "1046352481019494502",
		},
		game.Green: {
			Name: "augreendead",
			ID:   "1046352482856607864",
		},
		game.Pink: {
			Name: "aupinkdead",
			ID:   "1046352484211380254",
		},
		game.Orange: {
			Name: "auorangedead",
			ID:   "1046352485717135431",
		},
		game.Yellow: {
			Name: "auyellowdead",
			ID:   "1046352487071895652",
		},
		game.Black: {
			Name: "aublackdead",
			ID:   "1046352488468582420",
		},
		game.White: {
			Name: "auwhitedead",
			ID:   "1046352489894662204",
		},
		game.Purple: {
			Name: "aupurpledead",
			ID:   "1046352491316514888",
		},
		game.Brown: {
			Name: "aubrowndead",
			ID:   "1046352492725801030",
		},
		game.Cyan: {
			Name: "aucyandead",
			ID:   "1046352496811061268",
		},
		game.Lime: {
			Name: "aulimedead",
			ID:   "1046352502427222056",
		},
		game.Maroon: {
			Name: "aumaroondead",
			ID:   "1046352503635202088",
		},
		game.Rose: {
			Name: "aurosedead",
			ID:   "1046355494341718027",
		},
		game.Banana: {
			Name: "aubananadead",
			ID:   "1046355495415464008",
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

var GlobalAlivenessTorEmojis = AlivenessEmojis{
	true: []Emoji{
		game.Coral: {
			Name: "aucoral",
			ID:   "1046352453899137155",
		},
		game.Salmon: {
			Name: "aured",
			ID:   "1046352430000001075",
		},
		game.Bordeaux: {
			Name: "aublue",
			ID:   "1046352431254077502",
		},
		game.Olive: {
			Name: "augreen",
			ID:   "1046352432369770566",
		},
		game.Turqoise: {
			Name: "aupink",
			ID:   "1046352433682579466",
		},
		game.Mint: {
			Name: "auorange",
			ID:   "1046352435272233010",
		},
		game.Lavender: {
			Name: "auyellow",
			ID:   "1046352437184823347",
		},
		game.Nougat: {
			Name: "aublack",
			ID:   "1046352438233411614",
		},
		game.Peach: {
			Name: "auwhite",
			ID:   "1046352439386832916",
		},
		game.Wasabi: {
			Name: "aupurple",
			ID:   "1046352440884211812",
		},
		game.HotPink: {
			Name: "aubrown",
			ID:   "1046352442440306738",
		},
		game.Petrol: {
			Name: "autan",
			ID:   "1046352452586328064",
		},
		game.Lemon: {
			Name: "aucyan",
			ID:   "1046352443774083092",
		},
		game.SignalOrange: {
			Name: "aulime",
			ID:   "1046352445317578872",
		},
		game.Teal: {
			Name: "aumaroon",
			ID:   "1046352446538141756",
		},
		game.Blurple: {
			Name: "aurose",
			ID:   "1046352448031309884",
		},
		game.Sunrise: {
			Name: "aubanana",
			ID:   "1046352449310576761",
		},
		game.Ice: {
			Name: "augray",
			ID:   "1046352450866655323",
		},
	},
	false: []Emoji{
		game.Salmon: {
			Name: "aureddead",
			ID:   "1046352479119482921",
		},
		game.Bordeaux: {
			Name: "aubluedead",
			ID:   "1046352481019494502",
		},
		game.Olive: {
			Name: "augreendead",
			ID:   "1046352482856607864",
		},
		game.Turqoise: {
			Name: "aupinkdead",
			ID:   "1046352484211380254",
		},
		game.Mint: {
			Name: "auorangedead",
			ID:   "1046352485717135431",
		},
		game.Lavender: {
			Name: "auyellowdead",
			ID:   "1046352487071895652",
		},
		game.Nougat: {
			Name: "aublackdead",
			ID:   "1046352488468582420",
		},
		game.Peach: {
			Name: "auwhitedead",
			ID:   "1046352489894662204",
		},
		game.Wasabi: {
			Name: "aupurpledead",
			ID:   "1046352491316514888",
		},
		game.HotPink: {
			Name: "aubrowndead",
			ID:   "1046352492725801030",
		},
		game.Petrol: {
			Name: "autandead",
			ID:   "1046367153047216188",
		},
		game.Lemon: {
			Name: "aucyandead",
			ID:   "1046352496811061268",
		},
		game.SignalOrange: {
			Name: "aulimedead",
			ID:   "1046352502427222056",
		},
		game.Teal: {
			Name: "aumaroondead",
			ID:   "1046352503635202088",
		},
		game.Blurple: {
			Name: "aurosedead",
			ID:   "1046355494341718027",
		},
		game.Sunrise: {
			Name: "aubananadead",
			ID:   "1046355495415464008",
		},
		game.Ice: {
			Name: "augraydead",
			ID:   "1046367151621144616",
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
WASABI
\:auhotpink:
\:aupetrol:
LEMON
SIGNALORANGE
TEAL
BLURPLE
SUNRISE
ICE

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

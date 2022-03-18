package setting

import (
	"errors"
	"fmt"
	"github.com/automuteus/utils/pkg/game"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type Name string

const (
	MinDelay = 1
	MaxDelay = 10

	MinLeaderBoardSize = 1
	MaxLeaderBoardSize = 10

	MinLeaderBoardMin = 1
	MaxLeaderBoardMin = 100

	MinMatchSummaryDelete = -1
	MaxMatchSummaryDelete = 60
)

const (
	Language            = "language"
	VoiceRules          = "voice-rules"
	AdminUserIDs        = "admin-user-ids"
	RoleIDs             = "operator-roles"
	UnmuteDead          = "unmute-dead"
	MapVersion          = "map-version"
	Delays              = "delays"
	MatchSummary        = "match-summary-duration"
	MatchSummaryChannel = "match-summary-channel"
	AutoRefresh         = "auto-refresh"
	LeaderboardMention  = "leaderboard-mention"
	LeaderboardSize     = "leaderboard-size"
	LeaderboardMin      = "leaderboard-min"
	MuteSpectators      = "mute-spectators"
	DisplayRoomCode     = "display-room-code"
	Show                = "show"
	Reset               = "reset"
)

func GetSettingByName(name string) *Setting {
	for _, v := range AllSettings {
		if string(v.Name) == name {
			return &v
		}
	}
	return nil
}

type Argument struct {
	Name string
	// really wish I could figure out the way to use Generics to enforce the type matches across
	// the two fields below, but I don't think it's possible. Because then you end up having to constrain
	// the types at the highest level (Settings), which means all your settings can only accept strings,
	// only accept @usermentions, etc.
	OptionType    discordgo.ApplicationCommandOptionType
	optionChoices []any
	Required      bool
}

//type User string
//type Role string
//type Channel string
//
//type OptionType interface {
//	string | bool | User | Role | Channel
//}

func (a *Argument) Choices() []*discordgo.ApplicationCommandOptionChoice {
	var choices []*discordgo.ApplicationCommandOptionChoice

	// non-string types have choices specified, but they are min/max constraints, not strict options
	if a.OptionType == discordgo.ApplicationCommandOptionString {
		for _, choice := range a.optionChoices {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  choice.(string),
				Value: choice,
			})
		}
	}

	return choices
}

type IntOption int64

func (i IntOption) AsString() string {
	return fmt.Sprintf("%d", i)
}

type BoolOption bool

func (b BoolOption) AsString() string {
	return fmt.Sprintf("%t", b)
}

type User string

func (u User) AsString() string {
	return string(u)
}

type Channel string

func (c Channel) AsString() string {
	return string(c)
}

type OptionType interface {
	AsString() string
}

func (a Argument) AsString(option *discordgo.ApplicationCommandInteractionDataOption, s *discordgo.Session) string {
	switch a.OptionType {
	case discordgo.ApplicationCommandOptionBoolean:
		return BoolOption(option.BoolValue()).AsString()
	case discordgo.ApplicationCommandOptionString:
		return option.StringValue()
	case discordgo.ApplicationCommandOptionInteger:
		return IntOption(option.IntValue()).AsString()
	case discordgo.ApplicationCommandOptionUser:
		return option.UserValue(s).Mention()
	case discordgo.ApplicationCommandOptionChannel:
		return option.ChannelValue(s).Mention()
	default:
		return ""
	}
}

func (a Argument) Validate(option *discordgo.ApplicationCommandInteractionDataOption) error {
	switch a.OptionType {
	case discordgo.ApplicationCommandOptionString:
		if len(a.optionChoices) > 0 {
			str := option.StringValue()
			for _, v := range a.optionChoices {
				if v.(string) == str {
					return nil
				}
			}
			return errors.New("no option found in constraints that matches the provided arg: " + str)
		}
		return nil
	case discordgo.ApplicationCommandOptionInteger:
		if len(a.optionChoices) == 2 {
			v := int(option.IntValue())
			if v >= a.optionChoices[0].(int) && v <= a.optionChoices[1].(int) {
				return nil
			} else {
				return errors.New(fmt.Sprintf("argument %d was not in the range [%d, %d]", v, a.optionChoices[0].(int), a.optionChoices[1].(int)))
			}
		}
		return nil
	default:
		return nil
	}
}

type Setting struct {
	Name      Name
	ShortDesc string
	Arguments []Argument
	Premium   bool
}

var phaseOptions = []any{
	string(game.PhaseNames[game.LOBBY]),
	string(game.PhaseNames[game.TASKS]),
	string(game.PhaseNames[game.DISCUSS]),
}

// TODO parse these from JSON so the web UI can use the same file
var AllSettings = []Setting{
	{
		Name:      Language,
		ShortDesc: "Bot Language",
		Arguments: []Argument{
			{"language-code", discordgo.ApplicationCommandOptionString, []any{}, false},
		},
		Premium: false,
	},
	{
		Name:      VoiceRules,
		ShortDesc: "Bot round behavior",
		Arguments: []Argument{
			{"deaf-or-muted", discordgo.ApplicationCommandOptionString, []any{"deafened", "muted"}, true},
			{"phase", discordgo.ApplicationCommandOptionString, phaseOptions, true},
			{"alive", discordgo.ApplicationCommandOptionString, []any{"alive", "dead"}, true},
			{"value", discordgo.ApplicationCommandOptionBoolean, []any{}, false},
		},
		Premium: false,
	},
	{
		Name:      AdminUserIDs,
		ShortDesc: "Bot Admins",
		Arguments: []Argument{{"user", discordgo.ApplicationCommandOptionUser, []any{}, false}},
		Premium:   false,
	},
	{
		Name:      RoleIDs,
		ShortDesc: "Bot Operators",
		Arguments: []Argument{{"role", discordgo.ApplicationCommandOptionRole, []any{}, false}},
		Premium:   false,
	},
	{
		Name:      UnmuteDead,
		ShortDesc: "Bot unmutes deaths immediately",
		Arguments: []Argument{{"unmute", discordgo.ApplicationCommandOptionBoolean, []any{}, false}},
		Premium:   false,
	},
	{
		Name:      MapVersion,
		ShortDesc: "Map version",
		Arguments: []Argument{{"detailed", discordgo.ApplicationCommandOptionBoolean, []any{}, false}},
		Premium:   false,
	},
	{
		Name:      Delays,
		ShortDesc: "Game transition mute delays",
		Arguments: []Argument{
			{"start-phase", discordgo.ApplicationCommandOptionString, phaseOptions, true},
			{"end-phase", discordgo.ApplicationCommandOptionString, phaseOptions, true},
			{"delay", discordgo.ApplicationCommandOptionInteger, []any{MinDelay, MaxDelay}, false},
		},
		Premium: false,
	},
	{
		Name:      MatchSummary,
		ShortDesc: "Match Summary Message Duration",
		Arguments: []Argument{{"minutes-duration", discordgo.ApplicationCommandOptionInteger, []any{MinMatchSummaryDelete, MaxMatchSummaryDelete}, false}},
		Premium:   true,
	},
	{
		Name:      MatchSummaryChannel,
		ShortDesc: "Channel for Match Summaries",
		Arguments: []Argument{{"channel", discordgo.ApplicationCommandOptionChannel, []any{}, false}},
		Premium:   true,
	},
	{
		Name:      AutoRefresh,
		ShortDesc: "Autorefresh Status Message",
		Arguments: []Argument{{"autorefresh", discordgo.ApplicationCommandOptionBoolean, []any{}, false}},
		Premium:   true,
	},
	{
		Name:      LeaderboardMention,
		ShortDesc: "Mention players in Leaderboard",
		Arguments: []Argument{{"use-mention", discordgo.ApplicationCommandOptionBoolean, []any{}, false}},
		Premium:   true,
	},
	{
		Name:      LeaderboardSize,
		ShortDesc: "Player Leaderboard Size",
		Arguments: []Argument{{"size", discordgo.ApplicationCommandOptionInteger, []any{MinLeaderBoardSize, MaxLeaderBoardSize}, false}},
		Premium:   true,
	},
	{
		Name:      LeaderboardMin,
		ShortDesc: "Minimum Games for Leaderboard",
		Arguments: []Argument{{"minimum", discordgo.ApplicationCommandOptionInteger, []any{MinLeaderBoardMin, MaxLeaderBoardMin}, false}},
		Premium:   true,
	},
	{
		Name:      MuteSpectators,
		ShortDesc: "Mute Spectators like Dead Players",
		Arguments: []Argument{{"mute", discordgo.ApplicationCommandOptionBoolean, []any{}, false}},
		Premium:   true,
	},
	{
		Name:      DisplayRoomCode,
		ShortDesc: "Visibility for the ROOM CODE",
		Arguments: []Argument{{"visibility", discordgo.ApplicationCommandOptionString, []any{"always", "spoiler", "never"}, false}},
		Premium:   true,
	},
	{
		Name:      Show,
		ShortDesc: "Show All Settings",
		Arguments: []Argument{},
		Premium:   false,
	},
	{
		Name:      Reset,
		ShortDesc: "Reset Bot Settings",
		Arguments: []Argument{},
		Premium:   false,
	},
}

func ConstructEmbedForSetting(value string, setting *Setting, sett *settings.GuildSettings) discordgo.MessageEmbed {
	if setting == nil {
		return discordgo.MessageEmbed{}
	}
	title := string(setting.Name)
	if setting.Premium {
		title = "💎 " + title
	}
	if value == "" {
		value = "null"
	}

	desc := sett.LocalizeMessage(&i18n.Message{
		ID:    "settings.ConstructEmbedForSetting.StarterDesc",
		Other: "Type `/settings {{.Command}}` to view or change this setting.\n\n",
	}, map[string]interface{}{
		"Command": setting.Name,
	})
	return discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       title,
		Description: desc + setting.ShortDesc,
		Timestamp:   "",
		Color:       15844367, // GOLD
		Image:       nil,
		Thumbnail:   nil,
		Video:       nil,
		Provider:    nil,
		Author:      nil,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "settings.ConstructEmbedForSetting.Fields.CurrentValue",
					Other: "Current Value",
				}),
				Value:  value,
				Inline: false,
			},
		},
	}
}

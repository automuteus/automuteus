package setting

import (
	"github.com/automuteus/utils/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type Name string

const (
	Language            = "language"
	AdminUserIDs        = "admin-user-ids"
	RoleIDs             = "operator-roles"
	UnmuteDead          = "unmute-dead"
	MapVersion          = "map-version"
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

type ISetting interface {
	HandleSetting(*discordgo.MessageCreate, *settings.GuildSettings, []string) (interface{}, bool)
}

type Setting struct {
	Name         Name
	ShortDesc    string
	ArgumentName string
	ArgumentType discordgo.ApplicationCommandOptionType
	Premium      bool
}

// TODO parse these from JSON so the web UI can use the same file
var AllSettings = []Setting{
	{
		Name:         Language,
		ShortDesc:    "Bot Language",
		ArgumentName: "language-code",
		ArgumentType: discordgo.ApplicationCommandOptionString,
		Premium:      false,
	},
	{
		Name:         AdminUserIDs,
		ShortDesc:    "Bot Admins",
		ArgumentName: "user",
		ArgumentType: discordgo.ApplicationCommandOptionUser,
		Premium:      false,
	},
	{
		Name:         RoleIDs,
		ShortDesc:    "Bot Operators",
		ArgumentName: "role",
		ArgumentType: discordgo.ApplicationCommandOptionRole,
		Premium:      false,
	},
	{
		Name:         UnmuteDead,
		ShortDesc:    "Bot Unmutes Deaths",
		ArgumentName: "unmute",
		ArgumentType: discordgo.ApplicationCommandOptionBoolean,
		Premium:      false,
	},
	{
		Name:         MapVersion,
		ShortDesc:    "Map version",
		ArgumentName: "detailed",
		ArgumentType: discordgo.ApplicationCommandOptionBoolean,
		Premium:      false,
	},
	{
		Name:         MatchSummary,
		ShortDesc:    "Match Summary Message Duration",
		ArgumentName: "minute-duration",
		ArgumentType: discordgo.ApplicationCommandOptionInteger,
		Premium:      true,
	},
	{
		Name:         MatchSummaryChannel,
		ShortDesc:    "Channel for Match Summaries",
		ArgumentName: "channel",
		ArgumentType: discordgo.ApplicationCommandOptionChannel,
		Premium:      true,
	},
	{
		Name:         AutoRefresh,
		ShortDesc:    "Autorefresh Status Message",
		ArgumentName: "autorefresh",
		ArgumentType: discordgo.ApplicationCommandOptionBoolean,
		Premium:      true,
	},
	{
		Name:         LeaderboardMention,
		ShortDesc:    "Player Leaderboard Mention Format",
		ArgumentName: "format",
		ArgumentType: discordgo.ApplicationCommandOptionBoolean,
		Premium:      true,
	},
	{
		Name:         LeaderboardSize,
		ShortDesc:    "Player Leaderboard Size",
		ArgumentName: "size",
		ArgumentType: discordgo.ApplicationCommandOptionInteger,
		Premium:      true,
	},
	{
		Name:         LeaderboardMin,
		ShortDesc:    "Minimum Games for Leaderboard",
		ArgumentName: "minimum",
		ArgumentType: discordgo.ApplicationCommandOptionInteger,
		Premium:      true,
	},
	{
		Name:         MuteSpectators,
		ShortDesc:    "Mute Spectators like Dead Players",
		ArgumentName: "mute",
		ArgumentType: discordgo.ApplicationCommandOptionBoolean,
		Premium:      true,
	},
	{
		Name:         DisplayRoomCode,
		ShortDesc:    "Visibility for the ROOM CODE",
		ArgumentName: "visible",
		ArgumentType: discordgo.ApplicationCommandOptionBoolean,
		Premium:      true,
	},
	{
		Name:         Show,
		ShortDesc:    "Show All Settings",
		ArgumentName: "",
		ArgumentType: 0,
		Premium:      false,
	},
	{
		Name:         Reset,
		ShortDesc:    "Reset Bot Settings",
		ArgumentName: "",
		ArgumentType: 0,
		Premium:      false,
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

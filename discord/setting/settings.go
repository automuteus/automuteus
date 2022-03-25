package setting

import (
	"fmt"
	"github.com/automuteus/utils/pkg/game"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const (
	MaxDelay = 10

	MaxLeaderBoardSize float64 = 10

	MaxLeaderBoardMin float64 = 100

	MaxMatchSummaryDelete float64 = 60

	View  = "view"
	Clear = "clear"
	User  = "user"
	Role  = "role"
)

var (
	MinDelay float64 = 0

	MinLeaderBoardSize float64 = 1

	MinLeaderBoardMin float64 = 1

	MinMatchSummaryDelete float64 = -1
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
		if v.Name == name {
			return &v
		}
	}
	return nil
}

func ToString(option *discordgo.ApplicationCommandInteractionDataOption, s *discordgo.Session) string {
	switch option.Type {
	case discordgo.ApplicationCommandOptionBoolean:
		return fmt.Sprintf("%t", option.BoolValue())
	case discordgo.ApplicationCommandOptionString:
		return option.StringValue()
	case discordgo.ApplicationCommandOptionInteger:
		return fmt.Sprintf("%d", option.IntValue())
	case discordgo.ApplicationCommandOptionUser:
		return option.UserValue(s).Mention()
	case discordgo.ApplicationCommandOptionChannel:
		return option.ChannelValue(s).Mention()
	case discordgo.ApplicationCommandOptionSubCommand:
		return option.Name
	default:
		return ""
	}
}

type Setting struct {
	Name      string
	ShortDesc string
	Arguments []*discordgo.ApplicationCommandOption
	Premium   bool
}

var phaseChoices = []*discordgo.ApplicationCommandOptionChoice{
	{
		Name:  string(game.PhaseNames[game.LOBBY]),
		Value: string(game.PhaseNames[game.LOBBY]),
	},
	{
		Name:  string(game.PhaseNames[game.TASKS]),
		Value: string(game.PhaseNames[game.TASKS]),
	},
	{
		Name:  string(game.PhaseNames[game.DISCUSS]),
		Value: string(game.PhaseNames[game.DISCUSS]),
	},
}

// TODO parse these from JSON so the web UI can use the same file
var AllSettings = []Setting{
	{
		Name:      Language,
		ShortDesc: "Bot Language",
		Arguments: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "language-code",
				Description: "language-code",
			},
		},
		Premium: false,
	},
	{
		Name:      VoiceRules,
		ShortDesc: "Bot round behavior",
		Arguments: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "deaf-or-muted",
				Description: "deaf-or-muted",
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{
						Name:  "deafened",
						Value: "deafened",
					},
					{
						Name:  "muted",
						Value: "muted",
					},
				},
				Required: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "phase",
				Description: "phase",
				Choices:     phaseChoices,
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "alive",
				Description: "alive",
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{
						Name:  "alive",
						Value: "alive",
					},
					{
						Name:  "dead",
						Value: "dead",
					},
				},
				Required: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "value",
				Description: "value",
			},
		},
		Premium: false,
	},
	{
		Name:      AdminUserIDs,
		ShortDesc: "Bot Admins",
		Arguments: []*discordgo.ApplicationCommandOption{
			{
				Name:        View,
				Description: "View Admins",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        Clear,
				Description: "Clear Admins",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        User,
				Description: "Discord user to make an Admin",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        User,
						Description: "Discord user to make an Admin",
						Type:        discordgo.ApplicationCommandOptionUser,
						Required:    true,
					},
				},
			},
		},
		Premium: false,
	},
	{
		Name:      RoleIDs,
		ShortDesc: "Bot Operators",
		Arguments: []*discordgo.ApplicationCommandOption{
			{
				Name:        View,
				Description: "View Operators",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        Clear,
				Description: "Clear Operators",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        Role,
				Description: "Discord role to make Operators",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        Role,
						Description: "Discord role to make Operators",
						Type:        discordgo.ApplicationCommandOptionRole,
						Required:    true,
					},
				},
			},
		},
		Premium: false,
	},
	{
		Name:      UnmuteDead,
		ShortDesc: "Bot unmutes deaths immediately",
		Arguments: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "unmute",
				Description: "unmute",
			},
		},
		Premium: false,
	},
	{
		Name:      MapVersion,
		ShortDesc: "Map version",
		Arguments: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "detailed",
				Description: "detailed",
			},
		},
		Premium: false,
	},
	{
		Name:      Delays,
		ShortDesc: "Game transition mute delays",
		Arguments: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "start-phase",
				Description: "start-phase",
				Choices:     phaseChoices,
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "end-phase",
				Description: "end-phase",
				Choices:     phaseChoices,
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "delay",
				Description: "delay",
				MinValue:    &MinDelay,
				MaxValue:    MaxDelay,
			},
		},
		Premium: false,
	},
	{
		Name:      MatchSummary,
		ShortDesc: "Match Summary Message Duration",
		Arguments: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "minutes-duration",
				Description: "minutes-duration",
				MinValue:    &MinMatchSummaryDelete,
				MaxValue:    MaxMatchSummaryDelete,
			},
		},
		Premium: true,
	},
	{
		Name:      MatchSummaryChannel,
		ShortDesc: "Channel for Match Summaries",
		Arguments: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionChannel,
				Name:         "channel",
				Description:  "channel",
				ChannelTypes: []discordgo.ChannelType{discordgo.ChannelTypeGuildText},
			},
		},
		Premium: true,
	},
	{
		Name:      AutoRefresh,
		ShortDesc: "Autorefresh Status Message",
		Arguments: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "autorefresh",
				Description: "autorefresh",
			},
		},
		Premium: true,
	},
	{
		Name:      LeaderboardMention,
		ShortDesc: "Mention players in Leaderboard",
		Arguments: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "use-mention",
				Description: "use-mention",
			},
		},
		Premium: true,
	},
	{
		Name:      LeaderboardSize,
		ShortDesc: "Player Leaderboard Size",
		Arguments: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "size",
				Description: "size",
				MinValue:    &MinLeaderBoardSize,
				MaxValue:    MaxLeaderBoardSize,
			},
		},
		Premium: true,
	},
	{
		Name:      LeaderboardMin,
		ShortDesc: "Minimum Games for Leaderboard",
		Arguments: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "minimum",
				Description: "minimum",
				MinValue:    &MinLeaderBoardMin,
				MaxValue:    MaxLeaderBoardMin,
			},
		},
		Premium: true,
	},
	{
		Name:      MuteSpectators,
		ShortDesc: "Mute Spectators like Dead Players",
		Arguments: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "mute",
				Description: "mute",
			},
		},
		Premium: true,
	},
	{
		Name:      DisplayRoomCode,
		ShortDesc: "Visibility for the ROOM CODE",
		Arguments: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "visibility",
				Description: "visibility",
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{
						Name:  "always",
						Value: "always",
					},
					{
						Name:  "spoiler",
						Value: "spoiler",
					},
					{
						Name:  "never",
						Value: "never",
					},
				},
			},
		},
		Premium: true,
	},
	{
		Name:      Show,
		ShortDesc: "Show All Settings",
		Arguments: []*discordgo.ApplicationCommandOption{},
		Premium:   false,
	},
	{
		Name:      Reset,
		ShortDesc: "Reset Bot Settings",
		Arguments: []*discordgo.ApplicationCommandOption{},
		Premium:   false,
	},
}

func ConstructEmbedForSetting(value string, setting *Setting, sett *settings.GuildSettings) discordgo.MessageEmbed {
	if setting == nil {
		return discordgo.MessageEmbed{}
	}
	title := setting.Name
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

package command

import (
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"strings"
)

type Type int

const (
	Help Type = iota
	New
	End
	Pause
	Refresh
	Link
	Unlink
	UnmuteAll
	Force
	Settings
	Map
	Cache
	Privacy
	Info
	DebugState
	ASCII
	Stats
	Premium
	Null
)

type Command struct {
	Aliases     []string
	Command     string
	Example     string
	Emoji       string
	CommandType Type
	ShortDesc   *i18n.Message
	Description *i18n.Message
	Arguments   *i18n.Message
	IsSecret    bool
	IsAdmin     bool
	IsOperator  bool
}

func GetCommand(arg string) Command {
	arg = strings.ToLower(arg)
	for _, cmd := range AllCommands {
		if arg == cmd.Command {
			return cmd
		}
		for _, al := range cmd.Aliases {
			if arg == al {
				return cmd
			}
		}
	}
	return AllCommands[Null]
}

// note, this mapping is HIERARCHICAL. If you type `l`, "link" would be used over "log"
var AllCommands = []Command{
	{
		CommandType: Help,
		Command:     "help",
		Example:     "help track",
		ShortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Help.shortDesc",
			Other: "Display help",
		},
		Description: &i18n.Message{
			ID:    "commands.AllCommands.Help.desc",
			Other: "Display bot help message, or see info about a Command",
		},
		Arguments: &i18n.Message{
			ID:    "commands.AllCommands.Help.args",
			Other: "None, or optional Command to see info for",
		},
		Aliases:    []string{"h"},
		IsSecret:   false,
		Emoji:      "❓",
		IsAdmin:    false,
		IsOperator: false,
	},
	{
		CommandType: New,
		Command:     "new",
		Example:     "new",
		ShortDesc: &i18n.Message{
			ID:    "commands.AllCommands.New.shortDesc",
			Other: "Start a new game",
		},
		Description: &i18n.Message{
			ID:    "commands.AllCommands.New.desc",
			Other: "Start a new game",
		},
		Arguments: &i18n.Message{
			ID:    "commands.AllCommands.New.args",
			Other: "None",
		},
		Aliases:    []string{"start", "n"},
		IsSecret:   false,
		Emoji:      "🕹",
		IsAdmin:    false,
		IsOperator: true,
	},
	{
		CommandType: End,
		Command:     "end",
		Example:     "end",
		ShortDesc: &i18n.Message{
			ID:    "commands.AllCommands.End.shortDesc",
			Other: "End the game",
		},
		Description: &i18n.Message{
			ID:    "commands.AllCommands.End.desc",
			Other: "End the current game",
		},
		Arguments: &i18n.Message{
			ID:    "commands.AllCommands.End.args",
			Other: "None",
		},
		Aliases:    []string{"stop", "e"},
		IsSecret:   false,
		Emoji:      "🛑",
		IsAdmin:    false,
		IsOperator: true,
	},
	{
		CommandType: Pause,
		Command:     "pause",
		Example:     "pause",
		ShortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Pause.shortDesc",
			Other: "Pause the bot",
		},
		Description: &i18n.Message{
			ID:    "commands.AllCommands.Pause.desc",
			Other: "Pause the bot so it doesn't automute/deafen. Will unmute/undeafen all players!",
		},
		Arguments: &i18n.Message{
			ID:    "commands.AllCommands.Pause.args",
			Other: "None",
		},
		Aliases:    []string{"unpause", "p"},
		IsSecret:   false,
		Emoji:      "⏸",
		IsAdmin:    false,
		IsOperator: true,
	},
	{
		CommandType: Refresh,
		Command:     "refresh",
		Example:     "refresh",
		ShortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Refresh.shortDesc",
			Other: "Refresh the bot status",
		},
		Description: &i18n.Message{
			ID:    "commands.AllCommands.Refresh.desc",
			Other: "Recreate the bot status message if it ends up too far in the chat",
		},
		Arguments: &i18n.Message{
			ID:    "commands.AllCommands.Refresh.args",
			Other: "None",
		},
		Aliases:    []string{"reload", "ref", "rel", "r"},
		IsSecret:   false,
		Emoji:      "♻",
		IsAdmin:    false,
		IsOperator: true,
	},
	{
		CommandType: Link,
		Command:     "link",
		Example:     "link @Soup red",
		ShortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Link.shortDesc",
			Other: "Link a Discord User",
		},
		Description: &i18n.Message{
			ID:    "commands.AllCommands.Link.desc",
			Other: "Manually link a Discord User to their in-game color or name",
		},
		Arguments: &i18n.Message{
			ID:    "commands.AllCommands.Link.args",
			Other: "<discord User> <in-game color or name>",
		},
		Aliases:    []string{"l"},
		IsSecret:   false,
		Emoji:      "🔗",
		IsAdmin:    false,
		IsOperator: true,
	},
	{
		CommandType: Unlink,
		Command:     "unlink",
		Example:     "unlink @Soup",
		ShortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Unlink.shortDesc",
			Other: "Unlink a Discord User",
		},
		Description: &i18n.Message{
			ID:    "commands.AllCommands.Unlink.desc",
			Other: "Manually unlink a Discord User from their in-game player",
		},
		Arguments: &i18n.Message{
			ID:    "commands.AllCommands.Unlink.args",
			Other: "<discord User>",
		},
		Aliases:    []string{"un", "ul", "u"},
		IsSecret:   false,
		Emoji:      "🚷",
		IsAdmin:    false,
		IsOperator: true,
	},
	{
		CommandType: UnmuteAll,
		Command:     "unmuteall",
		Example:     "unmuteall",
		ShortDesc: &i18n.Message{
			ID:    "commands.AllCommands.UnmuteAll.shortDesc",
			Other: "Force the bot to unmute all",
		},
		Description: &i18n.Message{
			ID:    "commands.AllCommands.UnmuteAll.desc",
			Other: "Force the bot to unmute all linked players",
		},
		Arguments: &i18n.Message{
			ID:    "commands.AllCommands.UnmuteAll.args",
			Other: "None",
		},
		Aliases:    []string{"unmute", "ua"},
		IsSecret:   false,
		Emoji:      "🔊",
		IsAdmin:    false,
		IsOperator: true,
	},
	{
		CommandType: Force,
		Command:     "force",
		Example:     "force task",
		ShortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Force.shortDesc",
			Other: "Force the bot to transition",
		},
		Description: &i18n.Message{
			ID:    "commands.AllCommands.Force.desc",
			Other: "Force the bot to transition to another game stage, if it doesn't transition properly",
		},
		Arguments: &i18n.Message{
			ID:    "commands.AllCommands.Force.args",
			Other: "<phase name> (task, discuss, or lobby / t,d, or l)",
		},
		Aliases:    []string{"f"},
		IsSecret:   true, // force is broken rn, so hide it
		Emoji:      "📢",
		IsAdmin:    false,
		IsOperator: true,
	},
	{
		CommandType: Map,
		Command:     "map",
		Example:     "map skeld",
		ShortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Map.shortDesc",
			Other: "Display an in-game map",
		},
		Description: &i18n.Message{
			ID:    "commands.AllCommands.Map.desc",
			Other: "Display an image of an in-game map in the text channel. Two supported versions: simple or detailed",
		},
		Arguments: &i18n.Message{
			ID:    "commands.AllCommands.Map.args",
			Other: "<map_name> (skeld, mira_hq, polus) <version> (optional, simple or detailed)",
		},
		Aliases:    []string{"map"},
		IsSecret:   false,
		Emoji:      "🗺",
		IsAdmin:    false,
		IsOperator: false,
	},
	{
		CommandType: Cache,
		Command:     "cache",
		Example:     "cache @Soup",
		ShortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Cache.shortDesc",
			Other: "View cached usernames",
		},
		Description: &i18n.Message{
			ID:    "commands.AllCommands.Cache.desc",
			Other: "View a player's cached in-game names, and/or clear them",
		},
		Arguments: &i18n.Message{
			ID:    "commands.AllCommands.Cache.args",
			Other: "<player> (optionally, \"clear\")",
		},
		Aliases:    []string{"c"},
		IsSecret:   false,
		Emoji:      "📖",
		IsAdmin:    false,
		IsOperator: true,
	},
	{
		CommandType: Privacy,
		Command:     "privacy",
		Example:     "privacy showme",
		ShortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Privacy.shortDesc",
			Other: "View AutoMuteUs privacy information",
		},
		Description: &i18n.Message{
			ID:    "commands.AllCommands.Privacy.desc",
			Other: "AutoMuteUs privacy and data collection details.\nMore details [here](https://github.com/denverquane/automuteus/blob/master/PRIVACY.md)",
		},
		Arguments: &i18n.Message{
			ID:    "commands.AllCommands.Privacy.args",
			Other: "showme, optin, or optout",
		},
		Aliases:    []string{"private", "priv", "gpdr"},
		IsSecret:   false,
		Emoji:      "🔍",
		IsAdmin:    false,
		IsOperator: false,
	},
	{
		CommandType: Settings,
		Command:     "settings",
		Example:     "settings commandPrefix !",
		ShortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Settings.shortDesc",
			Other: "Adjust bot settings",
		},
		Description: &i18n.Message{
			ID:    "commands.AllCommands.Settings.desc",
			Other: "Adjust the bot settings. Type `{{.CommandPrefix}} settings` with no arguments to see more.",
		},
		Arguments: &i18n.Message{
			ID:    "commands.AllCommands.Settings.args",
			Other: "<setting> <value>",
		},
		Aliases:    []string{"sett", "set", "s"},
		IsSecret:   false,
		Emoji:      "🛠",
		IsAdmin:    true,
		IsOperator: true,
	},
	{
		CommandType: Premium,
		Command:     "premium",
		Example:     "premium",
		ShortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Premium.shortDesc",
			Other: "View Premium Bot Features",
		},
		Description: &i18n.Message{
			ID:    "commands.AllCommands.Premium.desc",
			Other: "View all the features and perks of Premium AutoMuteUs membership",
		},
		Arguments: &i18n.Message{
			ID:    "commands.AllCommands.Premium.args",
			Other: "None",
		},
		Aliases:    []string{"donate", "paypal", "prem", "$"},
		IsSecret:   false,
		Emoji:      "💎",
		IsAdmin:    false,
		IsOperator: false,
	},
	{
		CommandType: Stats,
		Command:     "stats",
		Example:     "stats @Soup",
		ShortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Stats.shortDesc",
			Other: "View Player and Guild stats",
		},
		Description: &i18n.Message{
			ID:    "commands.AllCommands.Stats.desc",
			Other: "View Player and Guild stats",
		},
		Arguments: &i18n.Message{
			ID:    "commands.AllCommands.Stats.args",
			Other: "<@discord user> or \"guild\"",
		},
		Aliases:    []string{"stat", "st"},
		IsSecret:   false,
		Emoji:      "📊",
		IsAdmin:    false,
		IsOperator: false,
	},
	{
		CommandType: Info,
		Command:     "info",
		Example:     "info",
		ShortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Info.shortDesc",
			Other: "View Bot info",
		},
		Description: &i18n.Message{
			ID:    "commands.AllCommands.Info.desc",
			Other: "View info about the bot, like total guild number, active games, etc",
		},
		Arguments: &i18n.Message{
			ID:    "commands.AllCommands.Info.args",
			Other: "None",
		},
		Aliases:    []string{"info", "inf", "in", "i"},
		IsSecret:   false,
		Emoji:      "📰",
		IsAdmin:    false,
		IsOperator: false,
	},
	{
		CommandType: ASCII,
		Command:     "ascii",
		Example:     "ascii @Soup t 10",
		ShortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Ascii.shortDesc",
			Other: "Print an ASCII crewmate",
		},
		Description: &i18n.Message{
			ID:    "commands.AllCommands.Ascii.desc",
			Other: "Print an ASCII crewmate",
		},
		Arguments: &i18n.Message{
			ID:    "commands.AllCommands.Ascii.args",
			Other: "<@discord user> <is imposter> (true|false) <x impostor remains> (count)",
		},
		Aliases:    []string{"ascii", "asc"},
		IsSecret:   true,
		IsAdmin:    false,
		IsOperator: false,
	},
	{
		CommandType: DebugState,
		Command:     "debugstate",
		Example:     "debugstate",
		ShortDesc: &i18n.Message{
			ID:    "commands.AllCommands.DebugState.shortDesc",
			Other: "View the full state of the Discord Guild Data",
		},
		Description: &i18n.Message{
			ID:    "commands.AllCommands.DebugState.desc",
			Other: "View the full state of the Discord Guild Data",
		},
		Arguments: &i18n.Message{
			ID:    "commands.AllCommands.DebugState.args",
			Other: "None",
		},
		Aliases:    []string{"debug", "ds", "state"},
		IsSecret:   true,
		IsAdmin:    false,
		IsOperator: true,
	},
	{
		CommandType: Null,
		Command:     "",
		Example:     "",
		ShortDesc:   nil,
		Description: nil,
		Arguments:   nil,
		Aliases:     []string{""},
		IsSecret:    true,
		IsAdmin:     true,
		IsOperator:  true,
	},
}

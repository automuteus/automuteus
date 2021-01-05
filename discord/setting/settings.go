package setting

import "github.com/nicksnyder/go-i18n/v2/i18n"

type SettingType int

const (
	Prefix SettingType = iota
	Language
	AdminUserIDs
	RoleIDs
	UnmuteDead
	Delays
	VoiceRules
	MapVersion
	MatchSummary
	MatchSummaryChannel
	AutoRefresh
	LeaderboardMention
	LeaderboardSize
	LeaderboardMin
	MuteSpectators
	Show
	Reset
	NullSetting
)

type Setting struct {
	SettingType SettingType
	Name        string
	Example     string
	ShortDesc   *i18n.Message
	Description *i18n.Message
	Arguments   *i18n.Message
	Aliases     []string
	Premium     bool
}

var AllSettings = []Setting{
	{
		SettingType: Prefix,
		Name:        "commandPrefix",
		Example:     "commandPrefix !",
		ShortDesc: &i18n.Message{
			ID:    "settings.AllSettings.Prefix.shortDesc",
			Other: "Bot Prefix",
		},
		Description: &i18n.Message{
			ID:    "settings.AllSettings.Prefix.desc",
			Other: "Change the prefix that the bot uses to detect commands",
		},
		Arguments: &i18n.Message{
			ID:    "settings.AllSettings.Prefix.args",
			Other: "<prefix>",
		},
		Aliases: []string{"prefix", "pref", "cp"},
		Premium: false,
	},
	{
		SettingType: Language,
		Name:        "language",
		Example:     "language ru",
		ShortDesc: &i18n.Message{
			ID:    "settings.AllSettings.Language.shortDesc",
			Other: "Bot Language",
		},
		Description: &i18n.Message{
			ID:    "settings.AllSettings.Language.desc",
			Other: "Change the bot messages language",
		},
		Arguments: &i18n.Message{
			ID:    "settings.AllSettings.Language.args",
			Other: "<language> or reload",
		},
		Aliases: []string{"local", "lang", "l"},
		Premium: false,
	},
	{
		SettingType: AdminUserIDs,
		Name:        "adminUserIDs",
		Example:     "adminUserIDs @Soup @Bob",
		ShortDesc: &i18n.Message{
			ID:    "settings.AllSettings.AdminUserIDs.shortDesc",
			Other: "Bot Admins",
		},
		Description: &i18n.Message{
			ID:    "settings.AllSettings.AdminUserIDs.desc",
			Other: "Specify which individual users have admin bot permissions",
		},
		Arguments: &i18n.Message{
			ID:    "settings.AllSettings.AdminUserIDs.args",
			Other: "<User @ mentions>...",
		},
		Aliases: []string{"admins", "admin", "auid", "aui", "a"},
		Premium: false,
	},
	{
		SettingType: RoleIDs,
		Name:        "operatorRoles",
		Example:     "operatorRoles @Bot Admins @Bot Mods",
		ShortDesc: &i18n.Message{
			ID:    "settings.AllSettings.RoleIDs.shortDesc",
			Other: "Bot Operators",
		},
		Description: &i18n.Message{
			ID:    "settings.AllSettings.RoleIDs.desc",
			Other: "Specify which roles have permissions to invoke the bot",
		},
		Arguments: &i18n.Message{
			ID:    "settings.AllSettings.RoleIDs.args",
			Other: "<role @ mentions>...",
		},
		Aliases: []string{"operators", "operator", "oproles", "roles", "role", "ops", "op"},
		Premium: false,
	},
	{
		SettingType: UnmuteDead,
		Name:        "unmuteDeadDuringTasks",
		Example:     "unmuteDeadDuringTasks false",
		ShortDesc: &i18n.Message{
			ID:    "settings.AllSettings.UnmuteDead.shortDesc",
			Other: "Bot Unmutes Deaths",
		},
		Description: &i18n.Message{
			ID:    "settings.AllSettings.UnmuteDead.desc",
			Other: "Specify if the bot should immediately unmute players when they die. **CAUTION. Leaks information!**",
		},
		Arguments: &i18n.Message{
			ID:    "settings.AllSettings.UnmuteDead.args",
			Other: "<true/false>",
		},
		Aliases: []string{"unmutedead", "unmute", "uddt", "ud"},
		Premium: false,
	},
	{
		SettingType: Delays,
		Name:        "delays",
		Example:     "delays lobby tasks 5",
		ShortDesc: &i18n.Message{
			ID:    "settings.AllSettings.Delays.shortDesc",
			Other: "Delays Between Stages",
		},
		Description: &i18n.Message{
			ID:    "settings.AllSettings.Delays.desc",
			Other: "Specify the delays for automute/deafen between stages of the game, like lobby->tasks",
		},
		Arguments: &i18n.Message{
			ID:    "settings.AllSettings.Delays.args",
			Other: "<start phase> <end phase> <delay>",
		},
		Aliases: []string{"delays", "d"},
		Premium: false,
	},
	{
		SettingType: VoiceRules,
		Name:        "voiceRules",
		Example:     "voiceRules mute tasks dead true",
		ShortDesc: &i18n.Message{
			ID:    "settings.AllSettings.VoiceRules.shortDesc",
			Other: "Mute/deafen Rules",
		},
		Description: &i18n.Message{
			ID:    "settings.AllSettings.VoiceRules.desc",
			Other: "Specify mute/deafen rules for the game, depending on the stage and the alive/deadness of players. Example given would mute dead players during the tasks stage",
		},
		Arguments: &i18n.Message{
			ID:    "settings.AllSettings.VoiceRules.args",
			Other: "<mute/deaf> <game phase> <dead/alive> <true/false>",
		},
		Aliases: []string{"voice", "vr"},
		Premium: false,
	},
	{
		SettingType: MapVersion,
		Name:        "mapVersion",
		Example:     "mapVersion detailed",
		ShortDesc: &i18n.Message{
			ID:    "settings.AllSettings.MapVersion.shortDesc",
			Other: "Map version",
		},
		Description: &i18n.Message{
			ID:    "settings.AllSettings.MapVersion.desc",
			Other: "Specify the default map version (simple, detailed) used by 'map' command",
		},
		Arguments: &i18n.Message{
			ID:    "settings.AllSettings.MapVersion.args",
			Other: "<version>",
		},
		Aliases: []string{"map"},
		Premium: false,
	},
	{
		SettingType: MatchSummary,
		Name:        "matchSummary",
		Example:     "matchSummary 5",
		ShortDesc: &i18n.Message{
			ID:    "settings.AllSettings.MatchSummary.shortDesc",
			Other: "Match Summary Message",
		},
		Description: &i18n.Message{
			ID:    "settings.AllSettings.MatchSummary.desc",
			Other: "Specify minutes before the match summary message is deleted. 0 for instant deletion, -1 for never delete",
		},
		Arguments: &i18n.Message{
			ID:    "settings.AllSettings.MatchSummary.args",
			Other: "<minutes>",
		},
		Aliases: []string{"matchsumm", "matchsum", "summary", "match", "summ", "sum"},
		Premium: true,
	},
	{
		SettingType: MatchSummaryChannel,
		Name:        "matchSummaryChannel",
		Example:     "matchSummaryChannel general",
		ShortDesc: &i18n.Message{
			ID:    "settings.AllSettings.MatchSummaryChannel.shortDesc",
			Other: "Channel for Match Summaries",
		},
		Description: &i18n.Message{
			ID:    "settings.AllSettings.MatchSummaryChannel.desc",
			Other: "Specify the text channel name where Match Summaries should be posted",
		},
		Arguments: &i18n.Message{
			ID:    "settings.AllSettings.MatchSummaryChannel.args",
			Other: "<text channel name>",
		},
		Aliases: []string{"matchsummchan", "matchsumchan", "summarychannel", "matchchannel", "summchan", "sumchan"},
		Premium: true,
	},
	{
		SettingType: AutoRefresh,
		Name:        "autoRefresh",
		Example:     "autoRefresh true",
		ShortDesc: &i18n.Message{
			ID:    "settings.AllSettings.AutoRefresh.shortDesc",
			Other: "Autorefresh Status Message",
		},
		Description: &i18n.Message{
			ID:    "settings.AllSettings.AutoRefresh.desc",
			Other: "Specify if the bot should auto-refresh the status message after a match ends",
		},
		Arguments: &i18n.Message{
			ID:    "settings.AllSettings.AutoRefresh.args",
			Other: "<true/false>",
		},
		Aliases: []string{"refresh", "auto", "ar"},
		Premium: true,
	},
	{
		SettingType: LeaderboardMention,
		Name:        "leaderboardMention",
		Example:     "leaderboardMention true",
		ShortDesc: &i18n.Message{
			ID:    "settings.AllSettings.LeaderboardMention.shortDesc",
			Other: "Player Leaderboard Mention Format",
		},
		Description: &i18n.Message{
			ID:    "settings.AllSettings.LeaderboardMention.desc",
			Other: "If players should be mentioned with @ on the leaderboard.\n**Disable this for large servers!**",
		},
		Arguments: &i18n.Message{
			ID:    "settings.AllSettings.LeaderboardMention.args",
			Other: "<true/false>",
		},
		Aliases: []string{"lboardmention", "leadermention", "mention", "ment"},
		Premium: true,
	},
	{
		SettingType: LeaderboardSize,
		Name:        "leaderboardSize",
		Example:     "leaderboardSize 5",
		ShortDesc: &i18n.Message{
			ID:    "settings.AllSettings.LeaderboardSize.shortDesc",
			Other: "Player Leaderboard Size",
		},
		Description: &i18n.Message{
			ID:    "settings.AllSettings.LeaderboardSize.desc",
			Other: "Specify the size of the player leaderboard",
		},
		Arguments: &i18n.Message{
			ID:    "settings.AllSettings.LeaderboardSize.args",
			Other: "<number>",
		},
		Aliases: []string{"lboardsize", "boardsize", "leadersize", "size"},
		Premium: true,
	},
	{
		SettingType: LeaderboardMin,
		Name:        "leaderboardMin",
		Example:     "leaderboardMin 3",
		ShortDesc: &i18n.Message{
			ID:    "settings.AllSettings.LeaderboardMin.shortDesc",
			Other: "Minimum Games for Leaderboard",
		},
		Description: &i18n.Message{
			ID:    "settings.AllSettings.LeaderboardMin.desc",
			Other: "Minimum amount of games before a player is displayed on the leaderboard",
		},
		Arguments: &i18n.Message{
			ID:    "settings.AllSettings.LeaderboardMin.args",
			Other: "<number>",
		},
		Aliases: []string{"lboardmin", "boardmin", "leadermin", "min"},
		Premium: true,
	},
	{
		SettingType: MuteSpectators,
		Name:        "muteSpectators",
		Example:     "muteSpectators true",
		ShortDesc: &i18n.Message{
			ID:    "settings.AllSettings.MuteSpectators.shortDesc",
			Other: "Mute Spectators like Dead Players",
		},
		Description: &i18n.Message{
			ID:    "settings.AllSettings.MuteSpectators.desc",
			Other: "Whether or not the bot should treat spectators like dead players (respecting your voice rules).\n**Note, this can cause delays or slowdowns when not self-hosting, or using a Premium worker bot!**",
		},
		Arguments: &i18n.Message{
			ID:    "settings.AllSettings.MuteSpectators.args",
			Other: "<true/false>",
		},
		Aliases: []string{"mutespectator", "mutespec", "spectators", "spectator", "spec"},
		Premium: true,
	},
	{
		SettingType: Show,
		Name:        "show",
		Example:     "show",
		ShortDesc: &i18n.Message{
			ID:    "settings.AllSettings.Show.shortDesc",
			Other: "Show All Settings",
		},
		Description: &i18n.Message{
			ID:    "settings.AllSettings.Show.desc",
			Other: "Show all the Bot settings for this server",
		},
		Arguments: &i18n.Message{
			ID:    "settings.AllSettings.Show.args",
			Other: "None",
		},
		Aliases: []string{"sh", "s"},
		Premium: false,
	},
	{
		SettingType: Reset,
		Name:        "reset",
		Example:     "reset",
		ShortDesc: &i18n.Message{
			ID:    "settings.AllSettings.Reset.shortDesc",
			Other: "Reset Bot Settings",
		},
		Description: &i18n.Message{
			ID:    "settings.AllSettings.Reset.desc",
			Other: "Reset all bot settings to their default values",
		},
		Arguments: &i18n.Message{
			ID:    "settings.AllSettings.Reset.args",
			Other: "None",
		},
		Aliases: []string{},
		Premium: false,
	},
}

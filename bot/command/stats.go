package command

import (
	"strconv"
	"strings"

	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const Guild = "guild"

// Stats no longer shows or clears anything itself: stats are viewed, and reset, on the web dashboard, and this
// command hands out the link to this server's page there.
var Stats = discordgo.ApplicationCommand{
	Name:        "stats",
	Description: "Get a link to the web dashboard, where stats from games played on this server are shown",
}

// StatsURL is the dashboard page for one guild's stats. webURL is the bot's WEB_URL, with or without a trailing
// slash; the page itself still requires the visitor to be a member of the guild.
func StatsURL(webURL, guildID string) string {
	return strings.TrimRight(webURL, "/") + "/stats?guild=" + guildID
}

// MatchURL is the dashboard page for one recorded match of a guild, or "" for a game that was never recorded.
func MatchURL(webURL, guildID string, matchID int64) string {
	if matchID <= 0 {
		return ""
	}
	return strings.TrimRight(webURL, "/") + "/stats/match?guild=" + guildID + "&match=" + strconv.FormatInt(matchID, 10)
}

// StatsResponse is the private reply to /stats: a short explanation plus a link button to the dashboard.
func StatsResponse(webURL, guildID string, sett *settings.GuildSettings) *discordgo.InteractionResponse {
	url := StatsURL(webURL, guildID)
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
			Content: sett.LocalizeMessage(&i18n.Message{
				ID: "commands.stats.dashboard",
				Other: "Stats for this server are on the web dashboard. Sign in with Discord to see the leaderboards, " +
					"any player's stats and match details, or to reset your own stats.\n<{{.URL}}>",
			}, map[string]interface{}{
				"URL": url,
			}),
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Style: discordgo.LinkButton,
							Label: sett.LocalizeMessage(&i18n.Message{
								ID:    "commands.stats.open",
								Other: "Open stats",
							}),
							URL: url,
						},
					},
				},
			},
		},
	}
}

package command

import (
	"fmt"
	"github.com/automuteus/automuteus/v8/bot/tokenprovider"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// TODO update localization keys

var emojiNums = []string{":one:", ":two:", ":three:"}

const basePremiumURL = "https://automute.us/premium?guild="

// if you're reading this, adding these bots won't help you.
// Galactus+AutoMuteUs verify the premium status internally before using these bots ;)
var premiumBotIDs = []string{
	"780323275624546304", // amu1 (verified, so it has no server limit)
	"769022114229125181", // amu2
	"780323801173983262", // amu3
	"780589033033302036", // amu4
	"780589278195220480", // amu5
}

// BotStatus reports whether the bot with the given user ID is already in the server, and whether it is in as many
// servers as Discord allows it, so it can't be invited anywhere new. Unknown is reported as neither.
type BotStatus func(botID string) (member, full bool)

// OfferedBot is one of the priority mute bots a server is offered.
type OfferedBot struct {
	ID     string
	Member bool
}

// OfferedBots picks the priority mute bots offered to a server with tier, up to its allowance: the bots already in
// the server first, then the first bots in premiumBotIDs that still have room. Fewer than the allowance are offered
// when too many bots are full.
func OfferedBots(tier premium.Tier, status BotStatus) (offered []OfferedBot, allowance int) {
	allowance = tokenprovider.PremiumBotConstraints[tier]
	if tier != premium.SilverTier && tier != premium.GoldTier {
		return nil, 0
	}
	if status == nil {
		status = func(string) (bool, bool) { return false, false }
	}
	members, invitable := []OfferedBot{}, []OfferedBot{}
	for _, id := range premiumBotIDs {
		switch member, full := status(id); {
		case member:
			members = append(members, OfferedBot{ID: id, Member: true})
		case !full:
			invitable = append(invitable, OfferedBot{ID: id})
		}
	}
	offered = append(members, invitable...)
	return offered[:min(len(offered), allowance)], allowance
}

func botInviteURL(botID string) string {
	return "https://discord.com/api/oauth2/authorize?client_id=" + botID + "&permissions=12582912&scope=bot"
}

const (
	PremiumInfo    string = "info"
	PremiumInvites string = "invites"
)

// TODO transfer functionality
// TODO "add another gold server" functionality
// TODO cancel functionality? This is harder/needs Paypal hooks
var Premium = discordgo.ApplicationCommand{
	Name:        "premium",
	Description: "View information about AutoMuteUs Premium",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Name:        PremiumInfo,
			Description: "View AutoMuteUs Premium information",
			Type:        discordgo.ApplicationCommandOptionSubCommand,
		},
		{
			Name:        PremiumInvites,
			Description: "Invite AutoMuteUs workers",
			Type:        discordgo.ApplicationCommandOptionSubCommand,
		},
	},
}

func GetPremiumParams(options []*discordgo.ApplicationCommandInteractionDataOption) string {
	return options[0].Name
}

func PremiumResponse(guildID string, tier premium.Tier, daysRem int, arg string, isAdmin bool, status BotStatus, sett *settings.GuildSettings) *discordgo.InteractionResponse {
	var embed *discordgo.MessageEmbed
	if arg == PremiumInvites {
		if !isAdmin {
			return InsufficientPermissionsResponse(sett)
		}
		embed = invitesResponse(tier, status, sett)
	} else {
		embed = premiumEmbedResponse(guildID, tier, daysRem, sett)
	}
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{
				embed,
			},
		},
	}

}

func invitesResponse(tier premium.Tier, status BotStatus, sett *settings.GuildSettings) *discordgo.MessageEmbed {
	desc := ""
	var fields []*discordgo.MessageEmbedField

	if tier == premium.FreeTier || tier == premium.BronzeTier {
		desc = sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.premiumInviteResponseNoAccess.desc",
			Other: "{{.Tier}} users don't have access to Priority mute bots!\nPlease type `/premium` to see more details about AutoMuteUs Premium",
		}, map[string]interface{}{
			"Tier": premium.TierStrings[tier],
		})
	} else {
		offered, allowance := OfferedBots(tier, status)
		desc = sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.premiumInviteResponse.desc",
			Other: "{{.Tier}} users have access to {{.Count}} Priority mute bots: invites provided below!",
		}, map[string]interface{}{
			"Tier":  premium.TierStrings[tier],
			"Count": allowance,
		})

		for i, bot := range offered {
			value := fmt.Sprintf("[Invite Me](%s)", botInviteURL(bot.ID))
			if bot.Member {
				value = sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.premiumInviteResponse.member",
					Other: "✅ Already in this server",
				})
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   fmt.Sprintf("Bot %s", emojiNums[i]),
				Value:  value,
				Inline: false,
			})
		}
		if len(offered) < allowance {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.premiumInviteResponse.full.title",
					Other: "Bots at capacity",
				}),
				Value: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.premiumInviteResponse.full",
					Other: "Some Priority mute bots have reached Discord's server limit and can't be invited right now. Please check back later!",
				}),
				Inline: false,
			})
		}
	}
	msg := discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.premiumInviteResponse.Title",
			Other: "Premium Bot Invites",
		}),
		Description: desc,
		Timestamp:   time.Now().Format(ISO8601),
		Color:       10181046, // PURPLE
		Footer:      nil,
		Image:       nil,
		Thumbnail:   nil,
		Video:       nil,
		Provider:    nil,
		Author:      nil,
		Fields:      fields,
	}
	return &msg
}

func premiumEmbedResponse(guildID string, tier premium.Tier, daysRem int, sett *settings.GuildSettings) *discordgo.MessageEmbed {
	desc := ""
	var fields []*discordgo.MessageEmbedField

	if tier != premium.FreeTier {
		if daysRem > 0 || daysRem == premium.NoExpiryCode {
			daysRemStr := ""
			if daysRem > 0 {
				daysRemStr = sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.premiumResponse.PremiumDescriptionDaysRemaining",
					Other: " for another {{.Days}} days",
				},
					map[string]interface{}{
						"Days": daysRem,
					})
			}
			desc = sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.premiumResponse.PremiumDescription",
				Other: "Looks like you have AutoMuteUs **{{.Tier}}**{{.DaysString}}! Thanks for the support!\n\nBelow are some of the benefits you can customize with your Premium status!",
			},
				map[string]interface{}{
					"Tier":       premium.TierStrings[tier],
					"DaysString": daysRemStr,
				})

			fields = []*discordgo.MessageEmbedField{
				{
					Name: "Bot Invites",
					Value: sett.LocalizeMessage(&i18n.Message{
						ID:    "responses.premiumResponse.Invites",
						Other: "View a list of Premium bots you can invite with `/premium invites`!",
					}),
					Inline: false,
				},
				{
					Name: "Premium Settings",
					Value: sett.LocalizeMessage(&i18n.Message{
						ID:    "responses.premiumResponse.SettingsDescExtra",
						Other: "Look for the settings marked with 💎 under `/settings list`!",
					}),
					Inline: false,
				},
			}
		} else {
			desc = sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.premiumResponse.PremiumDescriptionExpired",
				Other: "Oh no! It looks like you used to have AutoMuteUs **{{.Tier}}**, but it **expired {{.Days}} days ago**! 😦\n\nPlease consider re-subscribing here: [Get AutoMuteUs Premium]({{.BaseURL}}{{.GuildID}})",
			},
				map[string]interface{}{
					"Tier":    premium.TierStrings[tier],
					"Days":    0 - daysRem,
					"BaseURL": BasePremiumURL,
					"GuildID": guildID,
				})
		}
	} else {
		desc = sett.LocalizeMessage(&i18n.Message{
			ID: "responses.premiumResponse.FreeDescription",
			Other: "Check out the cool things that Premium AutoMuteUs has to offer!\n\n" +
				"[Get AutoMuteUs Premium]({{.BaseURL}}{{.GuildID}})\n",
		}, map[string]interface{}{
			"BaseURL": BasePremiumURL,
			"GuildID": guildID,
		})
		fields = []*discordgo.MessageEmbedField{
			{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.premiumResponse.PriorityGameAccess",
					Other: "👑 Priority Game Access",
				}),
				Value: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.premiumResponse.PriorityGameAccessDesc",
					Other: "If the Bot is under heavy load, Premium users will always be able to make new games!",
				}),
				Inline: false,
			},
			{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.premiumResponse.FastMute",
					Other: "🙊 Fast Mute/Deafen",
				}),
				Value: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.premiumResponse.FastMuteDesc",
					Other: "Premium users get access to \"helper\" bots that make sure muting is fast!",
				}),
				Inline: false,
			},
			{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.premiumResponse.Stats",
					Other: "📊 Game Stats and Leaderboards",
				}),
				Value: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.premiumResponse.StatsDesc",
					Other: "Premium users have access to a full suite of player stats and leaderboards!",
				}),
				Inline: false,
			},
			{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.premiumResponse.Settings",
					Other: "🛠 Special Settings",
				}),
				Value: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.premiumResponse.SettingsDesc",
					Other: "Premium users can specify additional settings, like displaying an end-game status message, or auto-refreshing the status message!",
				}),
				Inline: false,
			},
		}
	}

	msg := discordgo.MessageEmbed{
		URL:  basePremiumURL + guildID,
		Type: "",
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.premiumResponse.Title",
			Other: "💎 AutoMuteUs Premium 💎",
		}),
		Description: desc,
		Timestamp:   time.Now().Format(ISO8601),
		Color:       10181046, // PURPLE
		Footer:      nil,
		Image:       nil,
		Thumbnail:   nil,
		Video:       nil,
		Provider:    nil,
		Author:      nil,
		Fields:      fields,
	}
	return &msg
}

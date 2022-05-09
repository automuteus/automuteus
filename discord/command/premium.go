package command

import (
	"fmt"
	"github.com/automuteus/utils/pkg/premium"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"time"
)

// TODO update localization keys

var emojiNums = []string{":one:", ":two:", ":three:"}

const basePremiumURL = "https://automute.us/premium?guild="

// if you're reading this, adding these bots won't help you.
// Galactus+AutoMuteUs verify the premium status internally before using these bots ;)
var botInvites = []string{
	"https://discord.com/api/oauth2/authorize?client_id=780323275624546304&permissions=12582912&scope=bot", // amu1
	"https://discord.com/api/oauth2/authorize?client_id=780589033033302036&permissions=12582912&scope=bot", // amu4
	"https://discord.com/api/oauth2/authorize?client_id=780589278195220480&permissions=12582912&scope=bot"} // amu5

const (
	PremiumInfo    string = "info"
	PremiumInvites        = "invites"
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

func PremiumResponse(guildID string, tier premium.Tier, daysRem int, arg string, isAdmin bool, sett *settings.GuildSettings) *discordgo.InteractionResponse {
	var embed *discordgo.MessageEmbed
	if arg == PremiumInvites {
		if !isAdmin {
			return InsufficientPermissionsResponse(sett)
		}
		embed = invitesResponse(tier, sett)
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

func invitesResponse(tier premium.Tier, sett *settings.GuildSettings) *discordgo.MessageEmbed {
	desc := ""
	var fields []*discordgo.MessageEmbedField

	if tier == premium.FreeTier || tier == premium.BronzeTier || tier == premium.TrialTier {
		desc = sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.premiumInviteResponseNoAccess.desc",
			Other: "{{.Tier}} users don't have access to Priority mute bots!\nPlease type `/premium` to see more details about AutoMuteUs Premium",
		}, map[string]interface{}{
			"Tier": premium.TierStrings[tier],
		})
	} else {
		count := 0
		if tier == premium.SilverTier {
			count = 1
		} else if tier == premium.GoldTier {
			count = 3
		}
		desc = sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.premiumInviteResponse.desc",
			Other: "{{.Tier}} users have access to {{.Count}} Priority mute bots: invites provided below!",
		}, map[string]interface{}{
			"Tier":  premium.TierStrings[tier],
			"Count": count,
		})

		for i := 0; i < count; i++ {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   fmt.Sprintf("Bot %s", emojiNums[i]),
				Value:  fmt.Sprintf("[Invite Me](%s)", botInvites[i]),
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

	if tier != premium.FreeTier && tier != premium.TrialTier {
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
		if tier == premium.TrialTier {
			desc += sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.premiumResponse.Trial",
				Other: "You're currently on a TRIAL of AutoMuteUs Premium\n\n",
			})
		} else {
			desc += sett.LocalizeMessage(&i18n.Message{
				ID: "responses.premiumResponse.TopGG",
				Other: "or\n[Vote for the Bot on top.gg](https://top.gg/bot/753795015830011944) for 12 Hours of Free Premium!\n" +
					"(One time per user)\n\n",
			})
		}
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

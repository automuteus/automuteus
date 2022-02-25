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
	"https://discord.com/api/oauth2/authorize?client_id=780323801173983262&permissions=12582912&scope=bot"} // amu3

type PremiumArg int

const (
	PremiumInfo PremiumArg = iota
	PremiumInvites
	PremiumNil
)

const (
	Invites string = "invites"
)

var Premium = discordgo.ApplicationCommand{
	Name:        "premium",
	Description: "View information about AutoMuteUs Premium",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Name:        "argument",
			Description: "Premium argument",
			Type:        discordgo.ApplicationCommandOptionInteger,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{
					Name:  Invites,
					Value: PremiumInvites,
				},
			},
			Required: false,
		},
	},
}

func GetPremiumParams(options []*discordgo.ApplicationCommandInteractionDataOption) PremiumArg {
	if len(options) == 0 {
		return PremiumInfo
	}
	return PremiumArg(options[0].IntValue())
}

func PremiumResponse(guildID string, tier premium.Tier, daysRem int, arg PremiumArg, sett *settings.GuildSettings) *discordgo.InteractionResponse {
	var embed *discordgo.MessageEmbed
	if arg == PremiumInvites {
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

	if tier == premium.FreeTier || tier == premium.BronzeTier {
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
		} else if tier == premium.GoldTier || tier == premium.PlatTier {
			count = 3
		}
		// TODO account for Platinum
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
				Value:  fmt.Sprintf("[Invite](%s)", botInvites[i]),
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
						Other: "View a list of Premium bots you can invite with `{{.CommandPrefix}} premium invites`!",
					}, map[string]interface{}{
						"CommandPrefix": sett.GetCommandPrefix(),
					}),
					Inline: false,
				},
				{
					Name: "Premium Settings",
					Value: sett.LocalizeMessage(&i18n.Message{
						ID:    "responses.premiumResponse.SettingsDescExtra",
						Other: "Look for the settings marked with 💎 under `{{.CommandPrefix}} settings!`",
					}, map[string]interface{}{
						"CommandPrefix": sett.GetCommandPrefix(),
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
				"[Get AutoMuteUs Premium]({{.BaseURL}}{{.GuildID}})",
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

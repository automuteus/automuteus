package discord

import (
	"fmt"
	"github.com/automuteus/automuteus/metrics"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"log"
)

const (
	MaxDebugMessageSize = 1980
	trueString          = "true"
)

func (bot *Bot) HandleCommand(
	isAdmin bool,
	isPermissioned bool,
	sett *settings.GuildSettings,
	session *discordgo.Session,
	guild *discordgo.Guild,
	message *discordgo.MessageCreate,
	args []string,
) bool {
	if len(args) == 0 {
		return false
	}
	command, exists := getCommand(args[0])

	if !exists {
		log.Print(fmt.Sprintf("\"%s\" command typed by User %s\n", command.Command, message.Author.ID))
		session.ChannelMessageSend(
			message.ChannelID,
			sett.LocalizeMessage(
				&i18n.Message{
					ID:    "commands.HandleCommand.default",
					Other: "Sorry, I didn't understand `{{.InvalidCommand}}`! Please see `{{.CommandPrefix}} help` for commands",
				},
				map[string]interface{}{
					"CommandPrefix":  sett.CommandPrefix,
					"InvalidCommand": args[0],
				},
			),
		)
		return false
	}

	if command.IsAdmin && !isAdmin {
		session.ChannelMessageSend(message.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "message_handlers.handleMessageCreate.noPerms",
			Other: "User does not have the required permissions to execute this command!",
		}))
		return false
	}

	// admins can invoke moderator commands
	if command.IsOperator && (!isPermissioned && !isAdmin) {
		session.ChannelMessageSend(message.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "message_handlers.handleMessageCreate.noPerms",
			Other: "User does not have the required permissions to execute this command!",
		}))
		return false
	}

	msgsSent := int64(0)
	channelID, msgToSend := command.fn(bot, isAdmin, isPermissioned, sett, guild, message, args, &command)
	switch msg := msgToSend.(type) {
	case string:
		session.ChannelMessageSend(channelID, msg)
		msgsSent = 1
	case []string:
		for _, v := range msg {
			session.ChannelMessageSend(channelID, v)
			msgsSent++
		}
	case discordgo.MessageEmbed:
		embed := msg
		session.ChannelMessageSendEmbed(channelID, &embed)
		msgsSent = 1
	case *discordgo.MessageEmbed:
		session.ChannelMessageSendEmbed(channelID, msg)
		msgsSent = 1
	case nil:
		// do nothing
	default:
		log.Printf("Incapable of processing sendMessage of type: %T", msg)
	}
	metrics.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageCreateDelete, msgsSent)
	return true
}

package discord

import (
	"fmt"
	"github.com/automuteus/automuteus/amongus"
	"github.com/automuteus/utils/pkg/game"
	"github.com/bwmarrin/discordgo"
	"log"
	"strings"
)

func (bot *Bot) handleInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	lock := bot.RedisInterface.LockSnowflake(i.ID)
	// couldn't obtain lock; bail bail bail!
	if lock == nil {
		return
	}
	defer lock.Release(ctx)

	sett := bot.StorageInterface.GetGuildSettings(i.GuildID)

	switch i.ApplicationCommandData().Name {
	case "help":
		m := helpResponse(true, true, allCommands, sett)
		//if len(i.ApplicationCommandData().Options) > 0 {
		//	log.Println(i.ApplicationCommandData().Options[0].StringValue())
		//}
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{&m},
			},
		})
		if err != nil {
			log.Println(err)
		}
	case "info":
		m := bot.infoResponse(i.GuildID, sett)
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{m},
			},
		})
		if err != nil {
			log.Println(err)
		}
	case "link":
		user := i.ApplicationCommandData().Options[0].UserValue(s)
		if user == nil {
			log.Println("User is nil in call to link via slash interaction")
			return
		}
		colorOrName := strings.ReplaceAll(strings.ToLower(i.ApplicationCommandData().Options[1].StringValue()), " ", "")
		gsr := GameStateRequest{
			GuildID:     i.GuildID,
			TextChannel: i.ChannelID,
		}
		lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
		if lock == nil {
			log.Printf("No lock could be obtained when linking for guild %s, channel %s\n", i.GuildID, i.ChannelID)
			return
		}

		var auData amongus.PlayerData
		found := false
		if game.IsColorString(colorOrName) {
			auData, found = dgs.AmongUsData.GetByColor(colorOrName)
		} else {
			auData, found = dgs.AmongUsData.GetByName(colorOrName)
		}
		if found {
			foundID := dgs.AttemptPairingByUserIDs(auData, map[string]interface{}{user.ID: ""})
			if foundID != "" {
				log.Printf("Successfully linked %s to an in-game player\n", user.ID)
				err := bot.RedisInterface.AddUsernameLink(dgs.GuildID, user.ID, auData.Name)
				if err != nil {
					log.Println(err)
				}
			} else {
				log.Printf("No player was found with id %s\n", user.ID)
			}
			bot.RedisInterface.SetDiscordGameState(dgs, lock)
			// TODO refactor to return the edit, not perform it?
			dgs.Edit(bot.PrimarySession, bot.gameStateResponse(dgs, sett))
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags:   1 << 6, //private
					Content: "I've linked <@" + user.ID + "> to " + colorOrName + " successfully!",
				},
			})
		} else {
			// release the lock
			bot.RedisInterface.SetDiscordGameState(nil, lock)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: 1 << 6, //private
					Content: fmt.Sprintf("I couldn't find any data for the player <@%s>, "+
						"is the color or in-game name `%s` correct?", user.ID, colorOrName),
				},
			})
		}

	case "unlink":
		user := i.ApplicationCommandData().Options[0].UserValue(s)
		if user == nil {
			log.Println("User is nil in call to unlink via slash interaction")
			return
		}
		log.Print(fmt.Sprintf("Unlinking player %s", user.ID))
		gsr := GameStateRequest{
			GuildID:     i.GuildID,
			TextChannel: i.ChannelID,
		}
		lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
		if lock == nil {
			log.Printf("No lock could be obtained when unlinking for guild %s, channel %s\n", i.GuildID, i.ChannelID)
			return
		}
		// if we found the player and cleared their data
		if dgs.ClearPlayerData(user.ID) {
			bot.RedisInterface.SetDiscordGameState(dgs, lock)

			// TODO refactor to return the edit, not perform it?
			dgs.Edit(bot.PrimarySession, bot.gameStateResponse(dgs, sett))
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags:   1 << 6, //private
					Content: "I've unlinked <@" + user.ID + "> successfully!",
				},
			})
		} else {
			// release the lock
			bot.RedisInterface.SetDiscordGameState(nil, lock)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags:   1 << 6, //private
					Content: fmt.Sprintf("I couldn't find any data for the player <@%s>... Are they currently linked?", user.ID),
				},
			})
		}

	}
}

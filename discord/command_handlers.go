package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/denverquane/amongusdiscord/metrics"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/discord/command"
	"github.com/denverquane/amongusdiscord/storage"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const (
	MaxDebugMessageSize = 1980
	detailedMapString   = "detailed"
	clearArgumentString = "clear"
	trueString          = "true"
)

var MatchIDRegex = regexp.MustCompile(`^[A-Z0-9]{8}:[0-9]+$`)

// TODO cache/preconstruct these (no reason to make them fresh everytime help is called, except for the prefix...)
func ConstructEmbedForCommand(prefix string, cmd command.Command, sett *storage.GuildSettings) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		URL:   "",
		Type:  "",
		Title: cmd.Emoji + " " + strings.Title(cmd.Command),
		Description: sett.LocalizeMessage(cmd.Description,
			map[string]interface{}{
				"CommandPrefix": sett.CommandPrefix,
			}),
		Timestamp: "",
		Color:     15844367, // GOLD
		Image:     nil,
		Thumbnail: nil,
		Video:     nil,
		Provider:  nil,
		Author:    nil,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "commands.ConstructEmbedForCommand.Fields.Example",
					Other: "Example",
				}),
				Value:  "`" + fmt.Sprintf("%s %s", prefix, cmd.Example) + "`",
				Inline: false,
			},
			{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "commands.ConstructEmbedForCommand.Fields.Arguments",
					Other: "Arguments",
				}),
				Value:  "`" + sett.LocalizeMessage(cmd.Arguments) + "`",
				Inline: false,
			},
			{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "commands.ConstructEmbedForCommand.Fields.Aliases",
					Other: "Aliases",
				}),
				Value:  strings.Join(cmd.Aliases, ", "),
				Inline: false,
			},
		},
	}
}

func (bot *Bot) HandleCommand(isAdmin, isPermissioned bool, sett *storage.GuildSettings, s *discordgo.Session, g *discordgo.Guild, m *discordgo.MessageCreate, args []string) {
	prefix := sett.CommandPrefix
	cmd := command.GetCommand(args[0])

	gsr := GameStateRequest{
		GuildID:     m.GuildID,
		TextChannel: m.ChannelID,
	}

	if cmd.CommandType != command.Null {
		log.Print(fmt.Sprintf("\"%s\" command typed by User %s\n", cmd.Command, m.Author.ID))
	}

	switch {
	case cmd.IsAdmin && !isAdmin:
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "message_handlers.handleMessageCreate.noPerms",
			Other: "User does not have the required permissions to execute this command!",
		}))
	case cmd.IsOperator && (!isPermissioned && !isAdmin):
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "message_handlers.handleMessageCreate.noPerms",
			Other: "User does not have the required permissions to execute this command!",
		}))
	default:
		metrics.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageCreateDelete, 2)
		switch cmd.CommandType {
		case command.Help:
			if len(args[1:]) == 0 {
				embed := helpResponse(isAdmin, isPermissioned, prefix, command.AllCommands, sett)
				s.ChannelMessageSendEmbed(m.ChannelID, &embed)
			} else {
				cmd = command.GetCommand(args[1])
				if cmd.CommandType != command.Null {
					embed := ConstructEmbedForCommand(prefix, cmd, sett)
					s.ChannelMessageSendEmbed(m.ChannelID, embed)
				} else {
					s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
						ID:    "commands.HandleCommand.Help.notFound",
						Other: "I didn't recognize that command! View `help` for all available commands!",
					}))
				}
			}

		case command.New:
			bot.handleNewGameMessage(s, m, g, sett)

		case command.End:
			log.Println("User typed end to end the current game")

			dgs := bot.RedisInterface.GetReadOnlyDiscordGameState(gsr)
			if v, ok := bot.EndGameChannels[dgs.ConnectCode]; ok {
				v <- true
			}
			delete(bot.EndGameChannels, dgs.ConnectCode)

			bot.applyToAll(dgs, false, false)

		case command.Pause:
			lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
			if lock == nil {
				break
			}
			dgs.Running = !dgs.Running

			bot.RedisInterface.SetDiscordGameState(dgs, lock)
			if !dgs.Running {
				bot.applyToAll(dgs, false, false)
			}

			edited := dgs.Edit(s, bot.gameStateResponse(dgs, sett))
			if edited {
				metrics.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageEdit, 1)
			}

		case command.Refresh:
			bot.RefreshGameStateMessage(gsr, sett)
		case command.Link:
			if len(args[1:]) < 2 {
				embed := ConstructEmbedForCommand(prefix, cmd, sett)
				s.ChannelMessageSendEmbed(m.ChannelID, embed)
			} else {
				lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
				if lock == nil {
					break
				}
				bot.linkPlayer(s, dgs, args[1:])
				bot.RedisInterface.SetDiscordGameState(dgs, lock)

				edited := dgs.Edit(s, bot.gameStateResponse(dgs, sett))
				if edited {
					metrics.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageEdit, 1)
				}
			}

		case command.Unlink:
			if len(args[1:]) == 0 {
				embed := ConstructEmbedForCommand(prefix, cmd, sett)
				s.ChannelMessageSendEmbed(m.ChannelID, embed)
			} else {
				userID, err := extractUserIDFromMention(args[1])
				if err != nil {
					log.Println(err)
				} else {
					log.Print(fmt.Sprintf("Removing player %s", userID))
					lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
					if lock == nil {
						break
					}
					dgs.ClearPlayerData(userID)

					bot.RedisInterface.SetDiscordGameState(dgs, lock)

					// update the state message to reflect the player leaving
					edited := dgs.Edit(s, bot.gameStateResponse(dgs, sett))
					if edited {
						metrics.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageEdit, 1)
					}
				}
			}
		case command.UnmuteAll:
			dgs := bot.RedisInterface.GetReadOnlyDiscordGameState(gsr)
			bot.applyToAll(dgs, false, false)

		case command.Settings:
			premStatus, days := bot.PostgresInterface.GetGuildPremiumStatus(m.GuildID)
			isPrem := (premStatus != 0) && (days == storage.NoExpiryCode || days > 0)
			bot.HandleSettingsCommand(s, m, sett, args, isPrem)

		case command.Map:
			if len(args[1:]) == 0 {
				embed := ConstructEmbedForCommand(prefix, cmd, sett)
				s.ChannelMessageSendEmbed(m.ChannelID, embed)
			} else {
				mapVersion := args[len(args)-1]

				var mapName string
				switch mapVersion {
				case "simple", detailedMapString:
					mapName = strings.Join(args[1:len(args)-1], " ")
				default:
					mapName = strings.Join(args[1:], " ")
					mapVersion = sett.GetMapVersion()
				}
				mapItem, err := NewMapItem(mapName)
				if err != nil {
					log.Println(err)
					s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
						ID:    "commands.HandleCommand.Map.notFound",
						Other: "I don't have a map by that name!",
					}))
					break
				}
				switch mapVersion {
				case "simple":
					s.ChannelMessageSend(m.ChannelID, mapItem.MapImage.Simple)
				case detailedMapString:
					s.ChannelMessageSend(m.ChannelID, mapItem.MapImage.Detailed)
				default:
					log.Println("mapVersion has unexpected value for 'map' command")
				}
			}

		case command.Cache:
			if len(args[1:]) == 0 {
				embed := ConstructEmbedForCommand(prefix, cmd, sett)
				s.ChannelMessageSendEmbed(m.ChannelID, embed)
			} else {
				userID, err := extractUserIDFromMention(args[1])
				if err != nil {
					log.Println(err)
					s.ChannelMessageSend(m.ChannelID, "I couldn't find a user by that name or ID!")
					break
				}
				if len(args[2:]) == 0 {
					cached := bot.RedisInterface.GetUsernameOrUserIDMappings(m.GuildID, userID)
					if len(cached) == 0 {
						s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
							ID:    "commands.HandleCommand.Cache.emptyCachedNames",
							Other: "I don't have any cached player names stored for that user!",
						}))
					} else {
						buf := bytes.NewBuffer([]byte(sett.LocalizeMessage(&i18n.Message{
							ID:    "commands.HandleCommand.Cache.cachedNames",
							Other: "Cached in-game names:",
						})))
						buf.WriteString("\n```\n")
						for n := range cached {
							buf.WriteString(fmt.Sprintf("%s\n", n))
						}
						buf.WriteString("```")

						s.ChannelMessageSend(m.ChannelID, buf.String())
					}
				} else if strings.ToLower(args[2]) == clearArgumentString || strings.ToLower(args[2]) == "c" {
					err := bot.RedisInterface.DeleteLinksByUserID(m.GuildID, userID)
					if err != nil {
						log.Println(err)
					} else {
						s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
							ID:    "commands.HandleCommand.Cache.Success",
							Other: "Successfully deleted all cached names for that user!",
						}))
					}
				}
			}

		case command.Privacy:
			if m.Author != nil {
				var arg = ""
				if len(args[1:]) > 0 {
					arg = args[1]
				}
				if arg == "" || (arg != "showme" && arg != "optin" && arg != "optout") {
					embed := ConstructEmbedForCommand(prefix, cmd, sett)
					s.ChannelMessageSendEmbed(m.ChannelID, embed)
				} else {
					embed := bot.privacyResponse(m.GuildID, m.Author.ID, arg, sett)
					s.ChannelMessageSendEmbed(m.ChannelID, embed)
				}
			}

		case command.Info:
			embed := bot.infoResponse(m.GuildID, sett)
			_, err := s.ChannelMessageSendEmbed(m.ChannelID, embed)
			if err != nil {
				log.Println(err)
			}

		case command.DebugState:
			if m.Author != nil {
				state := bot.RedisInterface.GetReadOnlyDiscordGameState(gsr)
				if state != nil {
					jBytes, err := json.MarshalIndent(state, "", "  ")
					if err != nil {
						log.Println(err)
					} else {
						for i := 0; i < len(jBytes); i += MaxDebugMessageSize {
							end := i + MaxDebugMessageSize
							if end > len(jBytes) {
								end = len(jBytes)
							}
							s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("```JSON\n%s\n```", jBytes[i:end]))
						}
					}
				}
			}

		case command.ASCII:
			if len(args[1:]) == 0 {
				s.ChannelMessageSend(m.ChannelID, ASCIICrewmate)
			} else {
				id, err := extractUserIDFromMention(args[1])
				if id == "" || err != nil {
					s.ChannelMessageSend(m.ChannelID, "I couldn't find a user by that name or ID!")
				} else {
					imposter := false
					count := 1
					if len(args[2:]) > 0 {
						if args[2] == trueString || args[2] == "t" {
							imposter = true
						}
						if len(args[3:]) > 0 {
							if itCount, err := strconv.Atoi(args[3]); err == nil {
								count = itCount
							}
						}
					}
					s.ChannelMessageSend(m.ChannelID, ASCIIStarfield(sett, args[1], imposter, count))
				}
			}

		case command.Stats:
			premStatus, _ := bot.PostgresInterface.GetGuildPremiumStatus(m.GuildID)
			if len(args[1:]) == 0 {
				embed := ConstructEmbedForCommand(prefix, cmd, sett)
				s.ChannelMessageSendEmbed(m.ChannelID, embed)
			} else {
				userID, err := extractUserIDFromMention(args[1])
				if userID == "" || err != nil {
					arg := strings.ReplaceAll(args[1], "\"", "")
					if arg == "g" || arg == "guild" || arg == "server" {
						if len(args) > 2 && args[2] == "reset" {
							if !isAdmin {
								s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
									ID:    "message_handlers.handleResetGuild.noPerms",
									Other: "Only Admins are capable of resetting server stats",
								}))
							} else {
								if len(args) == 3 {
									_, err := s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
										ID:    "commands.StatsCommand.Reset.NoConfirm",
										Other: "Please type `{{.CommandPrefix}} stats guild reset confirm` if you are 100% certain that you wish to **completely reset** your guild's stats!",
									},
										map[string]interface{}{
											"CommandPrefix": prefix,
										}))
									if err != nil {
										log.Println(err)
									}
								} else if args[3] == "confirm" {
									err := bot.PostgresInterface.DeleteAllGamesForServer(m.GuildID)
									if err != nil {
										s.ChannelMessageSend(m.ChannelID, "Encountered the following error when deleting the server's stats: "+err.Error())
									} else {
										s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
											ID:    "commands.StatsCommand.Reset.Success",
											Other: "Successfully reset your guild's stats!",
										}))
									}
								}
							}
						} else {
							_, err := s.ChannelMessageSendEmbed(m.ChannelID, bot.GuildStatsEmbed(m.GuildID, sett, premStatus))
							if err != nil {
								log.Println(err)
							}
						}
					} else {
						arg = strings.ToUpper(arg)
						log.Println(arg)
						if MatchIDRegex.MatchString(arg) {
							strs := strings.Split(arg, ":")
							if len(strs) < 2 {
								log.Println("Something very wrong with the regex for match/conn codes...")
							} else {
								s.ChannelMessageSendEmbed(m.ChannelID, bot.GameStatsEmbed(strs[1], strs[0], sett, premStatus))
							}
						} else {
							s.ChannelMessageSend(m.ChannelID, "I didn't recognize that user, you mistyped 'guild', or didn't provide a valid Match ID")
						}
					}
				} else {
					if len(args) > 2 && args[2] == "reset" {
						if !isAdmin {
							s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
								ID:    "message_handlers.handleResetGuild.noPerms",
								Other: "Only Admins are capable of resetting server stats",
							}))
						} else {
							if len(args) == 3 {
								_, err := s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
									ID:    "commands.StatsCommand.ResetUser.NoConfirm",
									Other: "Please type `{{.CommandPrefix}} stats `{{.User}}` reset confirm` if you are 100% certain that you wish to **completely reset** that user's stats!",
								},
									map[string]interface{}{
										"CommandPrefix": prefix,
										"User":          args[1],
									}))
								if err != nil {
									log.Println(err)
								}
							} else if args[3] == "confirm" {
								err := bot.PostgresInterface.DeleteAllGamesForUser(userID)
								if err != nil {
									s.ChannelMessageSend(m.ChannelID, "Encountered the following error when deleting that user's stats: "+err.Error())
								} else {
									s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
										ID:    "commands.StatsCommand.ResetUser.Success",
										Other: "Successfully reset {{.User}}'s stats!",
									},
										map[string]interface{}{
											"User": args[1],
										}))
								}
							}
						}
					} else {
						s.ChannelMessageSendEmbed(m.ChannelID, bot.UserStatsEmbed(userID, m.GuildID, sett, premStatus))
					}
				}
			}

		case command.Premium:
			premStatus, days := bot.PostgresInterface.GetGuildPremiumStatus(m.GuildID)
			if len(args[1:]) == 0 {
				s.ChannelMessageSendEmbed(m.ChannelID, premiumEmbedResponse(m.GuildID, premStatus, days, sett))
			} else {
				arg := strings.ToLower(args[1])
				if isAdmin {
					if arg == "invite" || arg == "invites" || arg == "inv" {
						_, err := s.ChannelMessageSendEmbed(m.ChannelID, premiumInvitesEmbed(premStatus, sett))
						if err != nil {
							log.Println(err)
						}
					} else {
						s.ChannelMessageSend(m.ChannelID, "Sorry, I didn't recognize that premium command or argument!")
					}
				} else {
					s.ChannelMessageSend(m.ChannelID, "Viewing the premium invites is an Admin-only command")
				}
			}

		default:
			s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.HandleCommand.default",
				Other: "Sorry, I didn't understand `{{.InvalidCommand}}`! Please see `{{.CommandPrefix}} help` for commands",
			},
				map[string]interface{}{
					"CommandPrefix":  prefix,
					"InvalidCommand": args[0],
				}))
		}
	}

	deleteMessage(s, m.ChannelID, m.Message.ID)
}

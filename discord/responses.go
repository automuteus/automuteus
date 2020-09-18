package discord

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func helpResponse() string {
	buf := bytes.NewBuffer([]byte{})
	buf.WriteString("Among Us Bot command reference:\n")
	buf.WriteString("Having issues or have suggestions? Join the discord at https://discord.gg/ZkqZSWF !\n")
	buf.WriteString(fmt.Sprintf("`%s help` (`%s h`): Print help info and command usage.\n", CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s list` (`%s l`): Print the currently tracked players, and their in-game status (Beta).\n", CommandPrefix, CommandPrefix))
	//buf.WriteString(fmt.Sprintf("`%s dead` (`%s d`): Mark a user as dead so they aren't unmuted during discussions. Ex: `%s d @DiscordUser1 @DiscordUser2`\n", CommandPrefix, CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s track` (`%s t`): Tell Bot to use a single voice channel for mute/unmute, and ignore other players. Ex: `%s t Voice channel name`\n", CommandPrefix, CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s bcast` (`%s b`): Tell Bot to broadcast the room code and region. Ex: `%s b ABCD asia` or `%s b ABCD na`\n", CommandPrefix, CommandPrefix, CommandPrefix, CommandPrefix))
	//buf.WriteString(fmt.Sprintf("`%s add` (`%s a`): Manually add players to the tracked list (muted/unmuted throughout the game). Ex: `%s a @DiscordUser2 @DiscordUser1`\n", CommandPrefix, CommandPrefix, CommandPrefix))
	//buf.WriteString(fmt.Sprintf("`%s reset` (`%s r`): Reset the tracked player list manually (mainly for debug)\n", CommandPrefix, CommandPrefix))
	//buf.WriteString(fmt.Sprintf("`%s muteall` (`%s ma`): Forcibly mute ALL users (mainly for debug).\n", CommandPrefix, CommandPrefix))
	//buf.WriteString(fmt.Sprintf("`%s unmuteall` (`%s ua`): Forcibly unmute ALL users (mainly for debug).\n", CommandPrefix, CommandPrefix))
	return buf.String()
}

func (guild *GuildState) broadcastResponse(args []string) (string, error) {
	buf := bytes.NewBuffer([]byte{})
	code, region := "", ""
	//just the room code
	code = strings.ToUpper(args[0])

	if len(args) > 1 {
		region = strings.ToLower(args[1])
		switch region {
		case "na":
			fallthrough
		case "us":
			fallthrough
		case "usa":
			fallthrough
		case "north":
			region = "North America"
		case "eu":
			fallthrough
		case "europe":
			region = "Europe"
		case "as":
			fallthrough
		case "asia":
			region = "Asia"
		}
	}
	guild.UserDataLock.RLock()
	for _, player := range guild.UserData {
		if player.tracking {
			buf.WriteString(fmt.Sprintf("<@!%s> ", player.user.userID))
		}
	}
	guild.UserDataLock.RUnlock()
	buf.WriteString(fmt.Sprintf("\nThe Room Code is **%s**\n", code))

	if region == "" {
		buf.WriteString("I wasn't told the Region, though :cry:")
	} else {
		buf.WriteString(fmt.Sprintf("The Region is **%s**\n", region))
	}
	return buf.String(), nil
}

//TODO update original message, not post new one
//TODO delete player messages relating to this
func (guild *GuildState) playerListResponse() string {
	buf := bytes.NewBuffer([]byte{})
	//TODO print the tracked again
	//if TrackingVoiceId != "" {
	//	buf.WriteString(fmt.Sprintf("Currently tracking \"%s\" Voice Channel:\n", TrackingVoiceName))
	//} else {
	//	buf.WriteString("Not tracking a Voice Channel; all players will be Automuted (use `.au t` to track)\n")
	//}

	buf.WriteString("Player List:\n")
	guild.UserDataLock.RLock()
	for _, player := range guild.UserData {
		if player.tracking {
			if player.auData != nil {
				emoji := AlivenessColoredEmojis[player.auData.IsAlive][player.auData.Color]
				buf.WriteString(fmt.Sprintf("<:%s:%s> <@!%s>: %s\n", emoji.Name, emoji.ID, player.user.userID, player.auData.Name))
			} else {
				buf.WriteString(fmt.Sprintf(":x: <@!%s>: Use `.au link @%s <in-game name>`\n", player.user.userID, player.user.userName))
			}

		}
	}
	guild.UserDataLock.RUnlock()
	return buf.String()
}

func (guild *GuildState) trackChannelResponse(channelName string, allChannels []*discordgo.Channel, forGhosts bool) string {
	for _, c := range allChannels {
		if (strings.ToLower(c.Name) == strings.ToLower(channelName) || c.ID == channelName) && c.Type == 2 {
			//TODO check duplicates (for logging)
			guild.Tracking[c.ID] = Tracking{
				channelID:   c.ID,
				channelName: c.Name,
				forGhosts:   forGhosts,
			}
			return fmt.Sprintf("Now tracking \"%s\" Voice Channel for Automute (for ghosts? %v)!", c.Name, forGhosts)
		}
	}
	return fmt.Sprintf("No channel found by the name %s!\n", channelName)
}

//TODO implement a separate way to link by color (color is more volatile)
//TODO implement a way to match in-game names to discord ones WITHOUT the link command
func (guild *GuildState) linkPlayerResponse(args []string, allAuData *[]AmongUserData) string {
	userID, err := extractUserIDFromMention(args[0])
	if err != nil {
		return fmt.Sprintf("Invalid mention format for \"%s\"", args[0])
	}

	inGameName := strings.ToLower(args[1])
	for color, auData := range *allAuData {
		log.Println(strings.ToLower(auData.Name))
		if strings.ToLower(auData.Name) == inGameName {
			if user, ok := guild.UserData[userID]; ok {
				user.auData = &(*allAuData)[color] //point to the single copy in memory
				guild.UserData[userID] = user
				log.Printf("Linked %s to %s", args[0], user.auData.ToString())
				return fmt.Sprintf("Successfully linked player to existing game data!")
			}
			return fmt.Sprintf("No user found with userID %s", userID)
		}
	}
	return fmt.Sprintf(":x: No in-game name was found matching %s!\n", inGameName)
}

func extractUserIDFromMention(mention string) (string, error) {
	//nickname format
	if strings.HasPrefix(mention, "<@!") && strings.HasSuffix(mention, ">") {
		return mention[3 : len(mention)-1], nil
		//non-nickname format
	} else if strings.HasPrefix(mention, "<@") && strings.HasSuffix(mention, ">") {
		return mention[2 : len(mention)-1], nil
	} else {
		return "", errors.New("mention does not conform to the correct format")
	}
}

//func (guild *GuildState) processMarkAliveUsers(dg *discordgo.Session, args []string, markAlive bool) map[string]string {
//	responses := make(map[string]string)
//	for _, v := range args {
//		if strings.HasPrefix(v, "<@!") && strings.HasSuffix(v, ">") {
//			//strip the special characters off front and end
//			idLookup := v[3 : len(v)-1]
//			guild.UserDataLock.Lock()
//			for id, user := range guild.UserData {
//				if id == idLookup {
//					temp := guild.UserData[id]
//					temp.auData.IsAlive = markAlive
//					guild.UserData[id] = temp
//
//					nameIdx := user.user.userName
//					if user.user.nick != "" {
//						nameIdx = user.user.userName + " (" + user.user.nick + ")"
//					}
//					if markAlive {
//						responses[nameIdx] = "Marked Alive"
//					} else {
//						responses[nameIdx] = "Marked Dead"
//					}
//
//					guild.GamePhaseLock.RLock()
//					if guild.GamePhase == game.DISCUSS {
//						err := guildMemberMute(dg, guild.ID, id, !markAlive)
//						if err != nil {
//							log.Printf("Error muting/unmuting %s: %s\n", user.user.userName, err)
//						}
//						if markAlive {
//							responses[nameIdx] = "Marked Alive and Unmuted"
//						} else {
//							responses[nameIdx] = "Marked Dead and Muted"
//						}
//
//					}
//					guild.GamePhaseLock.RUnlock()
//				}
//			}
//			guild.UserDataLock.Unlock()
//		} else {
//			responses[v] = "Not currently supporting non-`@` direct mentions, sorry!"
//		}
//	}
//	return responses
//}
//
//func (guild *GuildState) processAddUsersArgs(args []string) map[string]string {
//	responses := make(map[string]string)
//	for _, v := range args {
//		if strings.HasPrefix(v, "<@!") && strings.HasSuffix(v, ">") {
//			//strip the special characters off front and end
//			idLookup := v[3 : len(v)-1]
//			guild.UserDataLock.Lock()
//			for id, user := range guild.UserData {
//				if id == idLookup {
//					guild.UserData[id] = UserData{
//						user:         user.user,
//						voiceState:   discordgo.VoiceState{},
//						tracking:     true, //always assume true if we're adding users manually
//						auData: &AmongUserData{
//							Color: AmongUsDefaultColor,
//							Name:  AmongUsDefaultName,
//							IsAlive: true,
//						},
//					}
//					nameIdx := user.user.userName
//					if user.user.nick != "" {
//						nameIdx = user.user.userName + " (" + user.user.nick + ")"
//					}
//					responses[nameIdx] = "Added successfully!"
//				}
//			}
//			guild.UserDataLock.Unlock()
//		} else {
//			responses[v] = "Not currently supporting non-`@` direct mentions, sorry!"
//		}
//	}
//	return responses
//}

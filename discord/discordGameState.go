package discord

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"log"
	"strings"
)

type TrackingChannel struct {
	ChannelID   string `json:"channelID"`
	ChannelName string `json:"channelName"`
}

func (tc TrackingChannel) ToStatusString() string {
	if tc.ChannelID == "" || tc.ChannelName == "" {
		return "Any Voice Channel"
	} else {
		return tc.ChannelName
	}
}

func (tc TrackingChannel) ToDescString() string {
	if tc.ChannelID == "" || tc.ChannelName == "" {
		return "any voice channel!"
	} else {
		return "the **" + tc.ChannelName + "** voice channel!"
	}
}

type DiscordGameState struct {
	GuildID string `json:"guildID"`

	ConnectCode string `json:"connectCode"`

	Linked  bool `json:"linked"`
	Running bool `json:"running"`

	UserData UserDataSet     `json:"userData"`
	Tracking TrackingChannel `json:"tracking"`

	GameStateMsg GameStateMessage `json:"gameStateMessage"`

	AmongUsData game.AmongUsData `json:"amongUsData"`
}

func NewDiscordGameState(guildID string) *DiscordGameState {
	return &DiscordGameState{
		GuildID:      guildID,
		ConnectCode:  "",
		Linked:       false,
		Running:      false,
		UserData:     UserDataSet{},
		Tracking:     TrackingChannel{},
		GameStateMsg: MakeGameStateMessage(),
		AmongUsData:  game.NewAmongUsData(),
	}
}

func (dgs *DiscordGameState) Reset() {
	dgs.ConnectCode = ""
	dgs.Linked = false
	dgs.Running = false
	dgs.UserData = map[string]UserData{}
	dgs.Tracking = TrackingChannel{}
	dgs.GameStateMsg = MakeGameStateMessage()
	dgs.AmongUsData = game.NewAmongUsData()
}

func (dgs *DiscordGameState) checkCacheAndAddUser(g *discordgo.Guild, s *discordgo.Session, userID string) (UserData, bool) {
	if g == nil {
		return UserData{}, false
	}
	//check and see if they're cached first
	for _, v := range g.Members {
		if v.User.ID == userID {
			user := MakeUserDataFromDiscordUser(v.User, v.Nick)
			dgs.UserData.AddFullUser(user)
			return user, true
		}
	}
	mem, err := s.GuildMember(g.ID, userID)
	if err != nil {
		log.Println(err)
		return UserData{}, false
	}
	user := MakeUserDataFromDiscordUser(mem.User, mem.Nick)
	dgs.UserData.AddFullUser(user)
	return user, true
}

func (dgs *DiscordGameState) clearGameTracking(s *discordgo.Session) {
	//clear the discord User links to underlying player data
	dgs.UserData.ClearAllPlayerData()

	//reset all the Tracking channels
	dgs.Tracking = TrackingChannel{}

	dgs.GameStateMsg.Delete(s)
}

func (bot *Bot) linkPlayer(s *discordgo.Session, dgs *DiscordGameState, args []string) {
	g, err := s.State.Guild(dgs.GuildID)
	if err != nil {
		log.Println(err)
		return
	}

	userID := getMemberFromString(s, dgs.GuildID, args[0])
	if userID == "" {
		log.Printf("Sorry, I don't know who `%s` is. You can pass in ID, username, username#XXXX, nickname or @mention", args[0])
	}

	_, added := dgs.checkCacheAndAddUser(g, s, userID)
	if !added {
		log.Println("No users found in Discord for UserID " + userID)
	}

	combinedArgs := strings.ToLower(strings.Join(args[1:], ""))
	var playerData game.PlayerData
	found := false
	if game.IsColorString(combinedArgs) {
		playerData, found = dgs.AmongUsData.GetByColor(combinedArgs)

	} else {
		playerData, found = dgs.AmongUsData.GetByName(combinedArgs)
	}
	if found {
		found, _, _ = dgs.UserData.AttemptPairingByMatchingNames(playerData)
		if found {
			log.Printf("Successfully linked %s to a color\n", userID)
			bot.RedisInterface.SetDiscordGameState(dgs.GuildID, dgs)
		} else {
			log.Printf("No player was found with id %s\n", userID)
		}
	}
	return
}

func (dgs *DiscordGameState) trackChannel(channelName string, allChannels []*discordgo.Channel) string {
	for _, c := range allChannels {
		if (strings.ToLower(c.Name) == strings.ToLower(channelName) || c.ID == channelName) && c.Type == 2 {

			dgs.Tracking = TrackingChannel{ChannelName: c.Name, ChannelID: c.ID}

			log.Println(fmt.Sprintf("Now Tracking \"%s\" Voice Channel for Automute!", c.Name))
			return fmt.Sprintf("Now Tracking \"%s\" Voice Channel for Automute!", c.Name)
		}
	}
	return fmt.Sprintf("No channel found by the name %s!\n", channelName)
}

func (dgs *DiscordGameState) ToEmojiEmbedFields(emojis AlivenessEmojis) []*discordgo.MessageEmbedField {
	unsorted := make([]*discordgo.MessageEmbedField, 12)
	num := 0

	for _, player := range dgs.AmongUsData.PlayerData {
		for _, userData := range dgs.UserData {
			if userData.InGameName == player.Name {
				emoji := emojis[player.IsAlive][player.Color]
				unsorted[player.Color] = &discordgo.MessageEmbedField{
					Name:   fmt.Sprintf("%s", player.Name),
					Value:  fmt.Sprintf("%s <@!%s>", emoji.FormatForInline(), userData.GetID()),
					Inline: true,
				}
				break
			}
		}
		//no player matched; unlinked player
		if unsorted[player.Color] == nil {
			emoji := emojis[player.IsAlive][player.Color]
			unsorted[player.Color] = &discordgo.MessageEmbedField{
				Name:   fmt.Sprintf("%s", player.Name),
				Value:  fmt.Sprintf("%s **Unlinked**", emoji.FormatForInline()),
				Inline: true,
			}
		}
		num++
	}

	sorted := make([]*discordgo.MessageEmbedField, num)
	num = 0
	for i := 0; i < 12; i++ {
		if unsorted[i] != nil {
			sorted[num] = unsorted[i]
			num++
		}
	}
	return sorted
}

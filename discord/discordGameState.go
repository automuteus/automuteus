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

	UserData UserDataSet     `json:"userDataSet"`
	Tracking TrackingChannel `json:"tracking"`

	GameStateMsg GameStateMessage `json:"gameStateMessage"`
}

func NewDiscordGameState(guildID string) *DiscordGameState {
	return &DiscordGameState{
		GuildID:      guildID,
		ConnectCode:  "",
		Linked:       false,
		Running:      false,
		UserData:     MakeUserDataSet(),
		Tracking:     TrackingChannel{},
		GameStateMsg: MakeGameStateMessage(),
	}
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
	//clear the discord user links to underlying player data
	dgs.UserData.ClearAllPlayerData()

	//reset all the Tracking channels
	dgs.Tracking = TrackingChannel{}

	dgs.GameStateMsg.Delete(s)
}

func (dgs *DiscordGameState) linkPlayer(s *discordgo.Session, aud *game.AmongUsData, args []string) {
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
		log.Println("No users found in Discord for userID " + userID)
	}

	combinedArgs := strings.ToLower(strings.Join(args[1:], ""))
	if game.IsColorString(combinedArgs) {
		playerData, found := aud.GetByColor(combinedArgs)
		if found {
			found = dgs.UserData.UpdatePlayerData(userID, &playerData)
			if found {
				//TODO update in DB
				//TODO update GSM
				log.Printf("Successfully linked %s to a color\n", userID)
			} else {
				log.Printf("No player was found with id %s\n", userID)
			}
		}
		return
	} else {
		playerData, found := aud.GetByName(combinedArgs)
		if found {
			found = dgs.UserData.UpdatePlayerData(userID, &playerData)
			if found {
				//TODO update in DB
				//TODO update GSM
				log.Printf("Successfully linked %s by name\n", userID)
			} else {
				log.Printf("No player was found with id %s\n", userID)
			}
		}
	}
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

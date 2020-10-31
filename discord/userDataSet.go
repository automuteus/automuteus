package discord

import (
	"errors"
	"fmt"
	"strings"

	"github.com/denverquane/amongusdiscord/game"
)

type UserDataSet map[string]UserData

func (dgs *DiscordGameState) UserDataSize() int {
	return len(dgs.UserData)
}

func (dgs *DiscordGameState) GetCountLinked() int {
	LinkedPlayerCount := 0

	for _, v := range dgs.UserData {
		if v.InGameName != game.UnlinkedPlayerName {
			LinkedPlayerCount++
		}
	}
	return LinkedPlayerCount
}

func (dgs *DiscordGameState) AttemptPairingByMatchingNames(data game.PlayerData) bool {
	name := strings.ReplaceAll(strings.ToLower(data.Name), " ", "")
	for userID, v := range dgs.UserData {
		if strings.ReplaceAll(strings.ToLower(v.GetUserName()), " ", "") == name || strings.ReplaceAll(strings.ToLower(v.GetNickName()), " ", "") == name {
			v.Link(data)
			dgs.UserData[userID] = v
			return true
		}
	}
	return false
}

func (dgs *DiscordGameState) UpdateUserData(userID string, data UserData) {
	dgs.UserData[userID] = data
}

func (dgs *DiscordGameState) AttemptPairingByUserIDs(data game.PlayerData, userIDs map[string]interface{}) bool {
	for userID := range userIDs {
		//only attempt to link players that aren't paired already
		if v, ok := dgs.UserData[userID]; ok && v.GetPlayerName() == game.UnlinkedPlayerName {
			v.Link(data)
			dgs.UserData[userID] = v
			return true
		}
	}
	return false
}

func (dgs *DiscordGameState) ClearPlayerData(userID string) {
	if v, ok := dgs.UserData[userID]; ok {
		v.InGameName = game.UnlinkedPlayerName
		dgs.UserData[userID] = v
	}
}

func (dgs *DiscordGameState) ClearPlayerDataByPlayerName(playerName string) {
	for i, v := range dgs.UserData {
		if v.GetPlayerName() == playerName {
			v.InGameName = game.UnlinkedPlayerName
			dgs.UserData[i] = v
			return
		}
	}
}

func (dgs *DiscordGameState) ClearAllPlayerData() {
	for i, v := range dgs.UserData {
		v.InGameName = game.UnlinkedPlayerName
		dgs.UserData[i] = v
	}
}

func (dgs *DiscordGameState) GetUser(userID string) (UserData, error) {
	if v, ok := dgs.UserData[userID]; ok {
		return v, nil
	}
	return UserData{}, errors.New(fmt.Sprintf("No User found with ID %s", userID))
}

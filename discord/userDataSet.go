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
		if v.InGameName == game.UnlinkedPlayerName {
			if strings.ReplaceAll(strings.ToLower(v.GetUserName()), " ", "") == name || strings.ReplaceAll(strings.ToLower(v.GetNickName()), " ", "") == name {
				v.Link(data)
				dgs.UserData[userID] = v
				dgs.NeedsUpload = true
				return true
			}
		}
	}
	return false
}

func (dgs *DiscordGameState) UpdateUserData(userID string, data UserData) {
	dgs.UserData[userID] = data
	dgs.NeedsUpload = true
}

func (dgs *DiscordGameState) AddFullUser(user UserData) {
	dgs.UserData[user.GetID()] = user
	dgs.NeedsUpload = true
}

func (dgs *DiscordGameState) AttemptPairingByUserIDs(data game.PlayerData, userIDs []string) bool {
	for _, userID := range userIDs {
		if v, ok := dgs.UserData[userID]; ok {
			v.Link(data)
			dgs.UserData[userID] = v
			dgs.NeedsUpload = true
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
	dgs.NeedsUpload = true
}

func (dgs *DiscordGameState) ClearPlayerDataByPlayerName(playerName string) {
	for i, v := range dgs.UserData {
		if v.GetPlayerName() == playerName {
			v.InGameName = game.UnlinkedPlayerName
			dgs.UserData[i] = v
			dgs.NeedsUpload = true
			return
		}
	}
}

func (dgs *DiscordGameState) ClearAllPlayerData() {
	for i, v := range dgs.UserData {
		v.InGameName = game.UnlinkedPlayerName
		dgs.UserData[i] = v
	}
	dgs.NeedsUpload = true
}

func (dgs *DiscordGameState) GetUser(userID string) (UserData, error) {
	if v, ok := dgs.UserData[userID]; ok {
		return v, nil
	}
	return UserData{}, errors.New(fmt.Sprintf("No User found with ID %s", userID))
}

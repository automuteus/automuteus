package discord

import (
	"errors"
	"fmt"
	"strings"

	"github.com/denverquane/amongusdiscord/game"
)

type UserDataSet map[string]UserData

func (uds UserDataSet) Size() int {
	return len(uds)
}

func (uds UserDataSet) GetCountLinked() int {
	LinkedPlayerCount := 0

	for _, v := range uds {
		if v.InGameName != game.UnlinkedPlayerName {
			LinkedPlayerCount++
		}
	}
	return LinkedPlayerCount
}

func (uds *UserDataSet) AddFullUser(user UserData) {
	(*uds)[user.GetID()] = user
}

func (uds *UserDataSet) UpdateUserData(userID string, data UserData) {
	(*uds)[userID] = data
}

func (uds *UserDataSet) AttemptPairingByMatchingNames(data game.PlayerData) (bool, string, string) {
	name := strings.ReplaceAll(strings.ToLower(data.Name), " ", "")
	for userID, v := range *uds {
		if v.InGameName == game.UnlinkedPlayerName {
			if strings.ReplaceAll(strings.ToLower(v.GetUserName()), " ", "") == name || strings.ReplaceAll(strings.ToLower(v.GetNickName()), " ", "") == name {
				v.Link(data)
				(*uds)[userID] = v
				return true, userID, v.User.UserName
			}
		}
	}
	return false, "", ""
}

func (uds *UserDataSet) ClearPlayerData(userID string) {
	if v, ok := (*uds)[userID]; ok {
		v.InGameName = game.UnlinkedPlayerName
		(*uds)[userID] = v
	}
}

func (uds *UserDataSet) ClearPlayerDataByPlayerName(playerName string) {
	for i, v := range *uds {
		if v.GetPlayerName() == playerName {
			v.InGameName = game.UnlinkedPlayerName
			(*uds)[i] = v
		}
	}
}

func (uds *UserDataSet) ClearAllPlayerData() {
	for i, v := range *uds {
		v.InGameName = game.UnlinkedPlayerName
		(*uds)[i] = v
	}
}

func (uds *UserDataSet) GetUser(userID string) (UserData, error) {
	if v, ok := (*uds)[userID]; ok {
		return v, nil
	}
	return UserData{}, errors.New(fmt.Sprintf("No User found with ID %s", userID))
}

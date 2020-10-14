package discord

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
)

type UserDataSet struct {
	userDataSet map[string]UserData
	lock        sync.RWMutex
}

func MakeUserDataSet() UserDataSet {
	return UserDataSet{
		userDataSet: map[string]UserData{},
		lock:        sync.RWMutex{},
	}
}

func (uds *UserDataSet) Size() int {
	uds.lock.RLock()
	defer uds.lock.RUnlock()
	return len(uds.userDataSet)
}

func (uds *UserDataSet) GetCountLinked() int {
	uds.lock.RLock()
	defer uds.lock.RUnlock()

	LinkedPlayerCount := 0

	for _, v := range uds.userDataSet {
		if v.IsLinked() {
			LinkedPlayerCount++
		}
	}
	return LinkedPlayerCount
}

func (uds *UserDataSet) AddFullUser(user UserData) {
	uds.lock.Lock()
	uds.userDataSet[user.GetID()] = user
	uds.lock.Unlock()
}

func (uds *UserDataSet) UpdateUserData(userID string, data UserData) {
	uds.lock.Lock()
	uds.userDataSet[userID] = data
	uds.lock.Unlock()
}

func (uds *UserDataSet) UpdatePlayerData(userID string, data *game.PlayerData) bool {
	uds.lock.Lock()
	defer uds.lock.Unlock()

	if v, ok := uds.userDataSet[userID]; ok {
		v.SetPlayerData(data)
		uds.userDataSet[userID] = v
		return true
	}
	return false
}

func (uds *UserDataSet) UpdatePlayerMappingByName(name string, data *game.PlayerData) {
	uds.lock.Lock()
	defer uds.lock.Unlock()
	for userID, v := range uds.userDataSet {
		if v.GetPlayerName() == name {
			v.SetPlayerData(data)
			uds.userDataSet[userID] = v
			return
		}
	}
}

func (uds *UserDataSet) AttemptPairingByMatchingNames(name string, data *game.PlayerData) (bool, string, string) {
	uds.lock.Lock()
	defer uds.lock.Unlock()
	name = strings.ReplaceAll(strings.ToLower(name), " ", "")
	for userID, v := range uds.userDataSet {
		if !v.IsLinked() {
			if strings.ReplaceAll(strings.ToLower(v.GetUserName()), " ", "") == name || strings.ReplaceAll(strings.ToLower(v.GetNickName()), " ", "") == name {
				v.SetPlayerData(data)
				uds.userDataSet[userID] = v
				return true, userID, v.user.userName
			}
		}
	}
	return false, "", ""
}

func (uds *UserDataSet) ClearPlayerData(userID string) {
	uds.lock.Lock()
	if v, ok := uds.userDataSet[userID]; ok {
		v.SetPlayerData(nil)
		uds.userDataSet[userID] = v
	}
	uds.lock.Unlock()
}

func (uds *UserDataSet) ClearPlayerDataByPlayerName(playerName string) {
	uds.lock.Lock()
	for i, v := range uds.userDataSet {
		if v.GetPlayerName() == playerName {
			v.SetPlayerData(nil)
			uds.userDataSet[i] = v
		}
	}
	uds.lock.Unlock()
}

func (uds *UserDataSet) ClearAllPlayerData() {
	uds.lock.Lock()
	for i, v := range uds.userDataSet {
		v.SetPlayerData(nil)
		uds.userDataSet[i] = v
	}
	uds.lock.Unlock()
}

func (uds *UserDataSet) GetUser(userID string) (UserData, error) {
	uds.lock.RLock()
	defer uds.lock.RUnlock()

	if v, ok := uds.userDataSet[userID]; ok {
		return v, nil
	}
	return UserData{}, errors.New(fmt.Sprintf("No user found with ID %s", userID))
}

func (uds *UserDataSet) ToEmojiEmbedFields(nameColorMap map[string]int, nameAliveMap map[string]bool, emojis AlivenessEmojis) []*discordgo.MessageEmbedField {
	uds.lock.RLock()
	defer uds.lock.RUnlock()

	unsorted := make([]*discordgo.MessageEmbedField, 12)
	num := 0

	for name, color := range nameColorMap {
		for _, player := range uds.userDataSet {
			if player.IsLinked() && player.GetPlayerName() == name {
				emoji := emojis[player.IsAlive()][color]
				unsorted[color] = &discordgo.MessageEmbedField{
					Name:   fmt.Sprintf("%s", name),
					Value:  fmt.Sprintf("%s <@!%s>", emoji.FormatForInline(), player.GetID()),
					Inline: true,
				}
				break
			}
		}
		//no player matched; unlinked player
		if unsorted[color] == nil {
			emoji := emojis[nameAliveMap[name]][color]
			unsorted[color] = &discordgo.MessageEmbedField{
				Name:   fmt.Sprintf("%s", name),
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

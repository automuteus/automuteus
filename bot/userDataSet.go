package bot

import (
	"fmt"
	"github.com/automuteus/automuteus/v8/pkg/amongus"
	"strings"
)

type UserDataSet map[string]UserData

func (dgs *GameState) GetCountLinked() int {
	LinkedPlayerCount := 0

	for _, v := range dgs.UserData {
		if v.InGameName != amongus.UnlinkedPlayerName {
			LinkedPlayerCount++
		}
	}
	return LinkedPlayerCount
}

func (dgs *GameState) AttemptPairingByMatchingNames(data amongus.PlayerData) string {
	name := strings.ReplaceAll(strings.ToLower(data.Name), " ", "")
	for userID, v := range dgs.UserData {
		if v.FriendCode != "" && data.FriendCode != "" && v.FriendCode != data.FriendCode {
			// they're linked to their own account; this is someone else with their name
			continue
		}
		if strings.ReplaceAll(strings.ToLower(v.GetUserName()), " ", "") == name || strings.ReplaceAll(strings.ToLower(v.GetNickName()), " ", "") == name {
			v.Link(data)
			dgs.UserData[userID] = v
			return userID
		}
	}
	return ""
}

func (dgs *GameState) UpdateUserData(userID string, data UserData) {
	if dgs.UserData != nil {
		dgs.UserData[userID] = data
	}
}

func (dgs *GameState) AttemptPairingByUserIDs(data amongus.PlayerData, userIDs map[string]interface{}) string {
	for userID := range userIDs {
		if v, ok := dgs.UserData[userID]; ok {
			if v.FriendCode != "" && data.FriendCode != "" && v.FriendCode != data.FriendCode {
				// they're linked to their own account; this is someone else with a name they used
				continue
			}
			// only attempt to link players that aren't paired already
			if v.GetPlayerName() == amongus.UnlinkedPlayerName {
				v.Link(data)
				dgs.UserData[userID] = v
			}
			return userID
		}
	}
	return ""
}

// RelinkByFriendCode keeps users linked to the player they were linked to: anyone linked to this Friend Code follows it
// to the player's current name, and anyone linked to this name before Friend Codes were known gets the code recorded.
// It returns the user it linked, or "".
func (dgs *GameState) RelinkByFriendCode(data amongus.PlayerData) string {
	if data.FriendCode == "" {
		return ""
	}
	// a user linked to the account wins over one linked to the name alone
	for _, matches := range []func(UserData) bool{
		func(v UserData) bool { return v.FriendCode == data.FriendCode },
		func(v UserData) bool { return v.FriendCode == "" && v.InGameName == data.Name },
	} {
		for userID, v := range dgs.UserData {
			if matches(v) {
				v.Link(data)
				dgs.UserData[userID] = v
				return userID
			}
		}
	}
	return ""
}

// AttemptPairingByFriendCode links the player to a user remembered as having played on their Friend Code. A user is
// only moved off a player who's no longer in the game, such as their own old name. It returns the user it linked, or "".
func (dgs *GameState) AttemptPairingByFriendCode(data amongus.PlayerData, userIDs map[string]interface{}) string {
	for userID := range userIDs {
		v, ok := dgs.UserData[userID]
		if !ok {
			continue
		}
		if _, present := dgs.GameData.PlayerData[v.InGameName]; present && v.GetPlayerName() != amongus.UnlinkedPlayerName {
			continue
		}
		v.Link(data)
		dgs.UserData[userID] = v
		return userID
	}
	return ""
}

// LinkedPlayer returns the in-game player the user is linked to, if they're in the game.
func (dgs *GameState) LinkedPlayer(user UserData) (amongus.PlayerData, bool) {
	if user.FriendCode != "" {
		if p, ok := dgs.GameData.GetByFriendCode(user.FriendCode); ok {
			return p, true
		}
	}
	p, ok := dgs.GameData.GetByName(user.InGameName)
	if ok && user.FriendCode != "" && p.FriendCode != "" {
		// a different account is playing under the name they had
		return amongus.UnlinkedPlayer, false
	}
	return p, ok
}

func (dgs *GameState) ClearPlayerData(userID string) bool {
	if v, ok := dgs.UserData[userID]; ok {
		v.Unlink()
		dgs.UserData[userID] = v
		return true
	}
	return false
}

// LinkedUserID returns the ID of the Discord user linked to the in-game player, or "" if nobody is.
func (dgs *GameState) LinkedUserID(player amongus.PlayerData) string {
	for userID, v := range dgs.UserData {
		if v.IsLinkedTo(player) {
			return userID
		}
	}
	return ""
}

func (dgs *GameState) ClearPlayerDataByPlayer(player amongus.PlayerData) {
	for i, v := range dgs.UserData {
		if v.IsLinkedTo(player) {
			v.Unlink()
			dgs.UserData[i] = v
			return
		}
	}
}

func (dgs *GameState) UnlinkAllUsers() {
	for i, v := range dgs.UserData {
		v.Unlink()
		dgs.UserData[i] = v
	}
}

func (dgs *GameState) GetUser(userID string) (UserData, error) {
	if v, ok := dgs.UserData[userID]; ok {
		return v, nil
	}
	return UserData{}, fmt.Errorf("no User found with ID %s", userID)
}

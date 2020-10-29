package game

import (
	"log"
	"strings"
)

type AmongUsData struct {
	//indexed by amongusname
	PlayerData map[string]PlayerData `json:"playerData"`

	Phase  Phase  `json:"phase"`
	Room   string `json:"room"`
	Region string `json:"region"`
}

func NewAmongUsData() AmongUsData {
	return AmongUsData{
		PlayerData: map[string]PlayerData{},
		Phase:      MENU,
		Room:       "",
		Region:     "",
	}
}

func (auData *AmongUsData) SetRoomRegion(room, region string) {
	auData.Room = room
	auData.Region = region
}

func (auData *AmongUsData) GetRoomRegion() (string, string) {
	return auData.Room, auData.Region
}

func (auData *AmongUsData) SetAllAlive() {
	for i, v := range auData.PlayerData {
		v.IsAlive = true
		auData.PlayerData[i] = v
	}
}

func (auData *AmongUsData) UpdatePhase(phase Phase) (old Phase) {
	old = auData.Phase
	auData.Phase = phase

	if old != phase {
		if phase == LOBBY || (phase == TASKS && old == LOBBY) {
			auData.SetAllAlive()
		} else if phase == MENU {
			auData.SetRoomRegion("", "")
		}
	}
	return old
}

func (auData *AmongUsData) UpdatePlayer(player Player) (updated, isAliveUpdated bool, data PlayerData) {
	phase := auData.Phase

	if phase == LOBBY && player.IsDead {
		player.IsDead = false
	}
	if player.Action == EXILED {
		player.IsDead = true
	}

	return auData.applyPlayerUpdate(player)
}

func (auData *AmongUsData) GetNumDetectedPlayers() int {
	return len(auData.PlayerData)
}

func (auData *AmongUsData) GetPhase() Phase {
	return auData.Phase
}

func (auData *AmongUsData) ClearPlayerData(name string) {
	delete(auData.PlayerData, name)
}

func (auData *AmongUsData) ClearAllPlayerData() {
	auData.PlayerData = map[string]PlayerData{}
}

func (auData *AmongUsData) applyPlayerUpdate(update Player) (bool, bool, PlayerData) {

	if _, ok := auData.PlayerData[update.Name]; !ok {
		auData.PlayerData[update.Name] = PlayerData{
			Color:   update.Color,
			Name:    update.Name,
			IsAlive: !update.IsDead,
		}
		log.Printf("Added new player instance for %s\n", update.Name)
		return true, false, auData.PlayerData[update.Name]
	}
	playerData := auData.PlayerData[update.Name]
	isUpdate := playerData.isDifferent(update)
	isAliveUpdate := auData.PlayerData[update.Name].IsAlive != !update.IsDead
	if isUpdate {
		p := PlayerData{
			Color:   update.Color,
			Name:    update.Name,
			IsAlive: !update.IsDead,
		}
		auData.PlayerData[update.Name] = p
	}

	return isUpdate, isAliveUpdate, auData.PlayerData[update.Name]
}

func (auData *AmongUsData) GetByColor(text string) (PlayerData, bool) {
	text = strings.ToLower(text)

	for _, playerData := range auData.PlayerData {
		if GetColorStringForInt(playerData.Color) == text {
			return playerData, true
		}
	}
	return UnlinkedPlayer, false
}

func (auData *AmongUsData) GetByName(text string) (PlayerData, bool) {
	text = strings.ToLower(text)

	for _, playerData := range auData.PlayerData {
		if strings.ReplaceAll(strings.ToLower(playerData.Name), " ", "") == strings.ReplaceAll(strings.ToLower(text), " ", "") {
			return playerData, true
		}
	}
	return UnlinkedPlayer, false
}

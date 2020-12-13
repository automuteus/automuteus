package amongus

import (
	"github.com/automuteus/utils/pkg/game"
	"log"
	"strings"
)

type AmongUsData struct {
	//indexed by amongusname
	PlayerData map[string]PlayerData `json:"playerData"`

	Phase  game.Phase   `json:"phase"`
	Room   string       `json:"room"`
	Region string       `json:"region"`
	Map    game.PlayMap `json:"map"`
}

func NewAmongUsData() AmongUsData {
	return AmongUsData{
		PlayerData: map[string]PlayerData{},
		Phase:      game.MENU,
		Room:       "",
		Region:     "",
		Map:        game.SKELD,
	}
}

func (auData *AmongUsData) SetRoomRegionMap(room, region string, playMap game.PlayMap) {
	auData.Room = room
	auData.Region = region
	auData.Map = playMap
}

func (auData *AmongUsData) GetRoomRegionMap() (string, string, game.PlayMap) {
	return auData.Room, auData.Region, auData.Map
}

func (auData *AmongUsData) SetAllAlive() {
	for i, v := range auData.PlayerData {
		v.IsAlive = true
		auData.PlayerData[i] = v
	}
}

func (auData *AmongUsData) UpdatePhase(phase game.Phase) (old game.Phase) {
	old = auData.Phase
	auData.Phase = phase

	if old != phase {
		if phase == game.LOBBY || (phase == game.TASKS && old == game.LOBBY) {
			auData.SetAllAlive()
		} else if phase == game.MENU {
			auData.SetRoomRegionMap("", "", game.EMPTYMAP)
		}
	}
	return old
}

func (auData *AmongUsData) UpdatePlayer(player game.Player) (updated, isAliveUpdated bool, data PlayerData) {
	phase := auData.Phase

	if phase == game.LOBBY && player.IsDead {
		player.IsDead = false
	}
	if player.Action == game.EXILED {
		player.IsDead = true
	}

	return auData.applyPlayerUpdate(player)
}

func (auData *AmongUsData) GetNumDetectedPlayers() int {
	return len(auData.PlayerData)
}

func (auData *AmongUsData) GetPhase() game.Phase {
	return auData.Phase
}

func (auData *AmongUsData) GetPlayMap() game.PlayMap {
	return auData.Map
}

func (auData *AmongUsData) ClearPlayerData(name string) {
	delete(auData.PlayerData, name)
}

func (auData *AmongUsData) ClearAllPlayerData() {
	auData.PlayerData = map[string]PlayerData{}
}

func (auData *AmongUsData) applyPlayerUpdate(update game.Player) (bool, bool, PlayerData) {

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
		if game.GetColorStringForInt(playerData.Color) == text {
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

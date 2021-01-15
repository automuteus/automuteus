package galactus_client

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/automuteus/galactus/pkg/endpoint"
	"github.com/bwmarrin/discordgo"
	"io/ioutil"
	"log"
	"net/http"
)

func (galactus *GalactusClient) GetGuildRoles(guildID string) ([]*discordgo.Role, error) {
	resp, err := galactus.client.Post(galactus.Address+endpoint.GetGuildRolesPartial+guildID, "application/json", bytes.NewBufferString(""))
	if err != nil {
		return nil, err
	}
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("error reading all bytes from resp body for getRoles")
		log.Println(err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err := errors.New("non-200 status code received for GetRoles:")
		return nil, err
	}

	var roles []*discordgo.Role
	err = json.Unmarshal(respBytes, &roles)
	if err != nil {
		return nil, err
	}
	return roles, nil
}

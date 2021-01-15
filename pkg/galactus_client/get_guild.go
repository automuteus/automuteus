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

func (galactus *GalactusClient) GetGuild(guildID string) (*discordgo.Guild, error) {
	resp, err := galactus.client.Post(galactus.Address+endpoint.GetGuildPartial+guildID, "application/json", bytes.NewBufferString(""))
	if err != nil {
		return nil, err
	}
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("error reading all bytes from resp body for getGuild")
		log.Println(err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err := errors.New("non-200 status code received for GetGuild:")
		return nil, err
	}

	var guild discordgo.Guild
	err = json.Unmarshal(respBytes, &guild)
	if err != nil {
		return nil, err
	}
	return &guild, nil
}

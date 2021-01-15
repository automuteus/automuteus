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

func (galactus *GalactusClient) GetGuildMember(guildID, userID string) (*discordgo.Member, error) {
	resp, err := galactus.client.Post(galactus.Address+endpoint.GetGuildMemberPartial+guildID+"/"+userID, "application/json", bytes.NewBufferString(""))
	if err != nil {
		return nil, err
	}
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("error reading all bytes from resp body for getGuildMember")
		log.Println(err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err := errors.New("non-200 status code received for GetChannels:")
		return nil, err
	}

	var member discordgo.Member
	err = json.Unmarshal(respBytes, &member)
	if err != nil {
		return nil, err
	}
	return &member, nil
}

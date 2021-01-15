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

func (galactus *GalactusClient) CreateUserChannel(userID string) (*discordgo.Channel, error) {
	resp, err := galactus.client.Post(galactus.Address+endpoint.UserChannelCreatePartial+userID, "application/json", bytes.NewBufferString(""))
	if err != nil {
		return nil, err
	}
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("error reading all bytes from resp body for createUserChannel")
		log.Println(err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err := errors.New("non-200 status code received for createUserChannel:")
		return nil, err
	}

	var channel discordgo.Channel
	err = json.Unmarshal(respBytes, &channel)
	if err != nil {
		return nil, err
	}
	return &channel, nil
}

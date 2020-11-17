package discord

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"time"
)

type GalactusClient struct {
	Address string
	client  *http.Client
}

func NewGalactusClient(address string) *GalactusClient {
	//TODO validate/ping galactus here
	return &GalactusClient{
		Address: address,
		client: &http.Client{
			Timeout: time.Second * 10,
		},
	}
}

func (gc *GalactusClient) AddToken(token string) error {
	resp, err := gc.client.Post(gc.Address+"/addtoken", "application/json", bytes.NewBuffer([]byte(token)))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return errors.New("non-okay response from adding token")
	}
	return nil
}

func (gc *GalactusClient) ModifyUser(guildID, connectCode, userID string, mute, deaf bool, nick string) error {
	fullUrl := fmt.Sprintf("%s/modify/%s/%s/%s?mute=%v&deaf=%v", gc.Address, guildID, connectCode, userID, mute, deaf)
	resp, err := gc.client.Post(fullUrl, "application/json", bytes.NewBufferString(""))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return errors.New("non-okay response from modifying user")
	}
	return nil
}

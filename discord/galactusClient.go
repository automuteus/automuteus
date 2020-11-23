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

func NewGalactusClient(address string) (*GalactusClient, error) {
	gc := GalactusClient{
		Address: address,
		client: &http.Client{
			Timeout: time.Second * 10,
		},
	}
	r, err := gc.client.Get(gc.Address + "/")
	if err != nil {
		return &gc, err
	}

	if r.StatusCode != http.StatusOK {
		return &gc, errors.New("galactus returned a non-200 status code; ensure it is reachable")
	}
	return &gc, nil

}

func (gc *GalactusClient) AddToken(token string) error {
	resp, err := gc.client.Post(gc.Address+"/addtoken", "application/json", bytes.NewBuffer([]byte(token)))
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusAlreadyReported {
		return errors.New("this token has already been added and recorded in Galactus")
	}
	if resp.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf("%d response from adding token", resp.StatusCode))
	}
	return nil
}

func (gc *GalactusClient) ModifyUser(guildID, connectCode, userID string, mute, deaf bool) error {
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

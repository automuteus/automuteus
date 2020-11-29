package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bsm/redislock"
	"github.com/denverquane/amongusdiscord/storage"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type GalactusClient struct {
	Address string
	client  *http.Client
}

type UserModify struct {
	UserID uint64 `json:"userID"`
	Mute   bool   `json:"mute"`
	Deaf   bool   `json:"deaf"`
}

type UserModifyRequest struct {
	Premium storage.PremiumTier `json:"premium"`
	Users   []UserModify        `json:"users"`
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

//a response indicating how the mutes/deafens were issued
type MuteDeafenSuccessCounts struct {
	Worker   int64 `json:"worker"`
	Capture  int64 `json:"capture"`
	Official int64 `json:"official"`
}

func (gc *GalactusClient) ModifyUsers(guildID, connectCode string, request UserModifyRequest, lock *redislock.Lock) *MuteDeafenSuccessCounts {
	if lock != nil {
		defer lock.Release(context.Background())
	}

	fullUrl := fmt.Sprintf("%s/modify/%s/%s", gc.Address, guildID, connectCode)
	jBytes, err := json.Marshal(request)
	if err != nil {
		return nil
	}

	log.Println(request)

	resp, err := gc.client.Post(fullUrl, "application/json", bytes.NewBuffer(jBytes))
	if err != nil {
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	mds := MuteDeafenSuccessCounts{}
	jBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return &mds
	}
	err = json.Unmarshal(jBytes, &mds)
	if err != nil {
		log.Println(err)
		return &mds
	}
	return &mds
}

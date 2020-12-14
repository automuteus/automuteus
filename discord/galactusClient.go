package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/automuteus/utils/pkg/task"
	"github.com/bsm/redislock"
	"github.com/denverquane/amongusdiscord/metrics"
	"github.com/go-redis/redis/v8"
	"io/ioutil"
	"log"
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
	defer r.Body.Close()

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
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusAlreadyReported {
		return errors.New("this token has already been added and recorded in Galactus")
	}
	if resp.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf("%d response from adding token", resp.StatusCode))
	}
	return nil
}

func RecordDiscordRequestsByCounts(client *redis.Client, counts *task.MuteDeafenSuccessCounts) {
	metrics.RecordDiscordRequests(client, metrics.MuteDeafenOfficial, counts.Official)
	metrics.RecordDiscordRequests(client, metrics.MuteDeafenWorker, counts.Worker)
	metrics.RecordDiscordRequests(client, metrics.MuteDeafenCapture, counts.Capture)
	metrics.RecordDiscordRequests(client, metrics.InvalidRequest, counts.RateLimit)
}

func (gc *GalactusClient) ModifyUsers(guildID, connectCode string, request task.UserModifyRequest, lock *redislock.Lock) *task.MuteDeafenSuccessCounts {
	if lock != nil {
		defer lock.Release(context.Background())
	}

	fullURL := fmt.Sprintf("%s/modify/%s/%s", gc.Address, guildID, connectCode)
	jBytes, err := json.Marshal(request)
	if err != nil {
		return nil
	}

	log.Println(request)

	resp, err := gc.client.Post(fullURL, "application/json", bytes.NewBuffer(jBytes))
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	mds := task.MuteDeafenSuccessCounts{}
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

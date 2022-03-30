package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/automuteus/automuteus/metrics"
	"github.com/automuteus/utils/pkg/premium"
	"github.com/automuteus/utils/pkg/task"
	"github.com/bsm/redislock"
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

func RecordDiscordRequestsByCounts(client *redis.Client, counts *task.MuteDeafenSuccessCounts) {
	metrics.RecordDiscordRequests(client, metrics.MuteDeafenOfficial, counts.Official)
	metrics.RecordDiscordRequests(client, metrics.MuteDeafenWorker, counts.Worker)
	metrics.RecordDiscordRequests(client, metrics.MuteDeafenCapture, counts.Capture)
	metrics.RecordDiscordRequests(client, metrics.InvalidRequest, counts.RateLimit)
}

func (gc *GalactusClient) ModifyUsers(guildID, connectCode string, request task.UserModifyRequest, lock *redislock.Lock) (*task.MuteDeafenSuccessCounts, error) {
	if lock != nil {
		defer lock.Release(context.Background())
	}

	fullURL := fmt.Sprintf("%s/modify/%s/%s", gc.Address, guildID, connectCode)
	jBytes, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	log.Println(request)

	resp, err := gc.client.Post(fullURL, "application/json", bytes.NewBuffer(jBytes))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	mds := task.MuteDeafenSuccessCounts{}
	jBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return &mds, err
	}
	err = json.Unmarshal(jBytes, &mds)
	if err != nil {
		log.Println(err)
		return &mds, err
	}
	if resp.StatusCode != http.StatusOK {
		return &mds, errors.New("non 200 response: " + resp.Status)
	}

	return &mds, nil
}

func (gc *GalactusClient) VerifyPremiumMembership(guildID uint64, prem premium.Tier) error {
	fullURL := fmt.Sprintf("%s/verify/%d/%d", gc.Address, guildID, prem)
	_, err := gc.client.Post(fullURL, "application/json", nil)
	if err != nil {
		return err
	}
	return nil
}

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

// TODO use endpoints from Galactus directly
const SendMessagePartial = "/sendMessage/"
const SendMessageFull = SendMessagePartial + "{channelID}"

const SendMessageEmbedPartial = "/sendMessageEmbed/"
const SendMessageEmbedFull = SendMessageEmbedPartial + "{channelID}"

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

func (galactus *GalactusClient) SendChannelMessage(channelID string, message string) error {
	resp, err := galactus.client.Post(galactus.Address+SendMessagePartial+channelID, "application/json", bytes.NewBufferString(message))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Println("non-200 status code received for SendMessage:")
		respBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Println("error reading all bytes from resp body for sendmessage")
			log.Println(err)
		}
		log.Println(string(respBytes))
	}
	return nil
}

func RecordDiscordRequestsByCounts(client *redis.Client, counts *task.MuteDeafenSuccessCounts) {
	metrics.RecordDiscordRequests(client, metrics.MuteDeafenOfficial, counts.Official)
	metrics.RecordDiscordRequests(client, metrics.MuteDeafenWorker, counts.Worker)
	metrics.RecordDiscordRequests(client, metrics.MuteDeafenCapture, counts.Capture)
	metrics.RecordDiscordRequests(client, metrics.InvalidRequest, counts.RateLimit)
}

func (galactus *GalactusClient) ModifyUsers(guildID, connectCode string, request task.UserModifyRequest, lock *redislock.Lock) *task.MuteDeafenSuccessCounts {
	if lock != nil {
		defer lock.Release(context.Background())
	}

	fullURL := fmt.Sprintf("%s/modify/%s/%s", galactus.Address, guildID, connectCode)
	jBytes, err := json.Marshal(request)
	if err != nil {
		return nil
	}

	log.Println(request)

	resp, err := galactus.client.Post(fullURL, "application/json", bytes.NewBuffer(jBytes))
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

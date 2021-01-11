package galactus_client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/automuteus/utils/pkg/task"
	"github.com/bsm/redislock"
	"io/ioutil"
	"log"
	"net/http"
)

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

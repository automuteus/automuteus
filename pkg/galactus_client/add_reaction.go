package galactus_client

import (
	"bytes"
	"github.com/automuteus/galactus/pkg/endpoint"
	"io/ioutil"
	"log"
	"net/http"
)

func (galactus *GalactusClient) AddReaction(channelID, messageID, emojiID string) error {
	resp, err := galactus.client.Post(galactus.Address+endpoint.AddReactionPartial+channelID+"/"+messageID+"/"+emojiID, "application/json", bytes.NewBufferString(""))
	if err != nil {
		return err
	}
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("error reading all bytes from resp body for sendmessage")
		log.Println(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Println("non-200 status code received for SendMessage:")
		log.Println(string(respBytes))
	}

	return err
}

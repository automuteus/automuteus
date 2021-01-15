package galactus_client

import (
	"bytes"
	"github.com/automuteus/galactus/pkg/endpoint"
	"io/ioutil"
	"log"
	"net/http"
)

func (galactus *GalactusClient) DeleteChannelMessage(channelID, messageID string) error {
	resp, err := galactus.client.Post(galactus.Address+endpoint.DeleteMessagePartial+channelID+"/"+messageID, "application/json", bytes.NewBufferString(""))
	if err != nil {
		return err
	}
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("error reading all bytes from resp body for deleteChannelMessage")
		log.Println(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Println("non-200 status code received for DeleteChannelMessage:")
		log.Println(string(respBytes))
	}
	return nil
}

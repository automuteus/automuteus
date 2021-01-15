package galactus_client

import (
	"bytes"
	"encoding/json"
	"github.com/automuteus/galactus/pkg/endpoint"
	"github.com/bwmarrin/discordgo"
	"io/ioutil"
	"log"
	"net/http"
)

func (galactus *GalactusClient) EditChannelMessageEmbed(channelID, messageID string, embed discordgo.MessageEmbed) (*discordgo.Message, error) {
	message, err := json.Marshal(embed)
	if err != nil {
		return nil, err
	}

	resp, err := galactus.client.Post(galactus.Address+endpoint.EditMessageEmbedPartial+channelID+"/"+messageID, "application/json", bytes.NewBuffer(message))
	if err != nil {
		return nil, err
	}
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("error reading all bytes from resp body for editChannelMessageEmbed")
		log.Println(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Println("non-200 status code received for EditChannelMessageEmbed:")
		log.Println(string(respBytes))
	}
	var msg discordgo.Message
	err = json.Unmarshal(respBytes, &msg)
	return &msg, err
}

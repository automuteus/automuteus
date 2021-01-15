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

func (galactus *GalactusClient) SendChannelMessage(channelID string, message string) (*discordgo.Message, error) {
	resp, err := galactus.client.Post(galactus.Address+endpoint.SendMessagePartial+channelID, "application/json", bytes.NewBufferString(message))
	if err != nil {
		return nil, err
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
	var msg discordgo.Message
	err = json.Unmarshal(respBytes, &msg)
	return &msg, err
}

func (galactus *GalactusClient) SendChannelMessageEmbed(channelID string, embed *discordgo.MessageEmbed) (*discordgo.Message, error) {
	message, err := json.Marshal(*embed)
	if err != nil {
		return nil, err
	}

	resp, err := galactus.client.Post(galactus.Address+endpoint.SendMessageEmbedPartial+channelID, "application/json", bytes.NewBuffer(message))
	if err != nil {
		return nil, err
	}
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("error reading all bytes from resp body for sendmessageembed")
		log.Println(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Println("non-200 status code received for SendMessageEmbed:")
		log.Println(string(respBytes))
	}
	var msg discordgo.Message
	err = json.Unmarshal(respBytes, &msg)
	return &msg, err
}

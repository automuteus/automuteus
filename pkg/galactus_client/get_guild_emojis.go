package galactus_client

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/automuteus/galactus/pkg/endpoint"
	"github.com/bwmarrin/discordgo"
	"io/ioutil"
	"log"
	"net/http"
)

func (galactus *GalactusClient) GetGuildEmojis(guildID string) ([]*discordgo.Emoji, error) {
	resp, err := galactus.client.Post(galactus.Address+endpoint.GetGuildEmojisPartial+guildID, "application/json", bytes.NewBufferString(""))
	if err != nil {
		return nil, err
	}
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("error reading all bytes from resp body for getGuildEmojis")
		log.Println(err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err := errors.New("non-200 status code received for getGuildEmojis")
		return nil, err
	}

	var emojis []*discordgo.Emoji
	err = json.Unmarshal(respBytes, &emojis)
	if err != nil {
		return nil, err
	}
	return emojis, nil
}

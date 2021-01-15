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

func (galactus *GalactusClient) CreateGuildEmoji(guildID, emojiName, content string) (*discordgo.Emoji, error) {
	resp, err := galactus.client.Post(galactus.Address+endpoint.CreateGuildEmojiPartial+guildID+"/"+emojiName, "application/json", bytes.NewBufferString(content))
	if err != nil {
		return nil, err
	}
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("error reading all bytes from resp body for createGuildEmoji")
		log.Println(err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err := errors.New("non-200 status code received for createGuildEmoji:")
		return nil, err
	}

	var emoji discordgo.Emoji
	err = json.Unmarshal(respBytes, &emoji)
	if err != nil {
		return nil, err
	}
	return &emoji, nil
}

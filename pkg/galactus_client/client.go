package galactus_client

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/automuteus/galactus/pkg/discord_message"
	"github.com/automuteus/galactus/pkg/endpoint"
	"github.com/bwmarrin/discordgo"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type GalactusClient struct {
	Address                   string
	client                    *http.Client
	killChannel               chan struct{}
	messageCreateHandler      func(m discordgo.MessageCreate)
	messageReactionAddHandler func(m discordgo.MessageReactionAdd)
	voiceStateUpdateHandler   func(m discordgo.VoiceStateUpdate)
	guildDeleteHandler        func(m discordgo.GuildDelete)
	guildCreateHandler        func(m discordgo.GuildCreate)
}

func NewGalactusClient(address string) (*GalactusClient, error) {
	gc := GalactusClient{
		Address: address,
		client: &http.Client{
			Timeout: time.Second * 10,
		},
		killChannel: nil,
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

func (galactus *GalactusClient) StartPolling() error {

	if galactus.killChannel != nil {
		return errors.New("client is already polling")
	}
	galactus.killChannel = make(chan struct{})
	connected := false

	go func() {
		for {
			select {
			case <-galactus.killChannel:
				return

			default:
				req, err := http.NewRequest("POST", galactus.Address+endpoint.RequestJob, bytes.NewBufferString(""))
				if err != nil {
					log.Println("Invalid URL provided: " + galactus.Address + endpoint.RequestJob)
					break
				}
				req.Cancel = galactus.killChannel

				response, err := http.DefaultClient.Do(req)
				if err != nil {
					connected = false
					log.Printf("Could not reach Galactus at %s; is this the right URL, and is Galactus online?", galactus.Address+endpoint.RequestJob)
					log.Println("Waiting 1 second before retrying")
					time.Sleep(time.Second * 1)
				} else {
					if !connected {
						log.Println("Successful connection to Galactus")
						connected = true
					}
					body, err := ioutil.ReadAll(response.Body)
					if err != nil {
						log.Println(err)
					}

					if response.StatusCode == http.StatusOK {
						var msg discord_message.DiscordMessage
						err := json.Unmarshal(body, &msg)
						if err != nil {
							log.Println(err)
						} else {
							galactus.dispatch(msg)
						}
					}
					response.Body.Close()
				}
			}
		}
	}()
	return nil
}

func (galactus *GalactusClient) dispatch(msg discord_message.DiscordMessage) {
	switch msg.MessageType {
	case discord_message.MessageCreate:
		var messageCreate discordgo.MessageCreate
		err := json.Unmarshal(msg.Data, &messageCreate)
		if err != nil {
			log.Println(err)
		} else {
			galactus.messageCreateHandler(messageCreate)
		}
	case discord_message.MessageReactionAdd:
		var messageReactionAdd discordgo.MessageReactionAdd
		err := json.Unmarshal(msg.Data, &messageReactionAdd)
		if err != nil {
			log.Println(err)
		} else {
			galactus.messageReactionAddHandler(messageReactionAdd)
		}
	case discord_message.VoiceStateUpdate:
		var voiceStateUpdate discordgo.VoiceStateUpdate
		err := json.Unmarshal(msg.Data, &voiceStateUpdate)
		if err != nil {
			log.Println(err)
		} else {
			galactus.voiceStateUpdateHandler(voiceStateUpdate)
		}
	case discord_message.GuildDelete:
		var guildDelete discordgo.GuildDelete
		err := json.Unmarshal(msg.Data, &guildDelete)
		if err != nil {
			log.Println(err)
		} else {
			galactus.guildDeleteHandler(guildDelete)
		}
	case discord_message.GuildCreate:
		var guildCreate discordgo.GuildCreate
		err := json.Unmarshal(msg.Data, &guildCreate)
		if err != nil {
			log.Println(err)
		} else {
			galactus.guildCreateHandler(guildCreate)
		}
	}
}

func (galactus *GalactusClient) StopPolling() {
	if galactus.killChannel != nil {
		galactus.killChannel <- struct{}{}
	}
}

func (galactus *GalactusClient) RegisterHandler(msgType discord_message.DiscordMessageType, f interface{}) bool {
	switch msgType {
	case discord_message.MessageCreate:
		galactus.messageCreateHandler = f.(func(m discordgo.MessageCreate))
		log.Println("Registered Message Create Handler")
		return true
	case discord_message.MessageReactionAdd:
		galactus.messageReactionAddHandler = f.(func(m discordgo.MessageReactionAdd))
		log.Println("Registered Message Reaction Add Handler")
		return true
	case discord_message.GuildDelete:
		galactus.guildDeleteHandler = f.(func(m discordgo.GuildDelete))
		log.Println("Registered Guild Delete Handler")
		return true
	case discord_message.VoiceStateUpdate:
		galactus.voiceStateUpdateHandler = f.(func(m discordgo.VoiceStateUpdate))
		log.Println("Registered Voice State Update Handler")
		return true
	case discord_message.GuildCreate:
		galactus.guildCreateHandler = f.(func(m discordgo.GuildCreate))
		log.Println("Registered Guild Create Handler")
		return true
	}
	return false
}

package galactus_client

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/bwmarrin/discordgo"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

// TODO use endpoints from Galactus directly!!!
const SendMessagePartial = "/sendMessage/"
const SendMessageFull = SendMessagePartial + "{channelID}"

const SendMessageEmbedPartial = "/sendMessageEmbed/"
const SendMessageEmbedFull = SendMessageEmbedPartial + "{channelID}"

const EditMessageEmbedPartial = "/editMessageEmbed/"
const EditMessageEmbedFull = EditMessageEmbedPartial + "{channelID}/{messageID}"

const DeleteMessagePartial = "/deleteMessage/"
const DeleteMessageFull = DeleteMessagePartial + "{channelID}/{messageID}"

const RemoveReactionPartial = "/removeReaction/"
const RemoveReactionFull = RemoveReactionPartial + "{channelID}/{messageID}/{emojiID}/{userID}"

const RemoveAllReactionsPartial = "/removeAllReactions/"
const RemoveAllReactionsFull = RemoveAllReactionsPartial + "{channelID}/{messageID}"

const AddReactionPartial = "/addReaction/"
const AddReactionFull = AddReactionPartial + "{channelID}/{messageID}/{emojiID}"

const ModifyUserbyGuildConnectCode = "/modify/{guildID}/{connectCode}"

const GetGuildPartial = "/guild/"
const GetGuildFull = GetGuildPartial + "{guildID}"

const GetGuildChannelsPartial = "/guildChannels/"
const GetGuildChannelsFull = GetGuildChannelsPartial + "{guildID}"

const GetGuildMemberPartial = "/guildMember/"
const GetGuildMemberFull = GetGuildMemberPartial + "{guildID}/{userID}"

const GetGuildRolesPartial = "/guildRoles/"
const GetGuildRolesFull = GetGuildRolesPartial + "{guildID}"

const UserChannelCreatePartial = "/createUserChannel/"
const UserChannelCreateFull = UserChannelCreatePartial + "{userID}"

const RequestJob = "/request/job"
const JobCount = "/totalJobs"

// TODO use endpoints from Galactus directly!!!

// TODO use from Galactus
type DiscordMessageType int

// TODO use from Galactus
const (
	GuildCreate DiscordMessageType = iota
	GuildDelete
	VoiceStateUpdate
	MessageCreate
	MessageReactionAdd
)

// TODO use from Galactus
var DiscordMessageTypeStrings = []string{
	"GuildCreate",
	"GuildDelete",
	"VoiceStateUpdate",
	"MessageCreate",
	"MessageReactionAdd",
}

// TODO use from Galactus
type DiscordMessage struct {
	MessageType DiscordMessageType
	Data        []byte
}

type GalactusClient struct {
	Address                   string
	client                    *http.Client
	killChannel               chan bool
	messageCreateHandler      func(m discordgo.MessageCreate)
	messageReactionAddHandler func(m discordgo.MessageReactionAdd)
	voiceStateUpdateHandler   func(m discordgo.VoiceStateUpdate)
	guildDeleteHandler        func(m discordgo.GuildDelete)

	//TODO guild create
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

func (galactus *GalactusClient) StartPolling(interval time.Duration) error {
	if galactus.killChannel != nil {
		return errors.New("client is already polling")
	}
	galactus.killChannel = make(chan bool)

	ticker := time.NewTicker(interval).C
	go func() {
		for {
			select {
			case <-galactus.killChannel:
				return

			case <-ticker:
				code := http.StatusOK

				for code == http.StatusOK {
					response, err := http.Post(galactus.Address+RequestJob, "application/json", bytes.NewBufferString(""))
					if err != nil {
						log.Println("error when trying to POST for new job")
						code = http.StatusBadGateway
					} else {
						code = response.StatusCode
						body, err := ioutil.ReadAll(response.Body)
						if err != nil {
							log.Println(err)
						}

						if code == http.StatusOK {
							var msg DiscordMessage
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
		}
	}()
	return nil
}

func (galactus *GalactusClient) dispatch(msg DiscordMessage) {
	switch msg.MessageType {
	case MessageCreate:
		var messageCreate discordgo.MessageCreate
		err := json.Unmarshal(msg.Data, &messageCreate)
		if err != nil {
			log.Println(err)
		} else {
			galactus.messageCreateHandler(messageCreate)
		}
	case MessageReactionAdd:
		var messageReactionAdd discordgo.MessageReactionAdd
		err := json.Unmarshal(msg.Data, &messageReactionAdd)
		if err != nil {
			log.Println(err)
		} else {
			galactus.messageReactionAddHandler(messageReactionAdd)
		}
	case VoiceStateUpdate:
		var voiceStateUpdate discordgo.VoiceStateUpdate
		err := json.Unmarshal(msg.Data, &voiceStateUpdate)
		if err != nil {
			log.Println(err)
		} else {
			galactus.voiceStateUpdateHandler(voiceStateUpdate)
		}
	case GuildDelete:
		var guildDelete discordgo.GuildDelete
		err := json.Unmarshal(msg.Data, &guildDelete)
		if err != nil {
			log.Println(err)
		} else {
			galactus.guildDeleteHandler(guildDelete)
		}
	}
}

func (galactus *GalactusClient) StopPolling() {
	if galactus.killChannel != nil {
		galactus.killChannel <- true
	}
}

func (galactus *GalactusClient) RegisterHandler(msgType DiscordMessageType, f interface{}) bool {
	switch msgType {
	case MessageCreate:
		galactus.messageCreateHandler = f.(func(m discordgo.MessageCreate))
		log.Println("Registered Message Create Handler")
		return true
	case MessageReactionAdd:
		galactus.messageReactionAddHandler = f.(func(m discordgo.MessageReactionAdd))
		log.Println("Registered Message Reaction Add Handler")
		return true
	case GuildDelete:
		galactus.guildDeleteHandler = f.(func(m discordgo.GuildDelete))
		log.Println("Registered Guild Delete Handler")
		return true
	case VoiceStateUpdate:
		galactus.voiceStateUpdateHandler = f.(func(m discordgo.VoiceStateUpdate))
		log.Println("Registered Voice State Update Handler")
		return true
	}
	return false
}

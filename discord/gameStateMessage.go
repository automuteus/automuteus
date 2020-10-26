package discord

import (
	"github.com/bwmarrin/discordgo"
)

type GameStateMessage struct {
	MessageID        string `json:"messageID"`
	MessageChannelID string `json:"messageChannelID"`
	MessageAuthorID  string `json:"messageAuthorID"`
	LeaderID         string `json:"leaderID"`
}

func MakeGameStateMessage() GameStateMessage {
	return GameStateMessage{
		MessageID:        "",
		MessageChannelID: "",
		LeaderID:         "",
	}
}

func (dgs *DiscordGameState) Exists() bool {
	return dgs.GameStateMsg.MessageID != ""
}

func (dgs *DiscordGameState) AddReaction(s *discordgo.Session, emoji string) {
	if dgs.GameStateMsg.MessageID != "" {
		addReaction(s, dgs.GameStateMsg.MessageChannelID, dgs.GameStateMsg.MessageID, emoji)
	}
}

func (dgs *DiscordGameState) RemoveAllReactions(s *discordgo.Session) {
	if dgs.GameStateMsg.MessageID != "" {
		removeAllReactions(s, dgs.GameStateMsg.MessageChannelID, dgs.GameStateMsg.MessageID)
	}
}

func (dgs *DiscordGameState) AddAllReactions(s *discordgo.Session, emojis []Emoji) {
	for _, e := range emojis {
		dgs.AddReaction(s, e.FormatForReaction())
	}
	dgs.AddReaction(s, "❌")
}

func (dgs *DiscordGameState) Delete(s *discordgo.Session) {
	if dgs.GameStateMsg.MessageID != "" {
		go deleteMessage(s, dgs.GameStateMsg.MessageChannelID, dgs.GameStateMsg.MessageID)
		dgs.GameStateMsg.MessageID = ""
		dgs.GameStateMsg.MessageChannelID = ""
		dgs.NeedsUpload = true
	}
}

//TODO bring back deferred edit
func (dgs *DiscordGameState) Edit(s *discordgo.Session, me *discordgo.MessageEmbed) {
	editMessageEmbed(s, dgs.GameStateMsg.MessageChannelID, dgs.GameStateMsg.MessageID, me)
}

func (dgs *DiscordGameState) CreateMessage(s *discordgo.Session, me *discordgo.MessageEmbed, channelID string, authorID string) {
	dgs.GameStateMsg.LeaderID = authorID
	msg := sendMessageEmbed(s, channelID, me)
	if msg != nil {
		dgs.GameStateMsg.MessageAuthorID = msg.Author.ID
		dgs.GameStateMsg.MessageChannelID = msg.ChannelID
		dgs.GameStateMsg.MessageID = msg.ID
	}
	dgs.NeedsUpload = true
}

func (dgs *DiscordGameState) SameChannel(channelID string) bool {
	if dgs.GameStateMsg.MessageID != "" {
		return dgs.GameStateMsg.MessageChannelID == channelID
	}
	return false
}

func (dgs *DiscordGameState) IsReactionTo(m *discordgo.MessageReactionAdd) bool {
	if !dgs.Exists() {
		return false
	}

	return m.ChannelID == dgs.GameStateMsg.MessageChannelID && m.MessageID == dgs.GameStateMsg.MessageID && m.UserID != dgs.GameStateMsg.MessageAuthorID
}

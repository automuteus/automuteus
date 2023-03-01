package bot

import (
	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/bwmarrin/discordgo"
	"log"
)

func (bot *Bot) addAllMissingEmojis(s *discordgo.Session, guildID string, alive bool, serverEmojis []*discordgo.Emoji) {
	for i, emoji := range discord.GlobalAlivenessEmojis[alive] {
		alreadyExists := false
		for _, v := range serverEmojis {
			if v.Name == emoji.Name {
				emoji.ID = v.ID
				bot.StatusEmojis[alive][i] = emoji
				alreadyExists = true
				break
			}
		}
		if !alreadyExists {
			b64 := emoji.DownloadAndBase64Encode()
			p := discordgo.EmojiParams{
				Name:  emoji.Name,
				Image: b64,
				Roles: nil,
			}
			em, err := s.GuildEmojiCreate(guildID, &p)
			if err != nil {
				log.Println(err)
			} else {
				log.Printf("Added emoji %s successfully!\n", emoji.Name)
				emoji.ID = em.ID
				bot.StatusEmojis[alive][i] = emoji
			}
		}
	}
}

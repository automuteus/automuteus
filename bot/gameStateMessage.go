package bot

import (
	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/automuteus/automuteus/v8/pkg/redis"
	"github.com/automuteus/automuteus/v8/pkg/settings"
)

func (bot *Bot) DispatchRefreshOrEdit(readOnlyDgs *discord.GameState, dgsRequest discord.GameStateRequest, sett *settings.GuildSettings) {
	if readOnlyDgs.ShouldRefresh() {
		bot.RefreshGameStateMessage(dgsRequest, sett)
	} else {
		edited := readOnlyDgs.DispatchEdit(bot.PrimarySession, bot.gameStateResponse(readOnlyDgs, sett))
		if edited {
			bot.RedisDriver.RecordDiscordRequests(redis.MessageEdit, 1)
		}
	}
}

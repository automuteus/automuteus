package redis

import (
	"context"
	"github.com/automuteus/automuteus/v8/pkg"
	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/automuteus/automuteus/v8/pkg/storage"
	"github.com/bwmarrin/discordgo"
)

func GetApiInfo(redisDriver Driver, psql storage.PsqlInterface, sess *discordgo.Session) discord.ApiInfo {
	totalGuilds := redisDriver.GetGuildCounter(context.Background())
	activeGames := redisDriver.GetActiveGames(context.Background(), GameTimeoutSeconds)

	totalUsers := redisDriver.GetTotalUsers(context.Background())
	if totalUsers == NotFound {
		totalUsers = redisDriver.RefreshTotalUsers(context.Background(), psql)
	}

	totalGames := redisDriver.GetTotalGames(context.Background())
	if totalGames == NotFound {
		totalGames = redisDriver.RefreshTotalGames(context.Background(), psql)
	}
	var shardCount int
	if sess != nil {
		shardCount = sess.ShardCount
	}
	return discord.ApiInfo{
		Version:     pkg.Version,
		Commit:      pkg.Commit,
		ShardCount:  shardCount,
		TotalGuilds: totalGuilds,
		ActiveGames: activeGames,
		TotalUsers:  totalUsers,
		TotalGames:  totalGames,
	}
}

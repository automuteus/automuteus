package discord

type BotInfo struct {
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	ShardID     int    `json:"shardID"`
	ShardCount  int    `json:"shardCount"`
	TotalGuilds int64  `json:"totalGuilds"`
	ActiveGames int64  `json:"activeGames"`
	TotalUsers  int64  `json:"totalUsers"`
	TotalGames  int64  `json:"totalGames"`
}

type ApiInfo struct {
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	ShardCount  int    `json:"shardCount"`
	TotalGuilds int64  `json:"totalGuilds"`
	ActiveGames int64  `json:"activeGames"`
	TotalUsers  int64  `json:"totalUsers"`
	TotalGames  int64  `json:"totalGames"`
}

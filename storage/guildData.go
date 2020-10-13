package storage

type GuildData struct {
	GuildID   string `json:"guildID"`
	GuildName string `json:"guildName"`

	GuildStats    *GuildStats
	GuildSettings *GuildSettings
}

func MakeDefaultGuildData(id string, name string) *GuildData {
	return &GuildData{
		GuildID:   id,
		GuildName: name,

		GuildStats:    MakeGuildStats(),
		GuildSettings: MakeGuildSettings(),
	}
}

func MakeEmptyGuildData(id string, name string) *GuildData {
	return &GuildData{
		GuildID:   id,
		GuildName: name,

		GuildStats:    nil,
		GuildSettings: nil,
	}
}

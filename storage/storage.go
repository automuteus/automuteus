package storage

// StorageInterface defines the format for how to interact with storage and fetch/write guild data.
// This is used, currently, to specify filesystem and Google Firestore DB operations, but could
// likely be easily extended into other DBs or storage connections
type StorageInterface interface {
	Init(string) error
	GetGuildSettings(string) (*GuildSettings, error)
	WriteGuildSettings(string, *GuildSettings) error

	GetAllUserSettings() *UserSettingsCollection
	WriteUserSettings(string, *UserSettings) error

	Close() error
}

package storage

type StorageInterface interface {
	Init(string) error
	GetGuildData(string) (map[string]interface{}, error)
	Close() error
}

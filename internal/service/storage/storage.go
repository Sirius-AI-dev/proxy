package storage

import (
	"github.com/unibackend/uniproxy/internal/config"
	"time"
)

type Storage interface {
	Get(string) string
	Put(string, string, time.Duration)
	Del(...string)
}

func New(cfg map[string]config.Storage) map[string]Storage {
	storage := make(map[string]Storage)
	for name, store := range cfg {
		switch store.Type {
		case "redis":
			storage[name] = NewRedisStorage(store.Host, store.Password, store.Database)
		}
	}
	return storage
}
